package transport

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	rabbitMQInitialReconnectBackoff = 500 * time.Millisecond
	rabbitMQMaxReconnectBackoff     = 30 * time.Second
)

var errRabbitMQClosed = errors.New("rabbitmq transport is closed")

type RabbitMQ struct {
	url string

	conn   *amqp.Connection
	pubCh  *amqp.Channel
	consCh *amqp.Channel

	broadcastExchange string
	directExchange    string
	queue             string

	stateMu          sync.RWMutex
	ready            chan struct{}
	readyClosed      bool
	generation       uint64
	generationChange chan struct{}
	closed           bool

	publishMu sync.Mutex
	consumeMu sync.Mutex

	subsMu     sync.Mutex
	subs       map[uint64]*rabbitMQSubscription
	nextSubID  uint64
	closeOnce  sync.Once
	closeDone  chan struct{}
	reconnectW sync.WaitGroup
}

type rabbitMQSubscriptionKind int

const (
	rabbitMQSubscriptionBroadcast rabbitMQSubscriptionKind = iota
	rabbitMQSubscriptionDirect
)

type rabbitMQSubscription struct {
	id     uint64
	kind   rabbitMQSubscriptionKind
	peerID string
	ctx    context.Context
	cancel context.CancelFunc
	out    chan []byte
}

type rabbitMQState struct {
	pubCh            *amqp.Channel
	consCh           *amqp.Channel
	ready            <-chan struct{}
	generationChange <-chan struct{}
	generation       uint64
	closed           bool
	connected        bool
}

type messageData struct {
	SenderID string `json:"sender_id"`
	Payload  []byte `json:"payload"`
}

var _ Transport = (*RabbitMQ)(nil)

func NewRabbitMQ(url string, queue string) (*RabbitMQ, error) {
	mq := &RabbitMQ{
		url:               url,
		broadcastExchange: fmt.Sprintf("herald.%s.broadcast", queue),
		directExchange:    fmt.Sprintf("herald.%s.direct", queue),
		queue:             queue,
		ready:             make(chan struct{}),
		generationChange:  make(chan struct{}),
		subs:              make(map[uint64]*rabbitMQSubscription),
		closeDone:         make(chan struct{}),
	}

	conn, pubCh, consCh, err := mq.connect()
	if err != nil {
		return nil, err
	}
	mq.setConnected(conn, pubCh, consCh)
	mq.watchConnection(conn, pubCh, consCh)

	return mq, nil
}

func (mq *RabbitMQ) PublishBroadcast(ctx context.Context, data []byte) error {
	return mq.publish(ctx, mq.broadcastExchange, "", data)
}

func (mq *RabbitMQ) PublishDirect(ctx context.Context, routingKey string, data []byte) error {
	return mq.publish(ctx, mq.directExchange, routingKey, data)
}

func (mq *RabbitMQ) SubscribeBroadcast(ctx context.Context) (<-chan []byte, error) {
	return mq.subscribe(ctx, rabbitMQSubscriptionBroadcast, "")
}

func (mq *RabbitMQ) SubscribeDirect(ctx context.Context, peerID string) (<-chan []byte, error) {
	return mq.subscribe(ctx, rabbitMQSubscriptionDirect, peerID)
}

func (mq *RabbitMQ) Close() error {
	var closeErr error

	mq.closeOnce.Do(func() {
		close(mq.closeDone)

		mq.stateMu.Lock()
		mq.closed = true
		conn := mq.conn
		pubCh := mq.pubCh
		consCh := mq.consCh
		mq.conn = nil
		mq.pubCh = nil
		mq.consCh = nil
		if !mq.readyClosed {
			close(mq.ready)
			mq.readyClosed = true
		}
		close(mq.generationChange)
		mq.generationChange = make(chan struct{})
		mq.stateMu.Unlock()

		mq.subsMu.Lock()
		for _, sub := range mq.subs {
			sub.cancel()
		}
		mq.subsMu.Unlock()

		if pubCh != nil {
			_ = pubCh.Close()
		}
		if consCh != nil {
			_ = consCh.Close()
		}
		if conn != nil {
			closeErr = conn.Close()
		}

		mq.reconnectW.Wait()
	})

	return closeErr
}

func (mq *RabbitMQ) publish(ctx context.Context, exchange string, routingKey string, data []byte) error {
	for {
		state := mq.snapshot()
		if state.closed {
			return errRabbitMQClosed
		}
		if !state.connected || state.pubCh == nil {
			if err := waitForRabbitMQReady(ctx, state.ready, mq.closeDone); err != nil {
				return err
			}
			continue
		}

		mq.publishMu.Lock()
		err := state.pubCh.PublishWithContext(
			ctx,
			exchange,
			routingKey,
			false,
			false,
			amqp.Publishing{
				ContentType: "application/json",
				Body:        data,
			},
		)
		mq.publishMu.Unlock()

		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}

		mq.triggerReconnect()
		if err := waitForRabbitMQReady(ctx, mq.snapshot().ready, mq.closeDone); err != nil {
			return err
		}
	}
}

func (mq *RabbitMQ) subscribe(ctx context.Context, kind rabbitMQSubscriptionKind, peerID string) (<-chan []byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	mq.stateMu.RLock()
	closed := mq.closed
	mq.stateMu.RUnlock()
	if closed {
		return nil, errRabbitMQClosed
	}

	subCtx, cancel := context.WithCancel(ctx)
	sub := &rabbitMQSubscription{
		kind:   kind,
		peerID: peerID,
		ctx:    subCtx,
		cancel: cancel,
		out:    make(chan []byte),
	}

	mq.subsMu.Lock()
	mq.nextSubID++
	sub.id = mq.nextSubID
	mq.subs[sub.id] = sub
	mq.subsMu.Unlock()

	consCh, generation, err := mq.waitForConsumerChannel(subCtx, 0)
	if err != nil {
		mq.removeSubscription(sub)
		close(sub.out)
		cancel()
		return nil, err
	}

	deliveries, err := mq.registerConsumer(consCh, sub)
	if err != nil {
		mq.removeSubscription(sub)
		close(sub.out)
		cancel()
		mq.triggerReconnect()
		return nil, err
	}

	go mq.runSubscription(sub, deliveries, generation)

	return sub.out, nil
}

func (mq *RabbitMQ) runSubscription(sub *rabbitMQSubscription, deliveries <-chan amqp.Delivery, generation uint64) {
	defer func() {
		mq.removeSubscription(sub)
		close(sub.out)
	}()

	lastGeneration := generation

	for {
		if !forwardRabbitMQDeliveries(sub.ctx, deliveries, sub.out, mq.closeDone) {
			return
		}

		consCh, generation, err := mq.waitForConsumerChannel(sub.ctx, lastGeneration)
		if err != nil {
			return
		}

		deliveries, err = mq.registerConsumer(consCh, sub)
		if err != nil {
			if sub.ctx.Err() != nil {
				return
			}
			mq.triggerReconnect()
			lastGeneration = generation
			continue
		}

		lastGeneration = generation
	}
}

func (mq *RabbitMQ) removeSubscription(sub *rabbitMQSubscription) {
	mq.subsMu.Lock()
	delete(mq.subs, sub.id)
	mq.subsMu.Unlock()
}

func (mq *RabbitMQ) waitForConsumerChannel(ctx context.Context, lastGeneration uint64) (*amqp.Channel, uint64, error) {
	for {
		state := mq.snapshot()
		if state.closed {
			return nil, 0, errRabbitMQClosed
		}
		if state.connected && state.consCh != nil && state.generation != lastGeneration {
			return state.consCh, state.generation, nil
		}

		wait := state.ready
		if state.connected && state.generation == lastGeneration {
			wait = state.generationChange
		}

		select {
		case <-ctx.Done():
			return nil, 0, ctx.Err()
		case <-mq.closeDone:
			return nil, 0, errRabbitMQClosed
		case <-wait:
		}
	}
}

func (mq *RabbitMQ) registerConsumer(ch *amqp.Channel, sub *rabbitMQSubscription) (<-chan amqp.Delivery, error) {
	mq.consumeMu.Lock()
	defer mq.consumeMu.Unlock()

	switch sub.kind {
	case rabbitMQSubscriptionBroadcast:
		return mq.registerBroadcastConsumer(ch, sub)
	case rabbitMQSubscriptionDirect:
		return mq.registerDirectConsumer(ch, sub)
	default:
		return nil, fmt.Errorf("unknown rabbitmq subscription kind: %d", sub.kind)
	}
}

func (mq *RabbitMQ) registerBroadcastConsumer(ch *amqp.Channel, sub *rabbitMQSubscription) (<-chan amqp.Delivery, error) {
	q, err := ch.QueueDeclare(
		"",
		false,
		true,
		true,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	if err := ch.QueueBind(
		q.Name,
		"",
		mq.broadcastExchange,
		false,
		nil,
	); err != nil {
		return nil, err
	}

	return ch.Consume(
		q.Name,
		fmt.Sprintf("herald-broadcast-%d", sub.id),
		true,
		false,
		false,
		false,
		nil,
	)
}

func (mq *RabbitMQ) registerDirectConsumer(ch *amqp.Channel, sub *rabbitMQSubscription) (<-chan amqp.Delivery, error) {
	queueName := "peer." + sub.peerID

	q, err := ch.QueueDeclare(
		queueName,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	if err := ch.QueueBind(
		q.Name,
		sub.peerID,
		mq.directExchange,
		false,
		nil,
	); err != nil {
		return nil, err
	}

	return ch.Consume(
		q.Name,
		fmt.Sprintf("herald-direct-%d-%s", sub.id, sub.peerID),
		true,
		false,
		false,
		false,
		nil,
	)
}

func (mq *RabbitMQ) connect() (*amqp.Connection, *amqp.Channel, *amqp.Channel, error) {
	conn, err := amqp.Dial(mq.url)
	if err != nil {
		return nil, nil, nil, err
	}

	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, err
	}

	consCh, err := conn.Channel()
	if err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return nil, nil, nil, err
	}

	if err := declareRabbitMQExchanges(pubCh, mq.broadcastExchange, mq.directExchange); err != nil {
		_ = consCh.Close()
		_ = pubCh.Close()
		_ = conn.Close()
		return nil, nil, nil, err
	}

	return conn, pubCh, consCh, nil
}

func declareRabbitMQExchanges(ch *amqp.Channel, broadcastExchange string, directExchange string) error {
	if err := ch.ExchangeDeclare(
		broadcastExchange,
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return err
	}

	return ch.ExchangeDeclare(
		directExchange,
		"direct",
		true,
		false,
		false,
		false,
		nil,
	)
}

func (mq *RabbitMQ) setConnected(conn *amqp.Connection, pubCh *amqp.Channel, consCh *amqp.Channel) bool {
	mq.stateMu.Lock()
	if mq.closed {
		mq.stateMu.Unlock()
		return false
	}
	mq.conn = conn
	mq.pubCh = pubCh
	mq.consCh = consCh
	mq.generation++
	if !mq.readyClosed {
		close(mq.ready)
	}
	mq.readyClosed = true
	close(mq.generationChange)
	mq.generationChange = make(chan struct{})
	mq.stateMu.Unlock()
	return true
}

func (mq *RabbitMQ) triggerReconnect() {
	if !mq.setDisconnected() {
		return
	}

	mq.reconnectW.Add(1)
	go func() {
		defer mq.reconnectW.Done()
		mq.reconnect()
	}()
}

func (mq *RabbitMQ) setDisconnected() bool {
	mq.stateMu.Lock()
	if mq.closed || !mq.readyClosed {
		mq.stateMu.Unlock()
		return false
	}

	conn := mq.conn
	pubCh := mq.pubCh
	consCh := mq.consCh
	mq.conn = nil
	mq.pubCh = nil
	mq.consCh = nil
	mq.ready = make(chan struct{})
	mq.readyClosed = false
	mq.stateMu.Unlock()

	if pubCh != nil {
		_ = pubCh.Close()
	}
	if consCh != nil {
		_ = consCh.Close()
	}
	if conn != nil {
		_ = conn.Close()
	}

	return true
}

func (mq *RabbitMQ) reconnect() {
	backoff := rabbitMQInitialReconnectBackoff

	for {
		select {
		case <-mq.closeDone:
			return
		default:
		}

		conn, pubCh, consCh, err := mq.connect()
		if err == nil {
			if mq.setConnected(conn, pubCh, consCh) {
				mq.watchConnection(conn, pubCh, consCh)
			} else {
				_ = consCh.Close()
				_ = pubCh.Close()
				_ = conn.Close()
			}
			return
		}

		select {
		case <-time.After(backoff):
		case <-mq.closeDone:
			return
		}
		backoff = nextRabbitMQBackoff(backoff)
	}
}

func (mq *RabbitMQ) watchConnection(conn *amqp.Connection, pubCh *amqp.Channel, consCh *amqp.Channel) {
	connClosed := conn.NotifyClose(make(chan *amqp.Error, 1))
	pubClosed := pubCh.NotifyClose(make(chan *amqp.Error, 1))
	consClosed := consCh.NotifyClose(make(chan *amqp.Error, 1))

	go func() {
		select {
		case <-connClosed:
			mq.triggerReconnect()
		case <-pubClosed:
			mq.triggerReconnect()
		case <-consClosed:
			mq.triggerReconnect()
		case <-mq.closeDone:
		}
	}()
}

func (mq *RabbitMQ) snapshot() rabbitMQState {
	mq.stateMu.RLock()
	defer mq.stateMu.RUnlock()

	return rabbitMQState{
		pubCh:            mq.pubCh,
		consCh:           mq.consCh,
		ready:            mq.ready,
		generationChange: mq.generationChange,
		generation:       mq.generation,
		closed:           mq.closed,
		connected:        mq.readyClosed,
	}
}

func forwardRabbitMQDeliveries(
	ctx context.Context,
	deliveries <-chan amqp.Delivery,
	out chan<- []byte,
	closeDone <-chan struct{},
) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case <-closeDone:
			return false
		case msg, ok := <-deliveries:
			if !ok {
				return true
			}
			select {
			case out <- msg.Body:
			case <-ctx.Done():
				return false
			case <-closeDone:
				return false
			}
		}
	}
}

func waitForRabbitMQReady(ctx context.Context, ready <-chan struct{}, closeDone <-chan struct{}) error {
	select {
	case <-ready:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-closeDone:
		return errRabbitMQClosed
	}
}

func nextRabbitMQBackoff(current time.Duration) time.Duration {
	next := current * 2
	if next > rabbitMQMaxReconnectBackoff {
		return rabbitMQMaxReconnectBackoff
	}
	return next
}
