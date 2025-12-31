package herald

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/abolfazlalz/herald/internal/handshake"
	"github.com/abolfazlalz/herald/internal/heartbeat"
	"github.com/abolfazlalz/herald/internal/message"
	"github.com/abolfazlalz/herald/internal/registry"
	"github.com/abolfazlalz/herald/internal/security"
	"github.com/google/uuid"
)

const (
	// PeerTimeout
	PeerTimeout = 5 * time.Second
)

type Herald struct {
	transport  Transport
	id         string
	privateKey []byte

	kp       *security.KeyPair
	signer   security.Signer
	verifier security.Verifier
	registry *registry.PeerRegistry

	handlers map[message.EventType]handlerFunc
}

func New(transport Transport, privateKey []byte) *Herald {
	id := uuid.New().String()

	h := &Herald{
		transport:  transport,
		id:         id,
		privateKey: privateKey,
		registry:   registry.NewPeerRegistry(),
	}
	h.handlers = map[message.EventType]handlerFunc{
		message.EventAnnounce:  handleAnnounce(),
		message.EventHeartbeat: handleHeartbeat(),
	}
	return h
}

func (h *Herald) startHandshake(ctx context.Context) error {
	msg, err := handshake.InitiateHandshake(h.id, h.kp, h.signer)
	if err != nil {
		return err
	}

	return h.publish(ctx, msg)
}

func (h *Herald) startCheckHeartbeats(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// shutdown graceful
			log.Println("healthcheck stopped:", ctx.Err())
			return

		case <-ticker.C:
			for id, peer := range h.registry.Peers() {
				if time.Since(peer.LastOnline) > PeerTimeout {
					fmt.Printf("peer %s is dead\n", id)
					h.registry.Remove(id)
				}
			}
		}
	}
}

func (h *Herald) startHeartbeatPublisher(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("heartbeat publisher stopped:", ctx.Err())
			return

		case <-ticker.C:
			env, err := heartbeat.InitiateHeartbeat(h.id, h.kp, h.signer)
			if err != nil {
				log.Printf("heartbeat initiation failed: %v", err)
				continue
			}

			if err := h.publish(ctx, env); err != nil {
				log.Printf("publish heartbeat failed: %v", err)
			}
		}
	}
}

func (h *Herald) publish(ctx context.Context, data any) error {
	jsonB, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return h.transport.Publish(ctx, jsonB)
}

func (h *Herald) subscribe(ctx context.Context, dataCh <-chan []byte) error {
	for data := range dataCh {
		if err := h.executeMessage(ctx, data); err != nil {
			log.Printf("error during execute message: %v", err)
		}
	}
	return nil
}

func (h *Herald) executeMessage(ctx context.Context, data []byte) error {
	var env message.Envelope
	if err := json.Unmarshal(data, &env); err != nil {
		return fmt.Errorf("error in unmarshal message: %v", err)
	}

	if h.id == env.SenderID {
		return nil
	}

	middlewares := []Middleware{
		VerifySignature(),
		UpdateLastOnline(),
	}

	context := NewMessageContext(ctx)

	for _, middleware := range middlewares {
		if context.IsAborted() {
			return nil
		}
		if err := middleware(context, h, &env); err != nil {
			return fmt.Errorf("error in middleware: %v", err)
		}
	}

	handler, ok := h.handlers[env.Type]
	if !ok {
		return errors.New("invalid event type")
	}
	if err := handler(context, h, &env); err != nil {
		return fmt.Errorf("error during handle envelope action: %v", err)
	}
	return nil
}

func (h *Herald) Start(ctx context.Context) error {
	defer func() {
		h.transport.Close()
	}()

	var err error
	h.kp, err = security.LoadFromBytes(h.privateKey)
	if err != nil {
		return err
	}
	h.verifier = security.NewVerifier()
	h.signer, err = security.NewSigner(h.kp)
	if err != nil {
		return err
	}

	dataCh, err := h.transport.Subscribe(ctx)
	if err != nil {
		return err
	}
	go h.subscribe(ctx, dataCh)

	if err := h.startHandshake(ctx); err != nil {
		return err
	}

	// start heartbeat publisher
	go h.startHeartbeatPublisher(ctx)
	// check healthcheck
	go h.startCheckHeartbeats(ctx)

	<-ctx.Done()
	return ctx.Err()
}

func (h *Herald) ID() string {
	return h.id
}
