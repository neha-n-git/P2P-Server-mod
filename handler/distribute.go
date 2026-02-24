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
			FileName    string `json:"file_name"`
			FileSize    int    `json:"file_size"`
			TotalChunks int    `json:"total_chunks"`
		}

		selfAddr := fmt.Sprintf("localhost:%d", p.Port)
		peers := registry.GetPeers()
		var allFiles []PeerFile

		for _, peerAddr := range peers {
			if peerAddr == selfAddr {
				continue // skip self
			}
			files, err := t.FetchFileList(peerAddr)
			if err != nil {
				log.Printf("[%s] Warning: could not fetch file list from %s: %v", p.PeerID, peerAddr, err)
				continue
			}
			for _, f := range files {
				allFiles = append(allFiles, PeerFile{
					PeerAddress: peerAddr,
					FileName:    f.FileName,
					FileSize:    f.FileSize,
					TotalChunks: f.TotalChunks,
				})
			}
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
		data, err := downloader.Download(fileName, peerAddr)
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
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			http.Error(w, fmt.Sprintf("Failed to save file: %v", err), http.StatusInternalServerError)
			return
		}

		log.Printf("[%s] Successfully downloaded '%s' (%d bytes) from %s", p.PeerID, fileName, len(data), peerAddr)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status":    "success",
			"file_name": fileName,
			"file_size": len(data),
			"saved_to":  outPath,
		})
	}
}
