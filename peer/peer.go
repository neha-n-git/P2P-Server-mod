// peer initialisation
package peer

import (
	"p2p/file"
	"sync"
)

type Peer struct {
	PeerID           string
	Port             int
	SharedFiles      map[string]file.FileMeta
	ChunkMap         map[string][]ChunkLocation
	ReceivedChunks   map[string]map[int][]byte // Temporary storage for reassembly: FileID -> Index -> Data
	ChunkDataStorage map[string][]byte         // Storage for owned chunks: Hash -> Data
	Mu               sync.Mutex                // For thread-safe updates
}

type ChunkLocation struct { // to store which peer has a certain chunk
	ChunkIndex int
	Peer       string
}

// initialisation
func NewPeer(peerID string, port int) *Peer {
	return &Peer{
		PeerID:           peerID,
		Port:             port,
		SharedFiles:      make(map[string]file.FileMeta),
		ChunkMap:         make(map[string][]ChunkLocation),
		ReceivedChunks:   make(map[string]map[int][]byte),
		ChunkDataStorage: make(map[string][]byte),
	}
}

func (p *Peer) AddFile(fileMsg file.FileMeta) {
	p.SharedFiles[fileMsg.FileName] = fileMsg
}
func (p *Peer) AddChunkLocation(fileName string, location ChunkLocation) {
	p.ChunkMap[fileName] = append(p.ChunkMap[fileName], location)
}
func (p *Peer) HasFile(fileName string) bool {
	_, exists := p.SharedFiles[fileName]
	return exists
}
