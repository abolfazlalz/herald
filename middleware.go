package herald

import (
	"errors"
	"log"

	"github.com/abolfazlalz/herald/internal/message"
)

type Middleware func(ctx *MessageContext, herald *Herald, env *message.Envelope) error

func UpdateLastOnline() Middleware {
	return func(ctx *MessageContext, h *Herald, e *message.Envelope) error {
		h.registry.UpdateLastOnline(e.SenderID)
		return nil
	}
}

func VerifySignature() Middleware {
	return func(ctx *MessageContext, h *Herald, env *message.Envelope) error {
		if env.Type != message.EventAnnounce {
			peer, ok := h.registry.PeerByID(env.SenderID)
			if !ok {
				ctx.Abort()
				log.Printf("unknown peer %s", env.SenderID)
				return nil
			}
			if err := env.Verify(h.verifier, peer.PublicKey); err != nil {
				return errors.New("signature verification failed")
			}
		}
		return nil
	}
}
