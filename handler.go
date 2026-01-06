package herald

import (
	"context"

	"github.com/abolfazlalz/herald/internal/handshake"
	"github.com/abolfazlalz/herald/internal/message"
)

type Handler interface {
	Handle(context.Context, *Herald, *message.Envelope) error
}

type handlerFunc func(context.Context, *Herald, *message.Envelope) error

func handleAnnounce() handlerFunc {
	return func(ctx context.Context, h *Herald, env *message.Envelope) error {
		if h.registry.Exists(env.SenderID) {
			return nil
		}

		if err := handshake.RespondHandshake(env, h.verifier, h.registry); err != nil {
			return err
		}

		h.callPeerJoinHook(ctx, env.SenderID)

		return h.startHandshake(ctx)
	}
}

func handleHeartbeat() handlerFunc {
	return func(ctx context.Context, h *Herald, env *message.Envelope) error {
		return nil
	}
}

func handleMessage(msgCh chan Message) handlerFunc {
	return func(ctx context.Context, h *Herald, env *message.Envelope) error {
		msgCh <- Message{
			ID:      env.ID,
			From:    env.SenderID,
			To:      env.ReceiverID,
			Type:    MessageTypeMessage,
			Payload: env.Payload,
		}
		return nil
	}
}

func handleAck() handlerFunc {
	return func(ctx context.Context, h *Herald, env *message.Envelope) error {
		id, _ := env.Payload["ack_for"].(string)

		h.mu.Lock()
		if p, ok := h.pending[id]; ok {
			close(p.ch)
			delete(h.pending, id)
		}
		h.mu.Unlock()
		return nil
	}
}

func handleOffline() handlerFunc {
	return func(ctx context.Context, h *Herald, e *message.Envelope) error {
		h.registry.Remove(e.SenderID)
		return nil
	}
}
