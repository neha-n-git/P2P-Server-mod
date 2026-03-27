package transport

import (
	"encoding/json"
	"fmt"
	"net"
	"p2p/file"
	"time"
)

// TCPTransport implements the Transport interface using raw TCP connections
// with a custom binary wire protocol (see protocol.go).
//
// Compared to HTTPTransport:
//   - No HTTP header overhead per request (~500-800 bytes saved per round-trip)
//   - No URL parsing, status codes, or content-type negotiation
//   - Direct binary framing — faster serialization for chunk data
//   - Better suited for high-throughput P2P data transfer
type TCPTransport struct {
	NetworkPassword string
	DialTimeout     time.Duration
	ReadTimeout     time.Duration
}

// NewTCPTransport creates a new TCPTransport with sensible defaults.
func NewTCPTransport(networkPassword string) *TCPTransport {
	return &TCPTransport{
		NetworkPassword: networkPassword,
		DialTimeout:     10 * time.Second,
		ReadTimeout:     30 * time.Second,
	}
}

// dial opens a TCP connection to a peer with timeout.
func (t *TCPTransport) dial(peerAddress string) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", peerAddress, t.DialTimeout)
	if err != nil {
		return nil, fmt.Errorf("TCP dial to %s failed: %w", peerAddress, err)
	}
	return conn, nil
}

// roundTrip sends a request message and reads a response. It manages the
// full lifecycle of a TCP connection for a single request/response exchange.
func (t *TCPTransport) roundTrip(peerAddress string, msgType byte, requestPayload []byte) ([]byte, error) {
	conn, err := t.dial(peerAddress)
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Set deadline for the entire exchange
	if err := conn.SetDeadline(time.Now().Add(t.ReadTimeout)); err != nil {
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	// Send request
	if err := WriteMessage(conn, msgType, requestPayload); err != nil {
		return nil, fmt.Errorf("TCP send to %s failed: %w", peerAddress, err)
	}

	// Read response
	respType, respPayload, err := ReadMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("TCP read from %s failed: %w", peerAddress, err)
	}

	if respType == StatusError {
		return nil, fmt.Errorf("peer %s returned error: %s", peerAddress, string(respPayload))
	}

	return respPayload, nil
}

// RegisterWithPeer sends a registration request to a remote peer over TCP.
func (t *TCPTransport) RegisterWithPeer(peerAddress string, selfAddress string) error {
	payload := map[string]string{
		"address":          selfAddress,
		"network_password": t.NetworkPassword,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal registration payload: %w", err)
	}

	_, err = t.roundTrip(peerAddress, MsgRegister, body)
	return err
}

// FetchPeerInfo retrieves the identity/name of a remote peer over TCP.
func (t *TCPTransport) FetchPeerInfo(peerAddress string) (string, error) {
	respPayload, err := t.roundTrip(peerAddress, MsgPeerInfo, nil)
	if err != nil {
		return "", fmt.Errorf("failed to fetch peer info from %s: %w", peerAddress, err)
	}

	var info struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(respPayload, &info); err != nil {
		return "", fmt.Errorf("failed to decode peer info from %s: %w", peerAddress, err)
	}
	return info.Name, nil
}

// FetchFileList retrieves the list of shared files from a remote peer over TCP.
func (t *TCPTransport) FetchFileList(peerAddress string) ([]file.FileMeta, error) {
	respPayload, err := t.roundTrip(peerAddress, MsgFileList, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file list from %s: %w", peerAddress, err)
	}

	var files []file.FileMeta
	if err := json.Unmarshal(respPayload, &files); err != nil {
		return nil, fmt.Errorf("failed to decode file list from %s: %w", peerAddress, err)
	}
	return files, nil
}

// FetchFileMeta retrieves metadata for a specific file from a remote peer over TCP.
func (t *TCPTransport) FetchFileMeta(peerAddress string, fileName string) (*file.FileMeta, error) {
	reqPayload, err := json.Marshal(map[string]string{"name": fileName})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal file meta request: %w", err)
	}

	respPayload, err := t.roundTrip(peerAddress, MsgFileMeta, reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file meta from %s: %w", peerAddress, err)
	}

	var meta file.FileMeta
	if err := json.Unmarshal(respPayload, &meta); err != nil {
		return nil, fmt.Errorf("failed to decode file meta from %s: %w", peerAddress, err)
	}
	return &meta, nil
}

// FetchChunk downloads raw chunk data identified by its hash from a remote peer over TCP.
// Unlike HTTP where the chunk is wrapped in an HTTP response with headers,
// here the raw bytes come directly in the protocol payload — zero overhead.
func (t *TCPTransport) FetchChunk(peerAddress string, chunkHash string) ([]byte, error) {
	reqPayload, err := json.Marshal(map[string]string{"hash": chunkHash})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal chunk request: %w", err)
	}

	respPayload, err := t.roundTrip(peerAddress, MsgChunk, reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chunk %s from %s: %w", chunkHash, peerAddress, err)
	}

	return respPayload, nil
}
