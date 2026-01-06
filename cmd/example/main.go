package main

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"fmt"
	"log/slog"
	mathRand "math/rand"
	"os"

	"github.com/abolfazlalz/herald"
)

type Data struct {
	Key     string `json:"key"`
	Message string `json:"message"`
	UUID    string `json:"uuid"`
}

func main() {
	transport, err := herald.NewRabbitMQ("amqp://guest:guest@localhost:5672/", "events")
	if err != nil {
		slog.Error("error during connect to RabbitMQ", "error", err)
	}
	defer transport.Close()

	_, pri, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		slog.Error("error during generate security", "error", err)
	}

	ctx := context.Background()

	h := herald.New(transport, pri)
	fmt.Println("Cluster ID:", h.ID())

	go func() {
		if err := h.Start(ctx); err != nil {
			slog.Error("error during listen to herald", "error", err)
		}
	}()

	go func() {
		sub := h.Subscribe(ctx, herald.MessageTypeMessage, 0)
		for msg := range sub.C {
			fmt.Println("Received message:", msg.Payload["message"])
		}
	}()

	fmt.Println("Type message and press Enter (type `exit` to quit):")

	reader := bufio.NewReader(os.Stdin)

	for {
		text, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("read error", "error", err)
			continue
		}

		text = text[:len(text)-1] // remove \n

		if text == "exit" {
			break
		}
		if text == "random" {
			peerID := h.Peers()[mathRand.Intn(len(h.Peers()))]
			h.SendToPeer(ctx, peerID, map[string]any{
				"key":     "random",
				"message": "random message",
			})
			continue
		}

		h.SendMessage(ctx, text)
	}
}
