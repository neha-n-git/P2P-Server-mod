// Package transport — protocol.go defines the binary wire protocol used by the
// TCP transport for peer-to-peer communication.
//
// Wire format (both request and response):
//
//	┌──────────┬────────────────┬─────────────────┐
//	│ Type     │ Payload Length │ Payload (JSON)   │
//	│ 1 byte   │ 4 bytes (BE)  │ N bytes          │
//	└──────────┴────────────────┴─────────────────┘
//
// This is a simple, length-prefixed protocol that avoids the overhead of HTTP
// headers while remaining debuggable (JSON payloads).
package transport

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

// ── Message Types ──────────────────────────────────────────────────────────

const (
	MsgRegister byte = 0x01 // Register self with a remote peer
	MsgPeerInfo byte = 0x02 // Request peer identity/name
	MsgFileList byte = 0x03 // Request list of shared files
	MsgFileMeta byte = 0x04 // Request metadata for a specific file
	MsgChunk    byte = 0x05 // Request raw chunk data by hash
)

// ── Response Status Codes ──────────────────────────────────────────────────

const (
	StatusOK    byte = 0x00
	StatusError byte = 0x01
)

// ── Max Payload Size ───────────────────────────────────────────────────────

// MaxPayloadSize limits individual messages to 64 MB to prevent memory abuse.
const MaxPayloadSize = 64 * 1024 * 1024

// ── Wire Helpers ───────────────────────────────────────────────────────────

// WriteMessage writes a single framed message (type + length + payload) to a
// net.Conn. This is used by both the client (to send requests) and the server
// (to send responses).
func WriteMessage(conn net.Conn, msgType byte, payload []byte) error {
	// Header: [1 byte type][4 bytes length]
	header := make([]byte, 5)
	header[0] = msgType
	binary.BigEndian.PutUint32(header[1:5], uint32(len(payload)))

	// Write header
	if _, err := conn.Write(header); err != nil {
		return fmt.Errorf("failed to write message header: %w", err)
	}
	// Write payload
	if len(payload) > 0 {
		if _, err := conn.Write(payload); err != nil {
			return fmt.Errorf("failed to write message payload: %w", err)
		}
	}
	return nil
}

// ReadMessage reads a single framed message from a net.Conn.
// Returns the message type and payload bytes.
func ReadMessage(conn net.Conn) (byte, []byte, error) {
	// Read header (5 bytes)
	header := make([]byte, 5)
	if _, err := io.ReadFull(conn, header); err != nil {
		return 0, nil, fmt.Errorf("failed to read message header: %w", err)
	}

	msgType := header[0]
	payloadLen := binary.BigEndian.Uint32(header[1:5])

	if payloadLen > MaxPayloadSize {
		return 0, nil, fmt.Errorf("payload too large: %d bytes (max %d)", payloadLen, MaxPayloadSize)
	}

	// Read payload
	payload := make([]byte, payloadLen)
	if payloadLen > 0 {
		if _, err := io.ReadFull(conn, payload); err != nil {
			return 0, nil, fmt.Errorf("failed to read message payload: %w", err)
		}
	}

	return msgType, payload, nil
}
