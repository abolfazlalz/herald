package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/abolfazlalz/herald"
	"github.com/abolfazlalz/herald/transport"
)

const (
	reset  = "\033[0m"
	red    = "\033[31m"
	yellow = "\033[33m"
	blue   = "\033[34m"
	gray   = "\033[90m"
	cyan   = "\033[36m"
	green  = "\033[32m"
)

func PrintError(err error) {
	fmt.Printf("%sError: %s%s\n", red, err.Error(), reset)
	PrintColorable(red, "Error:", err.Error())
}

func Send(ctx context.Context, h *herald.Herald, peerID, msg string) {
	PrintColorable(gray, "Send message:", msg, "to", peerID)
	if err := h.SendToPeer(ctx, peerID, []byte(msg)); err != nil {
		PrintError(err)
	}
}

func PrintColorable(color, msg string, args ...string) {
	fmt.Printf("%s%s%s\n", color, strings.Join(append([]string{msg}, args...), " "), reset)
}

func main() {
	transport, err := transport.NewRabbitMQ("amqp://guest:guest@localhost:5672/", "queue_name")
	if err != nil {
		panic(err)
	}
	// h, err := herald.New(transport, &herald.Option{Logger: herald.NewLogger(&slog.HandlerOptions{Level: slog.LevelError})})
	h, err := herald.New(transport, nil)
	// h, err := herald.New(transport, &herald.Option{Logger: herald.NewLogs(slog.New(slog.NewJSONHandler(os.Stdout, nil)))})
	if err != nil {
		log.Fatal(err)
	}

	h.OnPeerJoin(func(ctx context.Context, peerID string) {
		PrintColorable(green, peerID, "joined")
		Send(ctx, h, peerID, "Hello, peer!")
	})

	ctx := context.Background()

	sub := h.Subscribe(ctx, herald.MessageTypeMessage, 0)
	go func() {
		for msg := range sub.C {
			PrintColorable(blue, "Message received:", string(msg.Payload))
			if string(msg.Payload) == "Hello, peer!" {
				Send(ctx, h, msg.From, "nice to meet you, peer!")
			}
		}
	}()

	if err := h.Start(ctx); err != nil {
		panic(err)
	}
}
