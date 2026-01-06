package herald

import "context"

type PeerHook func(ctx context.Context, peerID string)

type Hook struct {
	OnPeerJoin  []PeerHook
	OnPeerLeave []PeerHook
}
