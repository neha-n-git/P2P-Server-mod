package file

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"testing"
)

// TestSplitFile_BasicSplit verifies that SplitFile produces the correct number of chunks
// and that each chunk has the correct data.
func TestSplitFile_BasicSplit(t *testing.T) {
	data := []byte("Hello, P2P World! This is a test file for chunking.")
	chunkSize := 10

	chunks, err := splitAndVerify(t, data, chunkSize)
	if err != nil {
		t.Fatalf("SplitFile failed: %v", err)
	}

	// Expected chunks: ceil(51 / 10) = 6
	expectedChunks := 6
	if len(chunks) != expectedChunks {
		t.Errorf("expected %d chunks, got %d", expectedChunks, len(chunks))
	}

	// Verify first chunk data
	if !bytes.Equal(chunks[0].Data, data[:10]) {
		t.Errorf("first chunk data mismatch: got %q, want %q", chunks[0].Data, data[:10])
	}

	// Verify last chunk (partial)
	lastChunk := chunks[len(chunks)-1]
	if !bytes.Equal(lastChunk.Data, data[50:]) {
		t.Errorf("last chunk data mismatch: got %q, want %q", lastChunk.Data, data[50:])
	}
}

// TestSplitFile_InvalidChunkSize verifies that SplitFile returns an error for invalid input.
func TestSplitFile_InvalidChunkSize(t *testing.T) {
	data := []byte("some data")

	_, err := SplitFile(data, 0)
	if err == nil {
		t.Fatal("expected error for chunkSize = 0, got nil")
	}

	_, err = SplitFile(data, -5)
	if err == nil {
		t.Fatal("expected error for negative chunkSize, got nil")
	}
}

// TestSplitFile_EmptyData verifies that SplitFile handles empty data.
func TestSplitFile_EmptyData(t *testing.T) {
	chunks, err := SplitFile([]byte{}, 10)
	if err != nil {
		t.Fatalf("SplitFile failed: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty data, got %d", len(chunks))
	}
}

// TestSplitFile_ExactDivisible verifies split when data length is exact multiple of chunkSize.
func TestSplitFile_ExactDivisible(t *testing.T) {
	data := []byte("1234567890") // 10 bytes
	chunks, err := SplitFile(data, 5)
	if err != nil {
		t.Fatalf("SplitFile failed: %v", err)
	}
	if len(chunks) != 2 {
		t.Errorf("expected 2 chunks, got %d", len(chunks))
	}
}

// TestSplitFile_HashIntegrity verifies that each chunk's hash matches SHA-256 of its data.
func TestSplitFile_HashIntegrity(t *testing.T) {
	data := []byte("Integrity check data for hashing verification test case.")
	chunks, err := SplitFile(data, 15)
	if err != nil {
		t.Fatalf("SplitFile failed: %v", err)
	}

	for i, chunk := range chunks {
		expectedHash := sha256.Sum256(chunk.Data)
		if chunk.Hash != expectedHash {
			t.Errorf("chunk %d hash mismatch: got %x, want %x", i, chunk.Hash, expectedHash)
		}
	}
}

// TestMergeChunks_CorrectOrder verifies that MergeChunks reassembles in index order.
func TestMergeChunks_CorrectOrder(t *testing.T) {
	data := []byte("ABCDE12345FGHIJ")
	chunks, err := SplitFile(data, 5)
	if err != nil {
		t.Fatalf("SplitFile failed: %v", err)
	}

	// Reverse the chunks to simulate out-of-order arrival
	reversed := make([]Chunk, len(chunks))
	for i, c := range chunks {
		reversed[len(chunks)-1-i] = c
	}

	merged, err := MergeChunks(reversed)
	if err != nil {
		t.Fatalf("MergeChunks failed: %v", err)
	}

	if !bytes.Equal(merged, data) {
		t.Errorf("merged data mismatch: got %q, want %q", merged, data)
	}
}

// TestSplitAndMerge_RoundTrip verifies that split + merge produces the original data.
func TestSplitAndMerge_RoundTrip(t *testing.T) {
	data := []byte("This is a complete round-trip test to verify data integrity through split and merge operations.")
	chunks, err := SplitFile(data, 20)
	if err != nil {
		t.Fatalf("SplitFile failed: %v", err)
	}

	merged, err := MergeChunks(chunks)
	if err != nil {
		t.Fatalf("MergeChunks failed: %v", err)
	}

	if !bytes.Equal(merged, data) {
		t.Errorf("round-trip failed: got %d bytes, want %d bytes", len(merged), len(data))
	}
}

// TestChunkMetaJSON_Serialization verifies JSON marshalling and unmarshalling of ChunkMeta.
func TestChunkMetaJSON_Serialization(t *testing.T) {
	original := ChunkMeta{
		Index: 42,
		Hash:  sha256.Sum256([]byte("test data")),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded ChunkMeta
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.Index != original.Index {
		t.Errorf("Index mismatch: got %d, want %d", decoded.Index, original.Index)
	}
	if decoded.Hash != original.Hash {
		t.Errorf("Hash mismatch: got %x, want %x", decoded.Hash, original.Hash)
	}
}

// TestFileMetaJSON_Serialization verifies JSON marshalling and unmarshalling of FileMeta.
func TestFileMetaJSON_Serialization(t *testing.T) {
	original := FileMeta{
		FileName:    "test.txt",
		FileSize:    1024,
		ChunkSize:   256,
		TotalChunks: 4,
		Chunks: []ChunkMeta{
			{Index: 0, Hash: sha256.Sum256([]byte("chunk0"))},
			{Index: 1, Hash: sha256.Sum256([]byte("chunk1"))},
		},
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var decoded FileMeta
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if decoded.FileName != original.FileName {
		t.Errorf("FileName mismatch: got %q, want %q", decoded.FileName, original.FileName)
	}
	if decoded.FileSize != original.FileSize {
		t.Errorf("FileSize mismatch: got %d, want %d", decoded.FileSize, original.FileSize)
	}
	if len(decoded.Chunks) != len(original.Chunks) {
		t.Fatalf("Chunks length mismatch: got %d, want %d", len(decoded.Chunks), len(original.Chunks))
	}
	for i := range decoded.Chunks {
		if decoded.Chunks[i].Hash != original.Chunks[i].Hash {
			t.Errorf("Chunk %d hash mismatch", i)
		}
	}
}

// splitAndVerify is a test helper that splits data and validates chunk indices.
func splitAndVerify(t *testing.T, data []byte, chunkSize int) ([]Chunk, error) {
	t.Helper()
	chunks, err := SplitFile(data, chunkSize)
	if err != nil {
		return nil, err
	}
	for i, c := range chunks {
		if c.Index != i {
			t.Errorf("chunk %d has incorrect index %d", i, c.Index)
		}
	}
	return chunks, nil
}
