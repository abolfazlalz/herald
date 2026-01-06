package herald

import (
	"context"
	"sync"

	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	conn *amqp.Connection

	pubCh  *amqp.Channel
	consCh *amqp.Channel

	broadcastExchange string
	directExchange    string
	queue             string

	closeOnce sync.Once
}

type messageData struct {
	SenderID string `json:"sender_id"`
	Payload  []byte `json:"payload"`
}

var _ Transport = (*RabbitMQ)(nil)

func NewRabbitMQ(url string, queue string) (*RabbitMQ, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	pubCh, err := conn.Channel()
	if err != nil {
		conn.Close()
		return nil, err
	}

	consCh, err := conn.Channel()
	if err != nil {
		pubCh.Close()
		conn.Close()
		return nil, err
	}

	// Broadcast exchange
	if err := pubCh.ExchangeDeclare(
		"herald.broadcast",
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, err
	}

	// Direct exchange (P2P)
	if err := pubCh.ExchangeDeclare(
		"herald.direct",
		"direct",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, err
	}

	return &RabbitMQ{
		conn:              conn,
		pubCh:             pubCh,
		consCh:            consCh,
		broadcastExchange: "herald.broadcast",
		directExchange:    "herald.direct",
		queue:             queue,
	}, nil
}

func (mq *RabbitMQ) PublishBroadcast(ctx context.Context, data []byte) error {
	return mq.pubCh.PublishWithContext(
		ctx,
		mq.broadcastExchange,
		"",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        data,
		},
	)
}

func (mq *RabbitMQ) PublishDirect(ctx context.Context, routingKey string, data []byte) error {
	return mq.pubCh.PublishWithContext(
		ctx,
		mq.directExchange,
		routingKey,
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        data,
		},
	)
}

func (mq *RabbitMQ) SubscribeBroadcast(ctx context.Context) (<-chan []byte, error) {
	q, err := mq.consCh.QueueDeclare(
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

	if err := mq.consCh.QueueBind(
		q.Name,
		"",
		mq.broadcastExchange,
		false,
		nil,
	); err != nil {
		return nil, err
	}

	msgs, err := mq.consCh.Consume(
		q.Name,
		"",
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	out := make(chan []byte)
	go forwardMessages(ctx, msgs, out)
	return out, nil
}

func (mq *RabbitMQ) SubscribeDirect(ctx context.Context, peerID string) (<-chan []byte, error) {
	queueName := "peer." + peerID

	q, err := mq.consCh.QueueDeclare(
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

	if err := mq.consCh.QueueBind(
		q.Name,
		peerID,
		mq.directExchange,
		false,
		nil,
	); err != nil {
		return nil, err
	}

	msgs, err := mq.consCh.Consume(
		q.Name,
		peerID,
		true,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	out := make(chan []byte)
	go forwardMessages(ctx, msgs, out)
	return out, nil
}

func (mq *RabbitMQ) Close() error {
	var err error
	mq.closeOnce.Do(func() {
		if mq.pubCh != nil {
			_ = mq.pubCh.Close()
		}
		if mq.consCh != nil {
			_ = mq.consCh.Close()
		}
		if mq.conn != nil {
			err = mq.conn.Close()
		}
	})
	return err
}

func forwardMessages(
	ctx context.Context,
	msgs <-chan amqp.Delivery,
	out chan<- []byte,
) {
	defer close(out)
	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-msgs:
			if !ok {
				return
			}
			out <- msg.Body
		}
	}
}
