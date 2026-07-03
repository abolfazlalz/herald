package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	mathRand "math/rand"
	"os"
	"strings"

	"github.com/abolfazlalz/herald"
	"github.com/abolfazlalz/herald/transport"
)

type Payload struct {
	Message string `json:"message"`
}

func main() {
	transport, err := transport.NewRabbitMQ("amqp://guest:guest@localhost:5672/", "events")
	if err != nil {
		slog.Error("❌ RabbitMQ connection failed", "error", err)
	}
	defer transport.Close()

	ctx := context.Background()
	h, err := herald.New(transport, nil)
	if err != nil {
		slog.Error("❌ Herald initialization failed", "error", err)
		return
	}

	fmt.Println("🆔 Cluster ID:", h.ID())

	h.OnPeerJoin(func(ctx context.Context, peerID string) {
		fmt.Println("🤝 Peer joined the cluster:", peerID)
	})

	h.OnPeerLeave(func(ctx context.Context, peerID string) {
		fmt.Println("👋 Peer left the cluster:", peerID)
	})

	go func() {
		if err := h.Start(ctx); err != nil {
			slog.Error("📡 Herald listener error", "error", err)
		}
	}()

	go func() {
		h.Subscribe(ctx, herald.MessageTypeMessage, func(mc *herald.MessageContext, msg *herald.Message) {
			var payload Payload
			if err := json.Unmarshal(msg.Payload, &payload); err != nil {
				slog.Error("❌ Failed to unmarshal payload", "error", err)
				return
			}
			fmt.Println("📩 Incoming message:", payload.Message)
		})
	}()

	reader := bufio.NewReader(os.Stdin)
	fmt.Println("⌨️ Type your message and press Enter (`exit` 🚪 to quit)")

	for {
		fmt.Println("\n🔹 Choose sending mode: [1] Broadcast to all, [2] Send to specific peer, [3] Random peer")
		fmt.Print("Mode: ")
		modeInput, _ := reader.ReadString('\n')
		modeInput = strings.TrimSpace(modeInput)

		if modeInput == "exit" {
			fmt.Println("👋 Shutting down…")
			break
		}

		switch modeInput {
		case "1":
			fmt.Print("📨 Enter your message: ")
			msg, _ := reader.ReadString('\n')
			msg = strings.TrimSpace(msg)
			if msg == "exit" {
				fmt.Println("👋 Shutting down…")
				return
			}
			payload, err := json.Marshal(Payload{Message: msg})
			if err != nil {
				slog.Error("❌ Failed to marshal payload", "error", err)
				continue
			}

			h.Broadcast(ctx, payload)
			fmt.Println("📤 Your message has been sent to all ✅")

		case "2":
			peers := h.Peers()
			if len(peers) == 0 {
				fmt.Println("⚠️ No peers connected ❌")
				continue
			}

			fmt.Println("🔹 Connected peers:")
			for i, p := range peers {
				fmt.Printf("[%d] %s\n", i, p)
			}
			fmt.Print("Select peer index: ")
			peerInput, _ := reader.ReadString('\n')
			peerInput = strings.TrimSpace(peerInput)
			index := 0
			fmt.Sscanf(peerInput, "%d", &index)
			if index < 0 || index >= len(peers) {
				fmt.Println("⚠️ Invalid peer index ❌")
				continue
			}

			fmt.Print("📨 Enter your message: ")
			msg, _ := reader.ReadString('\n')
			msg = strings.TrimSpace(msg)
			payload, err := json.Marshal(Payload{Message: msg})
			if err != nil {
				slog.Error("❌ Failed to marshal payload", "error", err)
				continue
			}
			err = h.SendToPeer(ctx, peers[index], payload)

			if err != nil {
				fmt.Println("⚠️ Failed to send message ❌")
			} else {
				fmt.Println("🎯 Message sent to", peers[index], "✅")
			}

		case "3":
			peers := h.Peers()
			if len(peers) == 0 {
				fmt.Println("⚠️ No peers connected ❌")
				continue
			}
			peerID := peers[mathRand.Intn(len(peers))]
			payload, err := json.Marshal(Payload{Message: "🎲 Random message sent!"})
			if err != nil {
				slog.Error("❌ Failed to marshal payload", "error", err)
				continue
			}

			err = h.SendToPeer(ctx, peerID, payload)

			if err != nil {
				fmt.Println("⚠️ Failed to send message ❌")
			} else {
				fmt.Println("🎯 Random message sent to", peerID, "✅")
			}

		default:
			fmt.Println("⚠️ Invalid mode, choose 1, 2 or 3 ❌")
		}
	}
}
