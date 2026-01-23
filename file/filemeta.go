// file and chunk meta
package file
type ChunkMeta struct {
	Index int
	Hash  [32]byte 
}

type FileMeta struct {
	FileName    string
	FileSize    int
	ChunkSize   int
	TotalChunks int
	Chunks      []ChunkMeta //slice
}