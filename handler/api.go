package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"p2p/peer"
	"p2p/registry"
)

// RegisterHandler handles incoming peer registration requests.
// Remote peers POST their address here so we can add them to our local registry.
func RegisterHandler(p *peer.Peer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var payload struct {
			Address string `json:"address"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request body: %v", err), http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		if payload.Address == "" {
			http.Error(w, "Address field is required", http.StatusBadRequest)
			return
		}

		registry.AddPeer(payload.Address)
		log.Printf("[%s] Registered peer: %s", p.PeerID, payload.Address)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "registered"})
	}
}

// FileListHandler returns JSON list of all files this peer has.
func FileListHandler(p *peer.Peer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		p.Mu.Lock()
		files := make([]interface{}, 0, len(p.SharedFiles))
		for _, meta := range p.SharedFiles {
			files = append(files, meta)
		}
		p.Mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(files); err != nil {
			log.Printf("[%s] Error encoding file list: %v", p.PeerID, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

// FileMetaHandler returns JSON metadata for a specific file.
// Query parameter: ?name=<filename>
func FileMetaHandler(p *peer.Peer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fileName := r.URL.Query().Get("name")
		if fileName == "" {
			http.Error(w, "Query parameter 'name' is required", http.StatusBadRequest)
			return
		}

		p.Mu.Lock()
		meta, exists := p.SharedFiles[fileName]
		p.Mu.Unlock()

		if !exists {
			http.Error(w, fmt.Sprintf("File '%s' not found", fileName), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(meta); err != nil {
			log.Printf("[%s] Error encoding file meta for '%s': %v", p.PeerID, fileName, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

// ChunkHandler serves raw chunk data by hash.
// Query parameter: ?hash=<hex-encoded hash>
func ChunkHandler(p *peer.Peer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		hashStr := r.URL.Query().Get("hash")
		if hashStr == "" {
			http.Error(w, "Query parameter 'hash' is required", http.StatusBadRequest)
			return
		}

		p.Mu.Lock()
		data, exists := p.ChunkDataStorage[hashStr]
		p.Mu.Unlock()

		if !exists {
			http.Error(w, fmt.Sprintf("Chunk with hash '%s' not found", hashStr), http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/octet-stream")
		if _, err := w.Write(data); err != nil {
			log.Printf("[%s] Error writing chunk data for hash '%s': %v", p.PeerID, hashStr, err)
		}
	}
}
