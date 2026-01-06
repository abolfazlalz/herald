package herald

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"sync"
	"time"

	"github.com/abolfazlalz/herald/internal/handshake"
	"github.com/abolfazlalz/herald/internal/heartbeat"
	"github.com/abolfazlalz/herald/internal/message"
	"github.com/abolfazlalz/herald/internal/registry"
	"github.com/abolfazlalz/herald/internal/security"
	"github.com/abolfazlalz/herald/transport"
	"github.com/google/uuid"
)

const (
	// PeerTimeout
	PeerTimeout = 5 * time.Second
)

type pendingAck struct {
	ch chan struct{}
}

type Herald struct {
	transport  transport.Transport
	id         string
	privateKey []byte

	kp       *security.KeyPair
	signer   security.Signer
	verifier security.Verifier
	registry *registry.PeerRegistry

	subs            map[MessageType][]chan Message
	receiveMsgCh    chan Message
	sendMessageChan chan Message
	mu              sync.RWMutex

	pending map[string]*pendingAck

	handlers map[message.EventType]handlerFunc

	hooks Hook
}

func New(transport transport.Transport, privateKey []byte) *Herald {
	id := uuid.New().String()

	h := &Herald{
		transport:       transport,
		id:              id,
		privateKey:      privateKey,
		registry:        registry.NewPeerRegistry(),
		receiveMsgCh:    make(chan Message),
		subs:            make(map[MessageType][]chan Message),
		sendMessageChan: make(chan Message),
		pending:         make(map[string]*pendingAck),
	}
	h.handlers = map[message.EventType]handlerFunc{
		message.EventAnnounce:  handleAnnounce(),
		message.EventHeartbeat: handleHeartbeat(),
		message.EventMessage:   handleMessage(h.receiveMsgCh),
		message.EventAck:       handleAck(),
		message.EventOffline:   handleOffline(),
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
					h.registry.Remove(id)
					h.callPeerLeaveHook(ctx, id)
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
			env, err := heartbeat.InitiateHeartbeat(h.id, h.kp)
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

func (h *Herald) sendMessage(ctx context.Context, msg Message) (*message.Envelope, error) {
	env := message.NewEnvelope(
		msg.Type.GetType(),
		h.ID(),
		msg.To,
		msg.Payload,
	)

	ackCh := make(chan struct{})
	h.mu.Lock()
	h.pending[env.ID] = &pendingAck{ch: ackCh}
	h.mu.Unlock()

	return env, h.publish(ctx, env)
}

func (h *Herald) handleReceiveMessages(ctx context.Context) {
	for {
		select {
		case msg := <-h.receiveMsgCh:
			h.notify(msg.Type, msg)
		case <-ctx.Done():
			log.Println("message handler stopped:", ctx.Err())
			return
		}
	}
}

func (h *Herald) handleSendMessages(ctx context.Context) {
	for {
		select {
		case msg := <-h.sendMessageChan:
			h.sendMessage(ctx, msg)
		case <-ctx.Done():
			return
		}
	}
}

func (h *Herald) publish(ctx context.Context, env *message.Envelope) error {
	if err := env.Sign(h.signer); err != nil {
		return fmt.Errorf("envelope sign failed: %v", err)
	}

	data, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("error in marshal message: %v", err)
	}

	if env.ReceiverID != "" {
		err = h.transport.PublishDirect(ctx, env.ReceiverID, data)
	} else {
		err = h.transport.PublishBroadcast(ctx, data)
	}
	if err != nil {
		return err
	}

	ack := make(chan struct{})
	h.mu.Lock()
	h.pending[env.ID] = &pendingAck{ch: ack}
	h.mu.Unlock()
	return nil
}

func (h *Herald) subscribe(ctx context.Context, dataCh <-chan []byte) error {
	for data := range dataCh {
		if err := h.executeMessage(ctx, data); err != nil {
			log.Printf("error during execute message: %v", err)
		}
	}
	return nil
}

func (h *Herald) handleAck(_ context.Context, env message.Envelope) error {
	if env.Type != message.EventMessage {
		return nil
	}

	h.sendMessageChan <- Message{
		From: h.ID(),
		To:   env.SenderID,
		Type: MessageTypeACK,
		Payload: map[string]any{
			"ack_for": env.ID,
			"status":  "ok",
		},
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
		CheckMessageAccess(),
	}

	msgCtx := NewMessageContext(ctx)

	for _, middleware := range middlewares {
		if msgCtx.IsAborted() {
			return nil
		}
		if err := middleware(msgCtx, h, &env); err != nil {
			return fmt.Errorf("error in middleware: %v", err)
		}
	}

	handler, ok := h.handlers[env.Type]
	if !ok {
		return errors.New("invalid event type")
	}
	if err := handler(msgCtx, h, &env); err != nil {
		return fmt.Errorf("error during handle envelope action: %v", err)
	}

	h.handleAck(ctx, env)
	return nil
}

func (h *Herald) notify(event MessageType, msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.subs[event] {
		select {
		case sub <- msg:
		default:
			// drop or log
		}
	}
}

func (h *Herald) callPeerJoinHook(ctx context.Context, peerID string) {
	for _, hook := range h.hooks.OnPeerJoin {
		hook(ctx, peerID)
	}
}

func (h *Herald) callPeerLeaveHook(ctx context.Context, peerID string) {
	for _, hook := range h.hooks.OnPeerLeave {
		hook(ctx, peerID)
	}
}

func (h *Herald) Start(ctx context.Context) error {
	defer func() {
		if h.transport != nil {
			h.transport.Close()
		}
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

	bc, err := h.transport.SubscribeBroadcast(ctx)
	if err != nil {
		return err
	}
	dc, err := h.transport.SubscribeDirect(ctx, h.ID())
	if err != nil {
		return err
	}

	go h.subscribe(ctx, bc)
	go h.subscribe(ctx, dc)

	if err := h.startHandshake(ctx); err != nil {
		return err
	}

	// start heartbeat publisher
	go h.startHeartbeatPublisher(ctx)
	// check healthcheck
	go h.startCheckHeartbeats(ctx)
	// start message handler
	go h.handleReceiveMessages(ctx)
	// handle start send message
	go h.handleSendMessages(ctx)

	<-ctx.Done()
	h.Close(ctx)
	return ctx.Err()
}

func (h *Herald) SendToPeer(ctx context.Context, peerID string, payload map[string]any) error {
	_, ok := h.registry.PeerByID(peerID)
	if !ok {
		return ErrUnknownPeer
	}

	msg := Message{
		ID:      uuid.NewString(),
		From:    h.ID(),
		To:      peerID,
		Type:    MessageTypeMessage,
		Payload: payload,
	}
	h.Send(ctx, msg)
	return nil
}

func (h *Herald) Send(ctx context.Context, msg Message) {
	h.sendMessageChan <- msg
}

func (h *Herald) SendPayload(ctx context.Context, payload map[string]any) {
	slog.Debug("Debug: send message")

	h.sendMessageChan <- Message{
		Type:    MessageTypeMessage,
		Payload: payload,
		From:    h.ID(),
		To:      "",
	}
}

func (h *Herald) SendMessage(ctx context.Context, msg string) {
	payload := map[string]any{
		"type":    MessageTypeMessage,
		"service": h.ID(),
		"message": msg,
	}
	h.SendPayload(ctx, payload)
}

func (h *Herald) SendAndWait(ctx context.Context, peerID string, payload map[string]any, timeout time.Duration) error {
	msg := Message{
		From:    h.ID(),
		To:      peerID,
		Type:    MessageTypeMessage,
		Payload: payload,
	}
	env, err := h.sendMessage(ctx, msg)
	if err != nil {
		return err
	}

	select {
	case <-h.pending[env.ID].ch:
		return nil
	case <-time.After(timeout):
		return errors.New("ack timeout")
	}
}

func (h *Herald) Subscribe(ctx context.Context, event MessageType, buffer int) Subscription {

	ch := make(chan Message, buffer)

	h.mu.Lock()
	h.subs[event] = append(h.subs[event], ch)
	h.mu.Unlock()

	go func() {
		<-ctx.Done()
		close(ch)
	}()

	return Subscription{
		C: ch,
		cancel: func() {
			close(ch)
		},
	}
}

func (h *Herald) ID() string {
	return h.id
}

func (h *Herald) Peers() []string {
	peers := h.registry.Peers()
	peersID := make([]string, len(peers))
	i := 0
	for id := range peers {
		peersID[i] = id
		i++
	}
	return peersID
}

func (h *Herald) OnPeerJoin(hook PeerHook) {
	h.hooks.OnPeerJoin = append(h.hooks.OnPeerJoin, hook)
}

func (h *Herald) OnPeerLeave(hook PeerHook) {
	h.hooks.OnPeerLeave = append(h.hooks.OnPeerLeave, hook)
}

func (h *Herald) Close(ctx context.Context) error {
	h.sendMessage(ctx, Message{
		Type: MessageTypeOffline,
		Payload: map[string]any{
			"service": h.ID(),
			"message": "bye",
		},
		From: h.ID(),
	})

	select {
	case <-time.After(500 * time.Millisecond):
	case <-ctx.Done():
	}

	if h.transport != nil {
		h.transport.Close()
	}

	close(h.sendMessageChan)
	close(h.receiveMsgCh)

	return nil
}
