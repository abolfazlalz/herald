package registry

import (
	"testing"
	"time"
)

func TestPeerRegistryAddGetAndRemove(t *testing.T) {
	registry := NewPeerRegistry()
	pubKey := []byte("public")

	registry.Add("peer-1", pubKey, "route-1")
	if !registry.Exists("peer-1") {
		t.Fatal("registry does not contain added peer")
	}

	peer, ok := registry.PeerByID("peer-1")
	if !ok {
		t.Fatal("PeerByID did not find added peer")
	}
	if peer.RouteKey != "route-1" {
		t.Fatalf("RouteKey = %q, want route-1", peer.RouteKey)
	}
	if peer.Status != PeerStatusConnecting {
		t.Fatalf("Status = %v, want PeerStatusConnecting", peer.Status)
	}

	registry.Remove("peer-1")
	if registry.Exists("peer-1") {
		t.Fatal("registry still contains removed peer")
	}
}

func TestPeerRegistryPeersReturnsCopy(t *testing.T) {
	registry := NewPeerRegistry()
	registry.Add("peer-1", []byte("public"), "route-1")

	peers := registry.Peers()
	delete(peers, "peer-1")

	if !registry.Exists("peer-1") {
		t.Fatal("mutating Peers result changed registry state")
	}
}

func TestPeerRegistryUpdateLastOnline(t *testing.T) {
	registry := NewPeerRegistry()
	registry.Add("peer-1", []byte("public"), "route-1")

	peer, ok := registry.PeerByID("peer-1")
	if !ok {
		t.Fatal("PeerByID did not find added peer")
	}
	peer.LastOnline = time.Now().Add(-time.Hour)
	oldLastOnline := peer.LastOnline

	registry.UpdateLastOnline("peer-1")
	if !peer.LastOnline.After(oldLastOnline) {
		t.Fatalf("LastOnline = %v, want after %v", peer.LastOnline, oldLastOnline)
	}
}

func TestPeerRegistryWaitIsReleasedWhenPeerConnects(t *testing.T) {
	registry := NewPeerRegistry()
	registry.Add("peer-1", []byte("public"), "route-1")

	wait := registry.Wait("peer-1")
	if wait == nil {
		t.Fatal("Wait returned nil for existing peer")
	}

	registry.ChangePeerStatus("peer-1", PeerStatusConnected)

	select {
	case <-wait:
	case <-time.After(time.Second):
		t.Fatal("waiter was not released")
	}

	peer, ok := registry.PeerByID("peer-1")
	if !ok {
		t.Fatal("PeerByID did not find added peer")
	}
	if peer.Status != PeerStatusConnected {
		t.Fatalf("Status = %v, want PeerStatusConnected", peer.Status)
	}
	if len(peer.WaitList) != 0 {
		t.Fatalf("WaitList length = %d, want 0", len(peer.WaitList))
	}
}

func TestPeerRegistryWaitReturnsNilForUnknownPeer(t *testing.T) {
	if ch := NewPeerRegistry().Wait("missing"); ch != nil {
		t.Fatal("Wait returned non-nil channel for missing peer")
	}
}
