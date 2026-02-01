package herald

import (
	"fmt"

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
		if env.Type == message.EventAnnounce || env.Type == message.EventAck {
			return nil
		}
		peer, ok := h.registry.PeerByID(env.SenderID)
		if !ok {
			h.logger.Debug(ctx, "verifySignature unknown peer", "correlation_id", env.CorrelationID, "sender", env.SenderID)
			return ErrUnknownPeer
		}

		if err := env.Verify(h.verifier, peer.PublicKey); err != nil {
			return fmt.Errorf("signature verification failed: %v", err)
		}
		return nil
	}
}

func CheckMessageAccess() Middleware {
	return func(ctx *MessageContext, h *Herald, env *message.Envelope) error {
		if env.ReceiverID != "" && env.ReceiverID != h.ID() && env.Type != message.EventAck {
			h.logger.Debug(ctx, "sender not allowed", "correlation_id", env.CorrelationID, "sender", env.SenderID)
			ctx.Abort()
			return nil
		}
		return nil
	}
}
