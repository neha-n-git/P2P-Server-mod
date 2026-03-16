package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"

	"p2p/handler"
	"p2p/peer"
	"p2p/registry"
	"p2p/transport"
)

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	peerName := flag.String("name", "peer1", "Name of the peer")
	peersList := flag.String("peers", "", "Comma-separated list of known peer addresses (e.g., localhost:8081)")
	flag.Parse()

	p := peer.NewPeer(*peerName, *port)

	t := transport.NewHTTPTransport()

	if *peersList != "" {
		selfAddr := fmt.Sprintf("localhost:%d", *port)

		// Collect peers from comma-separated flag value
		allPeerAddrs := strings.Split(*peersList, ",")

		// Also include any remaining positional args as peers
		// (supports: -peers localhost:8080 localhost:8081)
		allPeerAddrs = append(allPeerAddrs, flag.Args()...)

		for _, peerAddr := range allPeerAddrs {
			addr := strings.TrimSpace(peerAddr)
			if addr == "" {
				continue
			}
			registry.AddPeer(addr)
			// Attempt to register ourselves with the remote peer
			if err := t.RegisterWithPeer(addr, selfAddr); err != nil {
				log.Printf("[%s] Warning: could not register with peer %s: %v", p.PeerID, addr, err)
			} else {
				log.Printf("[%s] Registered with peer %s", p.PeerID, addr)
			}
		}
	}

	fmt.Printf("Starting Peer '%s' on port %d\n", p.PeerID, p.Port)

	// Setup Routes — Web Pages
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/home.html")
	})
	http.HandleFunc("/upload", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/upload.html")
	})
	http.HandleFunc("/download", handler.DownloadPageHandler())

	// Setup Routes — File Operations
	http.HandleFunc("/api/upload", handler.UploadHandler(p))

	// Setup Routes — P2P API (JSON-based)
	http.HandleFunc("/api/register", handler.RegisterHandler(p))
	http.HandleFunc("/api/files", handler.FileListHandler(p))
	http.HandleFunc("/api/filemeta", handler.FileMetaHandler(p))
	http.HandleFunc("/api/chunk", handler.ChunkHandler(p))

	// Setup Routes — Download Operations
	http.HandleFunc("/api/browse", handler.BrowseFilesHandler(p, t))
	http.HandleFunc("/api/download", handler.DownloadHandler(p, t))

	// Start Server
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[%s] Server listening on %s", p.PeerID, addr)
	log.Fatal(http.ListenAndServe(addr, nil))
}
