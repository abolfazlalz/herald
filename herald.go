package herald

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/abolfazlalz/herald/internal/acknowledge"
	"github.com/abolfazlalz/herald/internal/handshake"
	"github.com/abolfazlalz/herald/internal/heartbeat"
	"github.com/abolfazlalz/herald/internal/message"
	"github.com/abolfazlalz/herald/internal/offline"
	"github.com/abolfazlalz/herald/internal/registry"
	"github.com/abolfazlalz/herald/internal/security"
	"github.com/abolfazlalz/herald/transport"
	"github.com/google/uuid"
)

const (
	// PeerTimeout defines the duration after which a peer is considered offline.
	PeerTimeout           = 5 * time.Second
	MessageTimeout        = 10 * time.Second
	PeerConnectingTimeout = 5 * time.Second
)

// pendingAck represents a pending acknowledgement for a sent message.
type pendingAck struct {
	ch chan struct{}
}

// Herald represents the main P2P engine that handles messaging, heartbeat,
// peer registry, and hooks for join/leave events.
type Herald struct {
	transport transport.Transport
	id        string

	kp       *security.KeyPair
	signer   security.Signer
	verifier security.Verifier
	registry *registry.PeerRegistry

	subs            map[MessageType][]chan Message
	receiveMsgCh    chan Message
	sendMessageChan chan *message.Envelope
	mu              sync.RWMutex

	pending map[string]*pendingAck

	pendingPeers map[string][]chan int8

	handlers map[message.EventType]handlerFunc

	hooks Hook

	logger Log
}

type Option struct {
	Logger Log
}

// New creates a new Herald instance, generating a key pair and setting up default handlers.
func New(transport transport.Transport, opts *Option) (*Herald, error) {
	id := uuid.New().String()

	kp, err := security.Generate()
	if err != nil {
		return nil, err
	}

	verifier := security.NewVerifier()
	signer, err := security.NewSigner(kp)
	if err != nil {
		return nil, err
	}

	var logger Log
	if opts == nil || opts.Logger == nil {
		logger = NewLogger(&slog.HandlerOptions{Level: slog.LevelDebug})
	} else {
		logger = opts.Logger
	}

	h := &Herald{
		transport:       transport,
		id:              id,
		kp:              kp,
		verifier:        verifier,
		signer:          signer,
		registry:        registry.NewPeerRegistry(),
		receiveMsgCh:    make(chan Message),
		subs:            make(map[MessageType][]chan Message),
		sendMessageChan: make(chan *message.Envelope),
		pending:         make(map[string]*pendingAck),
		pendingPeers:    make(map[string][]chan int8),
		logger:          logger,
	}
	h.handlers = map[message.EventType]handlerFunc{
		message.EventAnnounce:  handleAnnounce(),
		message.EventHeartbeat: handleHeartbeat(),
		message.EventMessage:   handleMessage(h.receiveMsgCh),
		message.EventAck:       handleAck(),
		message.EventOffline:   handleOffline(),
	}
	return h, nil
}

// startHandshake initiates the handshake with peers.
func (h *Herald) startHandshake(ctx context.Context) error {
	msg, err := handshake.InitiateHandshake(h.id, h.kp, h.signer)
	if err != nil {
		return err
	}
	return h.publish(ctx, msg)
}

// startCheckHeartbeats monitors all peers and removes those who have timed out.
func (h *Herald) startCheckHeartbeats(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
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

// startHeartbeatPublisher periodically sends heartbeat messages to peers.
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

// handleReceiveMessages reads messages from the receive channel and notifies subscribers.
func (h *Herald) handleReceiveMessages(ctx context.Context) {
	for {
		select {
		case msg := <-h.receiveMsgCh:
			h.notify(msg.Type, msg)
		case <-ctx.Done():
			return
		}
	}
}

// handleSendMessages continuously publishes messages from the sendMessageChan.
func (h *Herald) handleSendMessages(ctx context.Context) {
	for {
		select {
		case msg := <-h.sendMessageChan:
			h.logger.Info(ctx, "handle send message", "type", msg.Type)
			h.publish(ctx, msg)
		case <-ctx.Done():
			return
		}
	}
}

// publish signs, marshals, and sends a message envelope either directly or as broadcast.
// It also registers the message in the pending ack map.
func (h *Herald) publish(ctx context.Context, env *message.Envelope) error {
	if env.Type != message.EventHeartbeat {
		h.logger.Info(ctx, "call publish", "type", env.Type, "id", env.ReceiverID)
	}
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

	return nil
}

// subscribe listens to incoming raw messages and executes them.
func (h *Herald) subscribe(ctx context.Context, dataCh <-chan []byte) error {
	for data := range dataCh {
		var env message.Envelope
		if err := json.Unmarshal(data, &env); err != nil {
			h.logger.Error(ctx, "error during execute message", "error", err.Error())
			continue
		}

		if err := h.executeMessage(ctx, env); err != nil {
			h.logger.Error(ctx, "error during execute message", "error", err.Error(), "type", env.Type, "correlation_id", env.CorrelationID, "sender", env.SenderID)
		}
	}
	return nil
}

// handleAck sends an acknowledgement for a received message.
func (h *Herald) handleAck(ctx context.Context, env message.Envelope) error {
	h.logger.Debug(ctx, "handleAck", "type", env.Type, "correlation_id", env.CorrelationID)
	ackEnv, err := acknowledge.InitiateAcknowledge(h.ID(), env.SenderID, env.CorrelationID, "OK")
	if err != nil {
		return err
	}
	h.logger.Debug(ctx, "handleAck Send", "type", env.Type, "correlation_id", env.CorrelationID)
	h.send(ctx, ackEnv)
	return nil
}

// executeMessage runs middleware and handler functions for an incoming message.
func (h *Herald) executeMessage(ctx context.Context, env message.Envelope) error {
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
			return fmt.Errorf("error in middleware: %s", err.Error())
		}
	}

	if env.Type != message.EventHeartbeat {
		h.logger.Info(ctx, "execute message", "correlation_id", env.CorrelationID, "type", env.Type, "payload", string(env.Payload), "sender", env.SenderID)
	}

	handler, ok := h.handlers[env.Type]
	if !ok {
		return ErrInvalidEventType
	}
	if err := handler(msgCtx, h, &env); err != nil {
		return fmt.Errorf("error during handle envelope action: %v", err)
	}

	if !slices.Contains([]message.EventType{message.EventHeartbeat, message.EventAck}, env.Type) {
		h.handleAck(ctx, env)
	}
	return nil
}

// notify sends a message to all subscribers of a specific event type.
func (h *Herald) notify(event MessageType, msg Message) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, sub := range h.subs[event] {
		select {
		case sub <- msg:
		default:
		}
	}
}

// callPeerJoinHook triggers all registered OnPeerJoin hooks.
func (h *Herald) callPeerJoinHook(ctx context.Context, peerID string) {
	for _, hook := range h.hooks.OnPeerJoin {
		hook(ctx, peerID)
	}
}

// callPeerLeaveHook triggers all registered OnPeerLeave hooks.
func (h *Herald) callPeerLeaveHook(ctx context.Context, peerID string) {
	for _, hook := range h.hooks.OnPeerLeave {
		hook(ctx, peerID)
	}
}

// Start runs the Herald engine, subscribes to messages, performs handshake,
// starts heartbeat, and manages message sending/receiving.
func (h *Herald) Start(ctx context.Context) error {
	defer func() {
		if h.transport != nil {
			h.transport.Close()
		}
	}()
	h.logger.Debug(ctx, "Service started", "ID", h.ID())

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

	go h.startHeartbeatPublisher(ctx)
	go h.startCheckHeartbeats(ctx)
	go h.handleReceiveMessages(ctx)
	go h.handleSendMessages(ctx)

	<-ctx.Done()
	h.Close(ctx)
	return ctx.Err()
}

// SendToPeer sends a message to a specific peer.
func (h *Herald) SendToPeer(ctx context.Context, peerID string, payload []byte) error {
	_, ok := h.registry.PeerByID(peerID)
	if !ok {
		h.logger.Debug(ctx, "sendToPeer unknown peer given", "payload", string(payload), "peer_id", peerID)
		return ErrUnknownPeer
	}
	if peerID == h.ID() {
		return ErrSelfMessage
	}

	env := message.NewEnvelope(message.EventMessage, h.ID(), peerID, payload)
	if err := h.sendAndWait(ctx, env, MessageTimeout); err != nil {
		return err
	}
	return nil
}

// send pushes a message envelope into the sendMessageChan.
func (h *Herald) send(ctx context.Context, msg *message.Envelope) {
	select {
	case <-ctx.Done():
		return
	case h.sendMessageChan <- msg:
	}
}

// sendAndWait sends a envelope to a specific peer.
func (h *Herald) sendAndWait(ctx context.Context, msg *message.Envelope, timeout time.Duration) error {
	h.logger.Debug(ctx, "start sending message", "type", msg.Type, "peer_id", msg.ReceiverID, "method", "sendAndWait", "correlation_id", msg.CorrelationID, "payload", string(msg.Payload))

	if msg.ReceiverID != "" && msg.Type == message.EventMessage && msg.Type != message.EventAck {
		peer, ok := h.registry.PeerByID(msg.ReceiverID)
		if ok && peer.Status != registry.PeerStatusConnected {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-h.registry.Wait(msg.ReceiverID):
			case <-time.After(MessageTimeout):
				h.logger.Error(ctx, "send timeout", "correlation_id", msg.CorrelationID, "type", msg.Type)
				return ErrMessageTimeout
			}
		}
	}

	ack := make(chan struct{})
	h.mu.Lock()
	h.pending[msg.CorrelationID] = &pendingAck{ch: ack}
	h.mu.Unlock()

	go h.send(ctx, msg)

	select {
	case <-h.pending[msg.CorrelationID].ch:
		h.logger.Debug(ctx, "correlationID has completed", "correlation_id", msg.CorrelationID)
		return nil
	case <-time.After(timeout):
		return ErrAckTimeout
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Broadcast sends a message to all peers.
func (h *Herald) Broadcast(ctx context.Context, payload []byte) {
	env := message.NewEnvelope(message.EventMessage, h.ID(), "", payload)
	h.send(ctx, env)
}

// Subscribe subscribes to a specific message type with a buffered channel.
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
		C:      ch,
		cancel: func() { close(ch) },
	}
}

// ID returns the unique identifier of this Herald instance.
func (h *Herald) ID() string {
	return h.id
}

// Peers returns a list of peer IDs currently in the registry.
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

// OnPeerJoin registers a hook for when a peer joins.
func (h *Herald) OnPeerJoin(hook PeerHook) {
	h.hooks.OnPeerJoin = append(h.hooks.OnPeerJoin, hook)
}

// OnPeerLeave registers a hook for when a peer leaves.
func (h *Herald) OnPeerLeave(hook PeerHook) {
	h.hooks.OnPeerLeave = append(h.hooks.OnPeerLeave, hook)
}

// Close sends an offline message and shuts down the Herald instance.
func (h *Herald) Close(ctx context.Context) error {
	msg, err := offline.InitOffline(h.ID(), "close")
	if err != nil {
		return err
	}
	h.publish(ctx, msg)

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
