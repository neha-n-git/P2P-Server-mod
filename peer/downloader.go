package peer

import (
	"crypto/sha256"
	"fmt"
	"log"
	"p2p/file"
	"p2p/transport"
	"sync"
)

// chunkResult holds the result of downloading a single chunk.
// Used by channels to communicate between goroutines.
type chunkResult struct {
	Index int
	Data  []byte
	Err   error
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

// Download fetches a file by name from a specific peer (or multiple peers).
// It downloads all chunks concurrently using goroutines, verifies each
// chunk's integrity via SHA-256 hash, and reassembles the file.
func (d *Downloader) Download(fileName string, primaryPeer string) ([]byte, error) {
	// 1. Fetch file metadata from the primary peer
	meta, err := d.Transport.FetchFileMeta(primaryPeer, fileName)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file metadata for '%s' from %s: %w", fileName, primaryPeer, err)
	}

	if meta.TotalChunks == 0 || len(meta.Chunks) == 0 {
		return nil, fmt.Errorf("file '%s' has no chunks", fileName)
	}

	log.Printf("[Downloader] Starting download of '%s' (%d bytes, %d chunks) from %s",
		fileName, meta.FileSize, meta.TotalChunks, primaryPeer)

	// 2. Launch goroutines to download each chunk concurrently
	resultCh := make(chan chunkResult, meta.TotalChunks)
	var wg sync.WaitGroup

	for _, chunkMeta := range meta.Chunks {
		wg.Add(1)
		go func(cm file.ChunkMeta) {
			defer wg.Done()
			d.downloadChunk(primaryPeer, cm, resultCh)
		}(chunkMeta)
	}

	// Close the results channel once all goroutines are done
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 3. Collect results from the channel
	chunks := make([]file.Chunk, 0, meta.TotalChunks)
	var downloadErrors []error

	for result := range resultCh {
		if result.Err != nil {
			downloadErrors = append(downloadErrors, result.Err)
			continue
		}
		chunks = append(chunks, file.Chunk{
			Index: result.Index,
			Data:  result.Data,
		})
	}

	if len(downloadErrors) > 0 {
		return nil, fmt.Errorf("failed to download %d/%d chunks; first error: %w",
			len(downloadErrors), meta.TotalChunks, downloadErrors[0])
	}

	if len(chunks) != meta.TotalChunks {
		return nil, fmt.Errorf("expected %d chunks but received %d", meta.TotalChunks, len(chunks))
	}

	// 4. Reassemble the file from chunks
	data, err := file.MergeChunks(chunks)
	if err != nil {
		return nil, fmt.Errorf("failed to merge chunks: %w", err)
	}

	log.Printf("[Downloader] Successfully downloaded and reassembled '%s' (%d bytes)", fileName, len(data))

	// 5. Store the file in the local peer's shared files
	d.LocalPeer.Mu.Lock()
	d.LocalPeer.SharedFiles[fileName] = *meta
	d.LocalPeer.Mu.Unlock()

	return data, nil
}

// downloadChunk downloads a single chunk from a peer and verifies its integrity.
// It sends the result (or error) to the results channel.
func (d *Downloader) downloadChunk(peerAddr string, cm file.ChunkMeta, resultCh chan<- chunkResult) {
	hashStr := fmt.Sprintf("%x", cm.Hash)

	data, err := d.Transport.FetchChunk(peerAddr, hashStr)
	if err != nil {
		resultCh <- chunkResult{
			Index: cm.Index,
			Err:   fmt.Errorf("chunk %d (hash %s): download failed: %w", cm.Index, hashStr, err),
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

	log.Printf("[Downloader] Chunk %d verified (hash: %s)", cm.Index, hashStr)

	// Store the chunk data locally
	d.LocalPeer.Mu.Lock()
	d.LocalPeer.ChunkDataStorage[hashStr] = data
	d.LocalPeer.Mu.Unlock()

	resultCh <- chunkResult{
		Index: cm.Index,
		Data:  data,
	}
}
