package registry

import (
	"maps"
	"sync"
	"time"

	"github.com/abolfazlalz/herald/internal/timeutil"
)

type PeerStatus int

const (
	PeerStatusConnected PeerStatus = iota
	PeerStatusConnecting
	PeerStatusDisconnected
)

type Peer struct {
	LastOnline time.Time
	Status     PeerStatus
	PublicKey  []byte
	RouteKey   string
	WaitList   []chan int
}

func NewPeer(id string, pubkey []byte, routeKey string, lastOnline time.Time, status PeerStatus) *Peer {
	return &Peer{
		LastOnline: lastOnline,
		Status:     status,
		PublicKey:  pubkey,
		RouteKey:   routeKey,
		WaitList:   make([]chan int, 0),
	}
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
	r.mu.RLock()
	defer r.mu.RUnlock()
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

func (r *PeerRegistry) Add(id string, pubkey []byte, routeKey string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.peers[id] = NewPeer(id, pubkey, routeKey, r.clock.Now(), PeerStatusConnecting)
}

func (r *PeerRegistry) Remove(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.peers, id)
}

func (r *PeerRegistry) Wait(id string) chan int {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.peers[id]; !ok {
		return nil
	}
	ch := make(chan int, 1)
	r.peers[id].WaitList = append(r.peers[id].WaitList, ch)
	return ch
}

func (r *PeerRegistry) ChangePeerStatus(id string, status PeerStatus) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.peers[id]; !ok {
		return
	}
	if status == PeerStatusConnected {
		for _, ch := range r.peers[id].WaitList {
			ch <- 1
		}
		r.peers[id].WaitList = make([]chan int, 0)
	}
	r.peers[id].Status = status
}
