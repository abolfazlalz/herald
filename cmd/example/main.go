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
	"time"

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
		slog.Error("❌ RabbitMQ connection failed", "error", err)
	}
	defer transport.Close()

	_, pri, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		slog.Error("🔐 key generation failed", "error", err)
	}

	ctx := context.Background()

	h := herald.New(transport, pri)
	fmt.Println("🆔 Cluster ID:", h.ID())

	go func() {
		if err := h.Start(ctx); err != nil {
			slog.Error("📡 herald listener error", "error", err)
		}
	}()

	go func() {
		sub := h.Subscribe(ctx, herald.MessageTypeMessage, 10)
		for msg := range sub.C {
			fmt.Println("📨 Incoming:", msg.Payload["message"])
		}
	}()

	fmt.Println("⌨️ Type a message and press Enter (`exit` 🚪 to quit):")

	reader := bufio.NewReader(os.Stdin)

	for {
		text, err := reader.ReadString('\n')
		if err != nil {
			slog.Error("⚠️ input read error", "error", err)
			continue
		}

		text = text[:len(text)-1]

		if text == "exit" {
			fmt.Println("👋 Shutting down…")
			break
		}

		if text == "random" {
			peerID := h.Peers()[mathRand.Intn(len(h.Peers()))]
			err := h.SendAndWait(ctx, peerID, map[string]any{
				"key":     "random",
				"message": "🎲 random message",
			}, time.Second*1)

			if err != nil {
				fmt.Println("⚠️ Message not sent ❌")
			} else {
				fmt.Println("🎯 Random message sent ✅")
			}
			continue
		}

		h.SendMessage(ctx, text)
		fmt.Println("📤 Message sent ✅")
	}
}
