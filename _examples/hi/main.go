package main

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/abolfazlalz/herald"
	"github.com/abolfazlalz/herald/transport"
)

func main() {
	transport, err := transport.NewRabbitMQ("amqp://guest:guest@localhost:5672/", "queue_name")
	if err != nil {
		panic(err)
	}
	h, err := herald.New(transport)
	if err != nil {
		panic(err)
	}
	slog.Info("Service started", "ID", h.ID())

	h.OnPeerJoin(func(ctx context.Context, peerID string) {
		h.SendToPeer(ctx, peerID, []byte("Hello, peer!"))
	})

	ctx := context.Background()

	sub := h.Subscribe(ctx, herald.MessageTypeMessage, 0)
	go func() {
		for {
			msg := <-sub.C
			fmt.Println(string(msg.Payload))
			if string(msg.Payload) != "Hello, peer!" {
				continue
			}
			h.SendToPeer(ctx, msg.From, []byte("nice to meet you, peer!"))
		}
	}()

	if err := h.Start(ctx); err != nil {
		panic(err)
	}
}
