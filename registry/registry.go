package registry

import "sync"

// Central registry of peers
// In a real P2P network, this would be replaced by a discovery service or DHT.
// For this simulation, we use a thread-safe global list.

var (
	Peers []string
	Mu    sync.RWMutex
)

func AddPeer(address string) {
	Mu.Lock()
	defer Mu.Unlock()
	// simple check to avoid duplicates
	for _, p := range Peers {
		if p == address {
			return
		}
	}
	Peers = append(Peers, address)
}

func GetPeers() []string {
	Mu.RLock()
	defer Mu.RUnlock()
	// Return a copy to avoid race conditions
	list := make([]string, len(Peers))
	copy(list, Peers)
	return list
}
