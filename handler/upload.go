package handler

import (
	"fmt"
	"io"
	"net/http"
	"p2p/file"
	"p2p/peer"
)

// UploadHandler handles file uploads from the web interface
func UploadHandler(p *peer.Peer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// 1. Parse Multipart Form
		err := r.ParseMultipartForm(10 << 20) // 10 MB limit
		if err != nil {
			http.Error(w, "Error parsing form", http.StatusBadRequest)
			return
		}

		fileVal, handler, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "Error retrieving file", http.StatusBadRequest)
			return
		}
		defer fileVal.Close()

		// 2. Read File Content
		data, err := io.ReadAll(fileVal)
		if err != nil {
			http.Error(w, "Error reading file", http.StatusInternalServerError)
			return
		}

		// 3. Chunk the File
		const chunkSize = 1024 * 1024 // 1MB chunks (adjust as needed)
		chunks, err := file.SplitFile(data, chunkSize)
		if err != nil {
			http.Error(w, "Error splitting file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// 4. Create File Metadata
		fileMeta := file.FileMeta{
			FileName:    handler.Filename,
			FileSize:    len(data),
			ChunkSize:   chunkSize,
			TotalChunks: len(chunks),
			Chunks:      make([]file.ChunkMeta, len(chunks)),
		}

		for i, c := range chunks {
			fileMeta.Chunks[i] = file.ChunkMeta{
				Index: c.Index,
				Hash:  c.Hash,
			}

			// Store chunk data in local storage
			hashStr := fmt.Sprintf("%x", c.Hash)
			p.Mu.Lock()
			p.ChunkDataStorage[hashStr] = c.Data
			p.Mu.Unlock()
		}

		// 5. Register File in Peer
		p.AddFile(fileMeta)

		fmt.Printf("File processed: %s (%d bytes, %d chunks)\n", fileMeta.FileName, fileMeta.FileSize, fileMeta.TotalChunks)
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("File '%s' uploaded and split into %d chunks", fileMeta.FileName, fileMeta.TotalChunks)))
	}
}
