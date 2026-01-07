package main

import (
	"context"
	"fmt"

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

	h.OnPeerJoin(func(ctx context.Context, peerID string) {
		h.SendToPeer(ctx, peerID, []byte("Hello, peer!"))
	})

	ctx := context.Background()

	sub := h.Subscribe(ctx, herald.MessageTypeMessage, 0)
	go func() {
		for {
			msg := <-sub.C
			fmt.Println(string(msg.Payload))
			// h.SendToPeer(ctx, msg.SenderID, []byte("Hello, peer!"), time.Second*2)
		}
	}()

	if err := h.Start(ctx); err != nil {
		panic(err)
	}
}
