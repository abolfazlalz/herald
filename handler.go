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
			From:    env.SenderID,
			To:      h.ID(),
			Type:    MessageTypeMessage,
			Payload: env.Payload,
		}
		return nil
	}
}
