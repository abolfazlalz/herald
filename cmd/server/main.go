package main

import (
	"encoding/base64"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/abolfazlalz/herald/internal/handshake"
	"github.com/abolfazlalz/herald/internal/message"
	"github.com/abolfazlalz/herald/internal/rabbitmq"
	reg "github.com/abolfazlalz/herald/internal/registry"
	"github.com/abolfazlalz/herald/internal/security"
	"github.com/abolfazlalz/herald/internal/transport"
)

func main() {
	// -----------------------------
	// Identity
	// -----------------------------
	serviceID := "auth-02"

	kp, err := security.Generate()
	if err != nil {
		log.Fatal(err)
	}

	signer, err := security.NewSigner(kp)
	if err != nil {
		log.Fatal(err)
	}

	verifier := security.NewVerifier()

	// -----------------------------
	// Peer registry (after handshake)
	// -----------------------------
	registry := handshake.NewPeerRegistry()
	registry.Peers()[serviceID] = kp.Public // self-trust (demo)

	// -----------------------------
	// Transport (Adapter)
	// -----------------------------
	var t transport.Transport

	rmq, err := rabbitmq.New(
		"amqp://guest:guest@localhost:5672/",
		serviceID,
	)
	if err != nil {
		log.Fatal("rabbitmq connect failed:", err)
	}
	defer rmq.Close()

	t = rmq

	// -----------------------------
	// lastSeen + mutex
	// -----------------------------
	lastSeen := make(map[string]int64)
	var mu sync.Mutex

	announce := message.NewEnvelope(
		message.EventAnnounce,
		serviceID,
		map[string]any{
			"uuid":         "33",
			"public_key":   kp.Public,
			"capabilities": []string{"auth", "jwt"},
			"version":      "1.0.0",
		},
	)

	_ = announce.Sign(signer)
	discoveryQueue := "discovery_exchange"
	_ = t.Publish(discoveryQueue, "", announce)

	log.Println("📣 ANNOUNCE sent")

	// -----------------------------
	// Subscribe (consumer)
	// -----------------------------
	err = t.Subscribe("heartbeat_queue", func(env *message.Envelope) {
		pubKey, ok := registry.Peers()[env.SenderID]
		if !ok {
			log.Println("❌ unknown sender:", env.SenderID)
			return
		}

		if err := env.Verify(verifier, pubKey); err != nil {
			log.Println("❌ invalid signature:", err)
			return
		}

		mu.Lock()
		lastSeen[env.SenderID] = env.Timestamp
		mu.Unlock()

		log.Printf("✅ heartbeat verified from %s\n", env.SenderID)
	})

	if err != nil {
		log.Fatal(err)
	}

	regi := reg.New()

	t.Subscribe(discoveryQueue, func(env *message.Envelope) {
		if env.Type != message.EventAnnounce {
			return
		}
		if env.SenderID == serviceID {
			return
		}

		payload := env.Payload.(map[string]any)

		pubKey, _ := base64.StdEncoding.DecodeString(payload["public_key"].(string))
		if err := env.Verify(verifier, pubKey); err != nil {
			log.Println("❌ invalid announce signature")
			return
		}

		info := &reg.ServiceInfo{
			ID:           env.SenderID,
			UUID:         payload["uuid"].(string),
			PublicKey:    pubKey,
			Capabilities: strings.Split(string(payload["capabilities"].(string)), ","),
			Version:      payload["version"].(string),
		}

		isNew := regi.AddOrUpdate(info)
		if isNew {
			log.Printf("🆕 SERVICE DISCOVERED: %s (%v)\n",
				info.ID, info.Capabilities)
		}
	})

	// -----------------------------
	// Heartbeat publisher loop
	// -----------------------------
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			env := message.NewEnvelope(
				message.EventHeartbeat,
				serviceID,
				map[string]any{
					"status": "alive",
				},
			)

			if err := env.Sign(signer); err != nil {
				log.Println("sign failed:", err)
				continue
			}

			if err := t.Publish("heartbeat_exchange", "", env); err != nil {
				log.Println("publish failed:", err)
			} else {
				log.Println("📤 heartbeat sent")
			}
		}
	}()

	// -----------------------------
	// Offline detection loop
	// -----------------------------
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			now := time.Now().Unix()

			mu.Lock()
			for id, ts := range lastSeen {
				if now-ts > 6 {
					log.Printf("⚠️ service %s OFFLINE\n", id)
				} else {
					log.Printf("🟢 service %s ONLINE\n", id)
				}
			}
			mu.Unlock()
		}
	}()

	// -----------------------------
	// Block forever
	// -----------------------------
	select {}
}
