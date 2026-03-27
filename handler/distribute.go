package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"p2p/peer"
	"p2p/registry"
	"p2p/transport"
	"path/filepath"
)

// DownloadPageHandler serves the download page HTML.
func DownloadPageHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/download.html")
	}
}

// BrowseFilesHandler queries all known peers for their file lists
// and returns a consolidated JSON response.
func BrowseFilesHandler(p *peer.Peer, t transport.Transport) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		type PeerFile struct {
			PeerAddress string `json:"peer_address"`
			PeerName    string `json:"peer_name"`
			FileName    string `json:"file_name"`
			FileSize    int    `json:"file_size"`
			TotalChunks int    `json:"total_chunks"`
			PeersCount  int    `json:"peers_count"`
		}

		selfAddr := fmt.Sprintf("localhost:%d", p.Port)
		peers := registry.GetPeers()
		
		seenFiles := make(map[string]*PeerFile)
		peerNames := make(map[string]string)

		for _, peerAddr := range peers {
			if peerAddr == selfAddr {
				continue // skip self
			}
			files, err := t.FetchFileList(peerAddr)
			if err != nil {
				log.Printf("[%s] Warning: could not fetch file list from %s: %v", p.PeerID, peerAddr, err)
				continue
			}

			// Try to get peer name
			peerName, err := t.FetchPeerInfo(peerAddr)
			if err != nil || peerName == "" {
				peerName = "Unknown Peer"
			}
			peerNames[peerAddr] = peerName

			for _, f := range files {
				if existing, ok := seenFiles[f.FileName]; ok {
					existing.PeersCount++
				} else {
					seenFiles[f.FileName] = &PeerFile{
						PeerAddress: peerAddr,
						PeerName:    peerName,
						FileName:    f.FileName,
						FileSize:    f.FileSize,
						TotalChunks: f.TotalChunks,
						PeersCount:  1,
					}
				}
			}
		}

		var allFiles []PeerFile
		for _, pf := range seenFiles {
			allFiles = append(allFiles, *pf)
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(allFiles); err != nil {
			log.Printf("[%s] Error encoding browse files response: %v", p.PeerID, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}
}

// DownloadHandler initiates a concurrent download of a file from peers.
// Query parameters: ?name=<filename>&peer=<peer_address>
func DownloadHandler(p *peer.Peer, t transport.Transport) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		fileName := r.URL.Query().Get("name")
		peerAddr := r.URL.Query().Get("peer")

		if fileName == "" || peerAddr == "" {
			http.Error(w, "Query parameters 'name' and 'peer' are required", http.StatusBadRequest)
			return
		}

		// Use the downloader
		downloader := peer.NewDownloader(p, t)
		result, err := downloader.Download(fileName, peerAddr)
		if err != nil {
			log.Printf("[%s] Download failed for '%s' from %s: %v", p.PeerID, fileName, peerAddr, err)
			http.Error(w, fmt.Sprintf("Download failed: %v", err), http.StatusInternalServerError)
			return
		}

		// Save reconstructed file to downloads directory
		downloadDir := "downloads"
		if err := os.MkdirAll(downloadDir, 0755); err != nil {
			http.Error(w, fmt.Sprintf("Failed to create download directory: %v", err), http.StatusInternalServerError)
			return
		}

		outPath := filepath.Join(downloadDir, fileName)
		if err := os.WriteFile(outPath, result.Data, 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save file: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("[%s] Successfully downloaded '%s' (%d bytes) using %d peers", p.PeerID, fileName, len(result.Data), len(result.PeersUsed))

		// Resolve peer names for UI display
		peerNames := make(map[string]string)
		for _, addr := range result.PeersUsed {
			name, err := t.FetchPeerInfo(addr)
			if err != nil || name == "" {
				name = "Unknown Peer"
			}
			peerNames[addr] = name
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":        "success",
			"file_name":     fileName,
			"file_size":     len(result.Data),
			"saved_to":      outPath,
			"chunk_sources": result.ChunkSources,
			"peers_used":    result.PeersUsed,
			"peer_names":    peerNames,
		})
	}
}
