// peer initialisation 
package peer

type Peer struct{
	PeerID string
	Port int
	SharedFiles map[string]FileMeta
	ChunkMap map[string][]ChunkLocation
}
type ChunkLocation struct{ // to store which peer has a certain chunk
	ChunkIndex int
	Peer string
}

//initialisation
func NewPeer(peerID string, port int) *Peer {
	return &Peer{
		PeerID:      peerID,
		Port:        port,
		SharedFiles: make(map[string]FileMeta),
		ChunkMap:    make(map[string][]ChunkLocation),
	}
}

func (p *Peer) AddFile(file FileMeta) {
	p.SharedFiles[file.FileName] = file
}
func (p *Peer) AddChunkLocation(fileName string, location ChunkLocation) {
	p.ChunkMap[fileName] = append(p.ChunkMap[fileName], location)
}
func (p *Peer) HasFile(fileName string) bool {
	_, exists := p.SharedFiles[fileName]
	return exists
}
