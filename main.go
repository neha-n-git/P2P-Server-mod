package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"p2p/auth"
	"p2p/handler"
	"p2p/peer"
	"p2p/registry"
	"p2p/transport"
)

func main() {
	port := flag.Int("port", 8080, "Port to listen on")
	peerName := flag.String("name", "peer1", "Name of the peer")
	peersList := flag.String("peers", "", "Comma-separated list of known peer addresses (e.g., localhost:8081)")
	networkPassword := flag.String("network-password", "", "Shared password for P2P network authentication (peers must provide this to join)")
	flag.Parse()

	// ── Initialize Peer ──
	p := peer.NewPeer(*peerName, *port)
	t := transport.NewHTTPTransport(*networkPassword)

	// ── Initialize Auth System ──
	userStore, err := auth.NewUserStore("data")
	if err != nil {
		log.Fatalf("Failed to initialize user store: %v", err)
	}

	sessionMgr := auth.NewSessionManager(24 * time.Hour)
	networkAuth := auth.NewNetworkAuth(*networkPassword)

	// ── Peer Discovery ──
	if *peersList != "" {
		selfAddr := fmt.Sprintf("localhost:%d", *port)

		allPeerAddrs := strings.Split(*peersList, ",")
		allPeerAddrs = append(allPeerAddrs, flag.Args()...)

		// Launch peer registration concurrently using goroutines
		for _, peerAddr := range allPeerAddrs {
			addr := strings.TrimSpace(peerAddr)
			if addr == "" {
				continue
			}
			registry.AddPeer(addr)

			// Register with each peer concurrently
			go func(address string) {
				if err := t.RegisterWithPeer(address, selfAddr); err != nil {
					log.Printf("[%s] Warning: could not register with peer %s: %v", p.PeerID, address, err)
				} else {
					log.Printf("[%s] Registered with peer %s", p.PeerID, address)
				}
			}(addr)
		}
	}

	fmt.Printf("Starting Peer '%s' on port %d\n", p.PeerID, p.Port)

	// ── Auth Routes (unprotected) ──
	http.HandleFunc("/login", handler.LoginPageHandler())
	http.HandleFunc("/api/auth/login", handler.AuthLoginHandler(userStore, sessionMgr))
	http.HandleFunc("/api/auth/register", handler.AuthRegisterHandler(userStore, sessionMgr, networkAuth))
	http.HandleFunc("/api/auth/logout", handler.AuthLogoutHandler(sessionMgr))
	http.HandleFunc("/api/auth/status", handler.AuthStatusHandler(userStore, sessionMgr))

	// ── Static Assets (unprotected) ──
	http.HandleFunc("/image.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/image.png")
	})

	// ── Protected Web Pages ──
	http.HandleFunc("/", auth.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/home.html")
	}, sessionMgr))
	http.HandleFunc("/upload", auth.RequireAuth(func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/upload.html")
	}, sessionMgr))
	http.HandleFunc("/download", auth.RequireAuth(handler.DownloadPageHandler(), sessionMgr))

	// ── Protected File Operations ──
	http.HandleFunc("/api/upload", auth.RequireAuth(handler.UploadHandler(p), sessionMgr))

	// ── Protected P2P API ──
	http.HandleFunc("/api/register", handler.RegisterPeerHandler(p, networkAuth))
	http.HandleFunc("/api/files", handler.FileListHandler(p))
	http.HandleFunc("/api/filemeta", handler.FileMetaHandler(p))
	http.HandleFunc("/api/chunk", handler.ChunkHandler(p))

	// ── Protected Download Operations ──
	http.HandleFunc("/api/browse", auth.RequireAuth(handler.BrowseFilesHandler(p, t), sessionMgr))
	http.HandleFunc("/api/download", auth.RequireAuth(handler.DownloadHandler(p, t), sessionMgr))

	// ── Start Server ──
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[%s] Server listening on %s", p.PeerID, addr)
	log.Printf("[%s] Login at http://localhost:%d/login", p.PeerID, *port)
	log.Fatal(http.ListenAndServe(addr, nil))
}
