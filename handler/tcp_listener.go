package handler

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"p2p/auth"
	"p2p/peer"
	"p2p/registry"
	"p2p/transport"
)

// TCPListener runs a TCP server that handles incoming P2P protocol requests.
// This is the server-side counterpart to TCPTransport (the client side).
//
// It mirrors the functionality of the HTTP API handlers in api.go,
// but serves them over raw TCP using the binary wire protocol defined in
// transport/protocol.go — avoiding HTTP overhead entirely for peer-to-peer traffic.
type TCPListener struct {
	Peer        *peer.Peer
	NetworkAuth *auth.NetworkAuth
	listener    net.Listener
}

// NewTCPListener creates a new TCPListener.
func NewTCPListener(p *peer.Peer, na *auth.NetworkAuth) *TCPListener {
	return &TCPListener{
		Peer:        p,
		NetworkAuth: na,
	}
}

// Start begins listening for incoming TCP P2P connections on the given address.
// This should be called in a goroutine — it blocks until the listener is closed.
func (tl *TCPListener) Start(address string) error {
	var err error
	tl.listener, err = net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("TCP listener failed to start on %s: %w", address, err)
	}

	log.Printf("[TCP] P2P protocol listener started on %s", address)

	for {
		conn, err := tl.listener.Accept()
		if err != nil {
			// If the listener was closed, exit gracefully
			if opErr, ok := err.(*net.OpError); ok && !opErr.Temporary() {
				return nil
			}
			log.Printf("[TCP] Accept error: %v", err)
			continue
		}
		go tl.handleConnection(conn)
	}
}

// Stop gracefully shuts down the TCP listener.
func (tl *TCPListener) Stop() error {
	if tl.listener != nil {
		return tl.listener.Close()
	}
	return nil
}

// handleConnection processes a single incoming TCP connection.
// It reads one request message, dispatches to the appropriate handler,
// and writes the response.
func (tl *TCPListener) handleConnection(conn net.Conn) {
	defer conn.Close()

	msgType, payload, err := transport.ReadMessage(conn)
	if err != nil {
		log.Printf("[TCP] Error reading message: %v", err)
		return
	}

	var respPayload []byte
	var respErr error

	switch msgType {
	case transport.MsgRegister:
		respPayload, respErr = tl.handleRegister(payload)
	case transport.MsgPeerInfo:
		respPayload, respErr = tl.handlePeerInfo()
	case transport.MsgFileList:
		respPayload, respErr = tl.handleFileList()
	case transport.MsgFileMeta:
		respPayload, respErr = tl.handleFileMeta(payload)
	case transport.MsgChunk:
		respPayload, respErr = tl.handleChunk(payload)
	default:
		respErr = fmt.Errorf("unknown message type: 0x%02x", msgType)
	}

	if respErr != nil {
		// Send error response
		errMsg := []byte(respErr.Error())
		if writeErr := transport.WriteMessage(conn, transport.StatusError, errMsg); writeErr != nil {
			log.Printf("[TCP] Error sending error response: %v", writeErr)
		}
		return
	}

	// Send success response
	if err := transport.WriteMessage(conn, transport.StatusOK, respPayload); err != nil {
		log.Printf("[TCP] Error sending response: %v", err)
	}
}

// ── Handler implementations (mirror api.go but over TCP) ───────────────────

func (tl *TCPListener) handleRegister(payload []byte) ([]byte, error) {
	var req struct {
		Address         string `json:"address"`
		NetworkPassword string `json:"network_password"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("invalid register request: %w", err)
	}

	if req.Address == "" {
		return nil, fmt.Errorf("address field is required")
	}

	if !tl.NetworkAuth.ValidateNetworkPassword(req.NetworkPassword) {
		log.Printf("[%s] TCP: Rejected peer registration from %s — invalid network password", tl.Peer.PeerID, req.Address)
		return nil, fmt.Errorf("invalid network password — access denied")
	}

	registry.AddPeer(req.Address)
	log.Printf("[%s] TCP: Registered peer: %s", tl.Peer.PeerID, req.Address)

	return json.Marshal(map[string]string{"status": "registered"})
}

func (tl *TCPListener) handlePeerInfo() ([]byte, error) {
	tl.Peer.Mu.Lock()
	displayName := tl.Peer.ActiveUser
	tl.Peer.Mu.Unlock()
	if displayName == "" {
		displayName = tl.Peer.PeerID
	}

	return json.Marshal(map[string]string{"name": displayName})
}

func (tl *TCPListener) handleFileList() ([]byte, error) {
	tl.Peer.Mu.Lock()
	files := make([]interface{}, 0, len(tl.Peer.SharedFiles))
	for _, meta := range tl.Peer.SharedFiles {
		files = append(files, meta)
	}
	tl.Peer.Mu.Unlock()

	return json.Marshal(files)
}

func (tl *TCPListener) handleFileMeta(payload []byte) ([]byte, error) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("invalid file meta request: %w", err)
	}

	if req.Name == "" {
		return nil, fmt.Errorf("file name is required")
	}

	tl.Peer.Mu.Lock()
	meta, exists := tl.Peer.SharedFiles[req.Name]
	tl.Peer.Mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("file '%s' not found", req.Name)
	}

	return json.Marshal(meta)
}

func (tl *TCPListener) handleChunk(payload []byte) ([]byte, error) {
	var req struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(payload, &req); err != nil {
		return nil, fmt.Errorf("invalid chunk request: %w", err)
	}

	if req.Hash == "" {
		return nil, fmt.Errorf("chunk hash is required")
	}

	tl.Peer.Mu.Lock()
	data, exists := tl.Peer.ChunkDataStorage[req.Hash]
	tl.Peer.Mu.Unlock()

	if !exists {
		return nil, fmt.Errorf("chunk with hash '%s' not found", req.Hash)
	}

	// Return raw chunk bytes directly — no JSON wrapping for binary data
	return data, nil
}
