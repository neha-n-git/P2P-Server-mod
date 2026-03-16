package peer

import (
	"crypto/sha256"
	"fmt"
	"log"
	"p2p/file"
	"p2p/registry"
	"p2p/transport"
	"sync"
)

// chunkResult holds the result of downloading a single chunk.
// Used by channels to communicate between goroutines.
type chunkResult struct {
	Index    int
	Data     []byte
	Err      error
	PeerAddr string // which peer served this chunk
}

// ChunkSource records which peer served a specific chunk.
type ChunkSource struct {
	ChunkIndex int    `json:"chunk_index"`
	PeerAddr   string `json:"peer_address"`
}

// DownloadResult holds the downloaded file data along with metadata
// about which peer served each chunk (for load balancing visibility).
type DownloadResult struct {
	Data         []byte        // reassembled file bytes
	ChunkSources []ChunkSource // which peer served each chunk
	PeersUsed    []string      // all peers that participated
}

// Downloader coordinates concurrent chunk downloads from peers.
// It uses goroutines for parallel downloads and channels for
// collecting results, then reassembles the file.
type Downloader struct {
	LocalPeer *Peer
	Transport transport.Transport
}

// NewDownloader creates a new Downloader instance.
func NewDownloader(p *Peer, t transport.Transport) *Downloader {
	return &Downloader{
		LocalPeer: p,
		Transport: t,
	}
}

// findPeersWithFile queries all known peers (except self) to find which ones
// have the given file. The primaryPeer is always included.
func (d *Downloader) findPeersWithFile(fileName string, primaryPeer string) []string {
	available := []string{primaryPeer}
	selfAddr := fmt.Sprintf("localhost:%d", d.LocalPeer.Port)

	for _, peerAddr := range registry.GetPeers() {
		// Skip self and the primary peer (already included)
		if peerAddr == selfAddr || peerAddr == primaryPeer {
			continue
		}
		// Probe the peer to see if it has the file
		_, err := d.Transport.FetchFileMeta(peerAddr, fileName)
		if err == nil {
			available = append(available, peerAddr)
		}
	}

	return available
}

// Download fetches a file by distributing chunk downloads across all peers
// that have the file. It uses round-robin assignment to balance the load,
// downloads chunks concurrently, verifies integrity, and reassembles the file.
func (d *Downloader) Download(fileName string, primaryPeer string) (*DownloadResult, error) {
	// 1. Fetch file metadata from the primary peer
	meta, err := d.Transport.FetchFileMeta(primaryPeer, fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file metadata for '%s' from %s: %w", fileName, primaryPeer, err)
	}

	if meta.TotalChunks == 0 || len(meta.Chunks) == 0 {
		return nil, fmt.Errorf("file '%s' has no chunks", fileName)
	}

	// 2. Discover all peers that have this file
	availablePeers := d.findPeersWithFile(fileName, primaryPeer)

	log.Printf("[Downloader] Starting download of '%s' (%d bytes, %d chunks) — load balanced across %d peer(s): %v",
		fileName, meta.FileSize, meta.TotalChunks, len(availablePeers), availablePeers)

	// 3. Launch goroutines to download each chunk concurrently,
	//    distributing chunks across peers using round-robin
	resultCh := make(chan chunkResult, meta.TotalChunks)
	var wg sync.WaitGroup
	peerCount := len(availablePeers)

	for i, chunkMeta := range meta.Chunks {
		assignedPeer := availablePeers[i%peerCount] // round-robin assignment
		wg.Add(1)
		go func(cm file.ChunkMeta, peer string) {
			defer wg.Done()
			d.downloadChunk(peer, cm, resultCh)
		}(chunkMeta, assignedPeer)
	}

	// Close the results channel once all goroutines are done
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 4. Collect results from the channel
	chunks := make([]file.Chunk, 0, meta.TotalChunks)
	var downloadErrors []error
	var chunkSources []ChunkSource

	// Track how many chunks each peer served (for logging)
	peerChunkCount := make(map[string]int)

	for result := range resultCh {
		if result.Err != nil {
			downloadErrors = append(downloadErrors, result.Err)
			continue
		}
		chunks = append(chunks, file.Chunk{
			Index: result.Index,
			Data:  result.Data,
		})
		chunkSources = append(chunkSources, ChunkSource{
			ChunkIndex: result.Index,
			PeerAddr:   result.PeerAddr,
		})
		peerChunkCount[result.PeerAddr]++
	}

	if len(downloadErrors) > 0 {
		return nil, fmt.Errorf("failed to download %d/%d chunks; first error: %w",
			len(downloadErrors), meta.TotalChunks, downloadErrors[0])
	}

	if len(chunks) != meta.TotalChunks {
		return nil, fmt.Errorf("expected %d chunks but received %d", meta.TotalChunks, len(chunks))
	}

	// Log load distribution
	for peer, count := range peerChunkCount {
		log.Printf("[Downloader] Peer %s served %d/%d chunks", peer, count, meta.TotalChunks)
	}

	// 5. Reassemble the file from chunks
	data, err := file.MergeChunks(chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to merge chunks: %w", err)
	}

	log.Printf("[Downloader] Successfully downloaded and reassembled '%s' (%d bytes)", fileName, len(data))

	// 6. Store the file in the local peer's shared files
	d.LocalPeer.Mu.Lock()
	d.LocalPeer.SharedFiles[fileName] = *meta
	d.LocalPeer.Mu.Unlock()

	return &DownloadResult{
		Data:         data,
		ChunkSources: chunkSources,
		PeersUsed:    availablePeers,
	}, nil
}

// downloadChunk downloads a single chunk from a peer and verifies its integrity.
// It sends the result (or error) to the results channel.
func (d *Downloader) downloadChunk(peerAddr string, cm file.ChunkMeta, resultCh chan<- chunkResult) {
	hashStr := fmt.Sprintf("%x", cm.Hash)

	log.Printf("[Downloader] Fetching chunk %d from peer %s", cm.Index, peerAddr)

	data, err := d.Transport.FetchChunk(peerAddr, hashStr)
	if err != nil {
		resultCh <- chunkResult{
			Index: cm.Index,
			Err:   fmt.Errorf("chunk %d (hash %s): download failed from %s: %w", cm.Index, hashStr, peerAddr, err),
		}
		return
	}

	// Verify chunk integrity using SHA-256
	actualHash := sha256.Sum256(data)
	if actualHash != cm.Hash {
		resultCh <- chunkResult{
			Index: cm.Index,
			Err: fmt.Errorf("chunk %d: integrity check failed (expected %x, got %x)",
				cm.Index, cm.Hash, actualHash),
		}
		return
	}

	log.Printf("[Downloader] Chunk %d verified from peer %s (hash: %s)", cm.Index, peerAddr, hashStr)

	// Store the chunk data locally
	d.LocalPeer.Mu.Lock()
	d.LocalPeer.ChunkDataStorage[hashStr] = data
	d.LocalPeer.Mu.Unlock()

	resultCh <- chunkResult{
		Index:    cm.Index,
		Data:     data,
		PeerAddr: peerAddr,
	}
}
