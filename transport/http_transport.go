package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"p2p/file"
	"time"
)

// HTTPTransport implements the Transport interface using HTTP and JSON
// for serialization. This allows the communication mechanism to be replaced
// in the future without changing the core peer logic.
type HTTPTransport struct {
	Client          *http.Client
	NetworkPassword string // shared password for P2P network authentication
}

// NewHTTPTransport creates a new HTTPTransport with sensible defaults.
func NewHTTPTransport(networkPassword string) *HTTPTransport {
	return &HTTPTransport{
		Client: &http.Client{
			Timeout: 30 * time.Second,
		},
		NetworkPassword: networkPassword,
	}
}

// RegisterWithPeer sends a JSON-encoded registration request to a remote peer.
// Includes the network password for authentication.
func (t *HTTPTransport) RegisterWithPeer(peerAddress string, selfAddress string) error {
	payload := map[string]string{
		"address":          selfAddress,
		"network_password": t.NetworkPassword,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal registration payload: %w", err)
	}

	reqURL := fmt.Sprintf("http://%s/api/register", peerAddress)
	resp, err := t.Client.Post(reqURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send registration to %s: %w", peerAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("registration failed at %s (status %d): %s", peerAddress, resp.StatusCode, string(respBody))
	}
	return nil
}

// FetchPeerInfo retrieves the identity/name of a remote peer.
func (t *HTTPTransport) FetchPeerInfo(peerAddress string) (string, error) {
	reqURL := fmt.Sprintf("http://%s/api/info", peerAddress)
	resp, err := t.Client.Get(reqURL)
	if err != nil {
		return "", fmt.Errorf("failed to fetch peer info from %s: %w", peerAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("fetch peer info failed at %s (status %d): %s", peerAddress, resp.StatusCode, string(respBody))
	}

	var info struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", fmt.Errorf("failed to decode peer info from %s: %w", peerAddress, err)
	}
	return info.Name, nil
}

// FetchFileList retrieves the list of shared files from a remote peer.
func (t *HTTPTransport) FetchFileList(peerAddress string) ([]file.FileMeta, error) {
	reqURL := fmt.Sprintf("http://%s/api/files", peerAddress)
	resp, err := t.Client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file list from %s: %w", peerAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch file list failed at %s (status %d): %s", peerAddress, resp.StatusCode, string(respBody))
	}

	var files []file.FileMeta
	if err := json.NewDecoder(resp.Body).Decode(&files); err != nil {
		return nil, fmt.Errorf("failed to decode file list from %s: %w", peerAddress, err)
	}
	return files, nil
}

// FetchFileMeta retrieves metadata for a specific file from a remote peer.
func (t *HTTPTransport) FetchFileMeta(peerAddress string, fileName string) (*file.FileMeta, error) {
	reqURL := fmt.Sprintf("http://%s/api/filemeta?name=%s", peerAddress, url.QueryEscape(fileName))
	resp, err := t.Client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch file meta from %s: %w", peerAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch file meta failed at %s (status %d): %s", peerAddress, resp.StatusCode, string(respBody))
	}

	var meta file.FileMeta
	if err := json.NewDecoder(resp.Body).Decode(&meta); err != nil {
		return nil, fmt.Errorf("failed to decode file meta from %s: %w", peerAddress, err)
	}
	return &meta, nil
}

// FetchChunk downloads raw chunk data identified by its hash from a remote peer.
func (t *HTTPTransport) FetchChunk(peerAddress string, chunkHash string) ([]byte, error) {
	reqURL := fmt.Sprintf("http://%s/api/chunk?hash=%s", peerAddress, url.QueryEscape(chunkHash))
	resp, err := t.Client.Get(reqURL)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch chunk %s from %s: %w", chunkHash, peerAddress, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("fetch chunk failed at %s (status %d): %s", peerAddress, resp.StatusCode, string(respBody))
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read chunk data from %s: %w", peerAddress, err)
	}
	return data, nil
}
