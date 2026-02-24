// file and chunk meta
package file

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// ChunkMeta holds metadata about a single chunk.
type ChunkMeta struct {
	Index int      `json:"index"`
	Hash  [32]byte `json:"-"` // excluded from default JSON; use custom methods
}

// chunkMetaJSON is a helper for JSON serialization of ChunkMeta.
type chunkMetaJSON struct {
	Index int    `json:"index"`
	Hash  string `json:"hash"` // hex-encoded hash string
}

// MarshalJSON implements custom JSON marshalling for ChunkMeta.
func (cm ChunkMeta) MarshalJSON() ([]byte, error) {
	return json.Marshal(chunkMetaJSON{
		Index: cm.Index,
		Hash:  fmt.Sprintf("%x", cm.Hash),
	})
}

// UnmarshalJSON implements custom JSON unmarshalling for ChunkMeta.
func (cm *ChunkMeta) UnmarshalJSON(data []byte) error {
	var aux chunkMetaJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return fmt.Errorf("failed to unmarshal ChunkMeta: %w", err)
	}
	cm.Index = aux.Index

	hashBytes, err := hex.DecodeString(aux.Hash)
	if err != nil {
		return fmt.Errorf("invalid hash hex string %q: %w", aux.Hash, err)
	}
	if len(hashBytes) != 32 {
		return fmt.Errorf("expected 32 bytes for hash, got %d", len(hashBytes))
	}
	copy(cm.Hash[:], hashBytes)
	return nil
}

// FileMeta holds metadata about an entire file.
type FileMeta struct {
	FileName    string      `json:"file_name"`
	FileSize    int         `json:"file_size"`
	ChunkSize   int         `json:"chunk_size"`
	TotalChunks int         `json:"total_chunks"`
	Chunks      []ChunkMeta `json:"chunks"`
}
