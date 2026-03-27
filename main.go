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
	transportMode := flag.String("transport", "http", "Transport protocol for P2P communication: 'http' or 'tcp'")
	flag.Parse()

	// ── Initialize Peer ──
	p := peer.NewPeer(*peerName, *port)

	// ── Initialize Auth System ──
	userStore, err := auth.NewUserStore("data")
	if err != nil {
		log.Fatalf("Failed to initialize user store: %v", err)
	}

	sessionMgr := auth.NewSessionManager(24 * time.Hour)
	networkAuth := auth.NewNetworkAuth(*networkPassword)

	// ── Initialize Transport ──
	// The Transport interface abstracts P2P communication so the protocol
	// can be swapped without changing any handler or downloader logic.
	var t transport.Transport
	tcpPort := *port + 1000 // TCP P2P port = HTTP port + 1000

	switch *transportMode {
	case "tcp":
		t = transport.NewTCPTransport(*networkPassword)
		log.Printf("[%s] Using TCP transport for P2P communication (port %d)", *peerName, tcpPort)

		// Start the TCP listener for incoming P2P requests
		tcpListener := handler.NewTCPListener(p, networkAuth)
		go func() {
			tcpAddr := fmt.Sprintf(":%d", tcpPort)
			if err := tcpListener.Start(tcpAddr); err != nil {
				log.Fatalf("[%s] TCP listener failed: %v", p.PeerID, err)
			}
		}()

	case "http":
		t = transport.NewHTTPTransport(*networkPassword)
		log.Printf("[%s] Using HTTP transport for P2P communication", *peerName)

	default:
		log.Fatalf("Unknown transport mode: %s (use 'http' or 'tcp')", *transportMode)
	}

	// ── Peer Discovery ──
	if *peersList != "" {
		// When using TCP, peers connect to each other's TCP port
		var selfAddr string
		if *transportMode == "tcp" {
			selfAddr = fmt.Sprintf("localhost:%d", tcpPort)
		} else {
			selfAddr = fmt.Sprintf("localhost:%d", *port)
		}

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

	fmt.Printf("Starting Peer '%s' on port %d (transport: %s)\n", p.PeerID, p.Port, *transportMode)

	// ── Auth Pages & API ──
	http.HandleFunc("/login", handler.LoginPageHandler())
	http.HandleFunc("/api/auth/login", handler.AuthLoginHandler(p, userStore, sessionMgr))
	http.HandleFunc("/api/auth/register", handler.AuthRegisterHandler(userStore, sessionMgr, networkAuth))
	http.HandleFunc("/api/auth/logout", handler.AuthLogoutHandler(p, sessionMgr))
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

	// ── P2P API (HTTP — always available for HTTP-mode peers) ──
	http.HandleFunc("/api/register", handler.RegisterPeerHandler(p, networkAuth))
	http.HandleFunc("/api/info", handler.PeerInfoHandler(p))
	http.HandleFunc("/api/files", handler.FileListHandler(p))
	http.HandleFunc("/api/filemeta", handler.FileMetaHandler(p))
	http.HandleFunc("/api/chunk", handler.ChunkHandler(p))

	// ── Protected Download Operations ──
	// These handlers accept the Transport interface, so they work identically
	// whether the underlying transport is HTTP or TCP.
	http.HandleFunc("/api/browse", auth.RequireAuth(handler.BrowseFilesHandler(p, t), sessionMgr))
	http.HandleFunc("/api/download", auth.RequireAuth(handler.DownloadHandler(p, t), sessionMgr))

	// ── Start HTTP Server (always runs — needed for web UI) ──
	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[%s] HTTP server listening on %s", p.PeerID, addr)
	log.Printf("[%s] Login at http://localhost:%d/login", p.PeerID, *port)
	if *transportMode == "tcp" {
		log.Printf("[%s] TCP P2P listener on :%d", p.PeerID, tcpPort)
	}
	log.Fatal(http.ListenAndServe(addr, nil))
}
