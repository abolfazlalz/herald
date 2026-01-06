package herald

import (
	"context"

	"github.com/abolfazlalz/herald/internal/acknowledge"
	"github.com/abolfazlalz/herald/internal/handshake"
	"github.com/abolfazlalz/herald/internal/message"
)

// Handler defines the interface that any message handler must implement.
type Handler interface {
	Handle(context.Context, *Herald, *message.Envelope) error
}

// handlerFunc is a helper type that implements Handler interface for functions.
type handlerFunc func(context.Context, *Herald, *message.Envelope) error

// handleAnnounce returns a handler function that processes EventAnnounce messages.
// It adds the peer to the registry if it doesn't exist, responds to the handshake,
// triggers OnPeerJoin hooks, and initiates the local handshake.
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

// handleHeartbeat returns a handler function for EventHeartbeat messages.
// Currently, it does nothing but can be extended to track peer liveness.
func handleHeartbeat() handlerFunc {
	return func(ctx context.Context, h *Herald, env *message.Envelope) error {
		return nil
	}
}

// handleMessage returns a handler function for EventMessage messages.
// It pushes the incoming message onto the provided channel for processing.
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

// handleAck returns a handler function for EventAck messages.
// It closes the pending acknowledgment channel and removes it from the pending map.
func handleAck() handlerFunc {
	return func(ctx context.Context, h *Herald, env *message.Envelope) error {
		payload, err := acknowledge.FromPayload(env.Payload)
		if err != nil {
			return err
		}

		h.mu.Lock()
		if p, ok := h.pending[payload.AckTo]; ok {
			close(p.ch)
			delete(h.pending, payload.AckTo)
		}
		h.mu.Unlock()
		return nil
	}
}

// handleOffline returns a handler function for EventOffline messages.
// It removes the peer from the registry when they go offline.
func handleOffline() handlerFunc {
	return func(ctx context.Context, h *Herald, e *message.Envelope) error {
		h.registry.Remove(e.SenderID)
		return nil
	}
}
