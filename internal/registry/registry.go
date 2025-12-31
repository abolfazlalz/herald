package registry

import (
	"maps"
	"sync"
	"time"

	"github.com/abolfazlalz/herald/internal/timeutil"
)

type Peer struct {
	LastOnline time.Time
	PublicKey  []byte
}

type PeerRegistry struct {
	peers map[string]*Peer
	clock timeutil.Clock
	mu    sync.RWMutex
}

func NewPeerRegistry() *PeerRegistry {
	return &PeerRegistry{
		peers: make(map[string]*Peer),
		clock: timeutil.NewClock(),
	}
}

func (r *PeerRegistry) Exists(id string) bool {
	_, ok := r.peers[id]
	return ok
}

func (r *PeerRegistry) PeerByID(id string) (*Peer, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	peer, exists := r.peers[id]
	return peer, exists
}

func (r *PeerRegistry) Peers() map[string]*Peer {
	r.mu.RLock()
	defer r.mu.RUnlock()

	cp := make(map[string]*Peer, len(r.peers))
	maps.Copy(cp, r.peers)
	return cp
}

func (r *PeerRegistry) UpdateLastOnline(ID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.peers[ID]
	if !ok {
		return
	}
	r.peers[ID].LastOnline = r.clock.Now()
}

func (r *PeerRegistry) Add(id string, pubkey []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[id] = &Peer{
		LastOnline: r.clock.Now(),
		PublicKey:  pubkey,
	}
}

func (r *PeerRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.peers, id)
}
