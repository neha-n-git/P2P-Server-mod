// Package transport defines the interface for peer-to-peer communication.
// By abstracting communication behind an interface, the underlying mechanism
// (HTTP, gRPC, TCP, etc.) can be swapped without affecting the core system logic.
package transport

import "p2p/file"

// Transport defines the contract for communication between peers.
// Any concrete implementation must satisfy this interface, enabling
// modular and testable peer communication.
type Transport interface {
	// RegisterWithPeer sends a registration request to a remote peer,
	// informing it of our address so it can communicate back.
	RegisterWithPeer(peerAddress string, selfAddress string) error

	// FetchFileList requests the list of available files from a remote peer.
	FetchFileList(peerAddress string) ([]file.FileMeta, error)

	// FetchFileMeta requests the metadata for a specific file from a remote peer.
	FetchFileMeta(peerAddress string, fileName string) (*file.FileMeta, error)

	// FetchChunk downloads a single chunk (by its hash) from a remote peer.
	// Returns the raw chunk data bytes.
	FetchChunk(peerAddress string, chunkHash string) ([]byte, error)
}
