package file

import (
	"crypto/sha256"
	"errors"
	"math"
	"sort"
)

type Chunk struct {
	Index int
	Data  []byte
	Hash  [32]byte
}

// SplitFile splits a byte slice into chunks of a given size
func SplitFile(data []byte, chunkSize int) ([]Chunk, error) {
	if chunkSize <= 0 {
		return nil, errors.New("chunk size must be greater than 0")
	}

	totalChunks := int(math.Ceil(float64(len(data)) / float64(chunkSize)))
	chunks := make([]Chunk, 0, totalChunks)

	for i := 0; i < totalChunks; i++ {
		start := i * chunkSize
		end := start + chunkSize
		if end > len(data) {
			end = len(data)
		}

		chunkData := data[start:end]
		hash := sha256.Sum256(chunkData)

		chunks = append(chunks, Chunk{
			Index: i,
			Data:  chunkData,
			Hash:  hash,
		})
	}

	return chunks, nil
}

// MergeChunks reassembles chunks into a single byte slice
func MergeChunks(chunks []Chunk) ([]byte, error) {
	// Sort chunks by index to ensure correct order
	sort.Slice(chunks, func(i, j int) bool {
		return chunks[i].Index < chunks[j].Index
	})

	var data []byte
	for _, chunk := range chunks {
		data = append(data, chunk.Data...)
	}

	return data, nil
}
