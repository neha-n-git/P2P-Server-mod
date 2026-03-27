package peer

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"p2p/file"
	"testing"
)

// mockTransport is a fake Transport implementation for testing the Downloader
// without real network calls.
type mockTransport struct {
	// Files maps peer address -> filename -> FileMeta
	Files map[string]map[string]file.FileMeta
	// ChunkData maps peer address -> hash string -> data
	ChunkData map[string]map[string][]byte
}

func (m *mockTransport) RegisterWithPeer(peerAddress string, selfAddress string) error {
	return nil
}

func (m *mockTransport) FetchFileList(peerAddress string) ([]file.FileMeta, error) {
	files, ok := m.Files[peerAddress]
	if !ok {
		return nil, fmt.Errorf("peer %s not found", peerAddress)
	}
	result := make([]file.FileMeta, 0, len(files))
	for _, f := range files {
		result = append(result, f)
	}
	return result, nil
}

func (m *mockTransport) FetchFileMeta(peerAddress string, fileName string) (*file.FileMeta, error) {
	files, ok := m.Files[peerAddress]
	if !ok {
		return nil, fmt.Errorf("peer %s not found", peerAddress)
	}
	meta, ok := files[fileName]
	if !ok {
		return nil, fmt.Errorf("file %s not found on peer %s", fileName, peerAddress)
	}
	return &meta, nil
}

func (m *mockTransport) FetchChunk(peerAddress string, chunkHash string) ([]byte, error) {
	chunks, ok := m.ChunkData[peerAddress]
	if !ok {
		return nil, fmt.Errorf("peer %s not found", peerAddress)
	}
	data, ok := chunks[chunkHash]
	if !ok {
		return nil, fmt.Errorf("chunk %s not found on peer %s", chunkHash, peerAddress)
	}
	return data, nil
}

// TestDownloader_ConcurrentDownload verifies that the downloader can fetch
// all chunks concurrently from a mock peer and reassemble the file correctly.
func TestDownloader_ConcurrentDownload(t *testing.T) {
	originalData := []byte("This is a test file to verify the concurrent downloader works correctly with chunk integrity checks.")

	chunkSize := 20
	chunks, err := file.SplitFile(originalData, chunkSize)
	if err != nil {
		t.Fatalf("SplitFile failed: %v", err)
	}

	// Build mock chunk data
	chunkDataMap := make(map[string][]byte)
	chunkMetas := make([]file.ChunkMeta, len(chunks))
	for i, c := range chunks {
		hashStr := fmt.Sprintf("%x", c.Hash)
		chunkDataMap[hashStr] = c.Data
		chunkMetas[i] = file.ChunkMeta{
			Index: c.Index,
			Hash:  c.Hash,
		}
	}

	meta := file.FileMeta{
		FileName:    "test.txt",
		FileSize:    len(originalData),
		ChunkSize:   chunkSize,
		TotalChunks: len(chunks),
		Chunks:      chunkMetas,
	}

	mt := &mockTransport{
		Files: map[string]map[string]file.FileMeta{
			"localhost:9090": {"test.txt": meta},
		},
		ChunkData: map[string]map[string][]byte{
			"localhost:9090": chunkDataMap,
		},
	}

	localPeer := NewPeer("testPeer", 8080)
	downloader := NewDownloader(localPeer, mt)

	result, err := downloader.Download("test.txt", "localhost:9090")
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	if !bytes.Equal(result.Data, originalData) {
		t.Errorf("downloaded data mismatch: got %d bytes, want %d bytes", len(result.Data), len(originalData))
	}
}

// TestDownloader_IntegrityFailure verifies that the downloader detects corrupted chunks.
func TestDownloader_IntegrityFailure(t *testing.T) {
	originalData := []byte("Integrity test data for hash verification.")

	chunkSize := 10
	chunks, err := file.SplitFile(originalData, chunkSize)
	if err != nil {
		t.Fatalf("SplitFile failed: %v", err)
	}

	// Build mock chunk data with one corrupted chunk
	chunkDataMap := make(map[string][]byte)
	chunkMetas := make([]file.ChunkMeta, len(chunks))
	for i, c := range chunks {
		hashStr := fmt.Sprintf("%x", c.Hash)
		if i == 1 {
			// Corrupt the second chunk's data
			chunkDataMap[hashStr] = []byte("CORRUPTED!")
		} else {
			chunkDataMap[hashStr] = c.Data
		}
		chunkMetas[i] = file.ChunkMeta{
			Index: c.Index,
			Hash:  c.Hash,
		}
	}

	meta := file.FileMeta{
		FileName:    "corrupt.txt",
		FileSize:    len(originalData),
		ChunkSize:   chunkSize,
		TotalChunks: len(chunks),
		Chunks:      chunkMetas,
	}

	mt := &mockTransport{
		Files: map[string]map[string]file.FileMeta{
			"localhost:9090": {"corrupt.txt": meta},
		},
		ChunkData: map[string]map[string][]byte{
			"localhost:9090": chunkDataMap,
		},
	}

	localPeer := NewPeer("testPeer", 8080)
	downloader := NewDownloader(localPeer, mt)

	_, err = downloader.Download("corrupt.txt", "localhost:9090")
	if err == nil {
		t.Fatal("expected download to fail due to corrupted chunk, but it succeeded")
	}
	t.Logf("Download correctly failed with: %v", err)
}

// TestDownloader_FileNotFound verifies error handling when file doesn't exist on peer.
func TestDownloader_FileNotFound(t *testing.T) {
	mt := &mockTransport{
		Files:     map[string]map[string]file.FileMeta{},
		ChunkData: map[string]map[string][]byte{},
	}

	localPeer := NewPeer("testPeer", 8080)
	downloader := NewDownloader(localPeer, mt)

	_, err := downloader.Download("nonexistent.txt", "localhost:9090")
	if err == nil {
		t.Fatal("expected error for non-existent file, got nil")
	}
}

// TestDownloader_ChunkStoredLocally verifies chunks are stored in local peer after download.
func TestDownloader_ChunkStoredLocally(t *testing.T) {
	data := []byte("Store this locally")
	chunkSize := 10
	chunks, err := file.SplitFile(data, chunkSize)
	if err != nil {
		t.Fatalf("SplitFile failed: %v", err)
	}

	chunkDataMap := make(map[string][]byte)
	chunkMetas := make([]file.ChunkMeta, len(chunks))
	for i, c := range chunks {
		hashStr := fmt.Sprintf("%x", c.Hash)
		chunkDataMap[hashStr] = c.Data
		chunkMetas[i] = file.ChunkMeta{Index: c.Index, Hash: c.Hash}
	}

	meta := file.FileMeta{
		FileName:    "local.txt",
		FileSize:    len(data),
		ChunkSize:   chunkSize,
		TotalChunks: len(chunks),
		Chunks:      chunkMetas,
	}

	mt := &mockTransport{
		Files:     map[string]map[string]file.FileMeta{"localhost:9090": {"local.txt": meta}},
		ChunkData: map[string]map[string][]byte{"localhost:9090": chunkDataMap},
	}

	localPeer := NewPeer("testPeer", 8080)
	downloader := NewDownloader(localPeer, mt)

	_, err = downloader.Download("local.txt", "localhost:9090")
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}

	// Verify chunks are stored locally
	for _, c := range chunks {
		hashStr := fmt.Sprintf("%x", sha256.Sum256(c.Data))
		if _, exists := localPeer.ChunkDataStorage[hashStr]; !exists {
			t.Errorf("chunk %d not stored locally (hash: %s)", c.Index, hashStr)
		}
	}

	// Verify file meta is stored
	if _, exists := localPeer.SharedFiles["local.txt"]; !exists {
		t.Error("file metadata not stored in local peer")
	}
}
