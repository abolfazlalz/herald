package herald

import (
	"context"
	"errors"
	"sync"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	conn *amqp.Connection

	pubCh  *amqp.Channel
	consCh *amqp.Channel

	exchange   string
	consumerID string

	closeOnce sync.Once
}

type messageData struct {
	SenderID string `json:"sender_id"`
	Payload  []byte `json:"payload"`
}

var _ Transport = (*RabbitMQ)(nil)

func NewRabbitMQ(url, exchange string) (*RabbitMQ, error) {
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

	if err := pubCh.ExchangeDeclare(
		exchange,
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		consCh.Close()
		pubCh.Close()
		conn.Close()
		return nil, err
	}

	return &RabbitMQ{
		conn:       conn,
		pubCh:      pubCh,
		consCh:     consCh,
		exchange:   exchange,
		consumerID: uuid.NewString(),
	}, nil
}

func (mq *RabbitMQ) Publish(ctx context.Context, data []byte) error {
	if mq.pubCh == nil {
		return errors.New("publish channel is nil")
	}

	return mq.pubCh.PublishWithContext(
		ctx,
		mq.exchange,
		"",
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        data,
		},
	)
}

func (mq *RabbitMQ) Subscribe(ctx context.Context) (<-chan []byte, error) {
	queueName := "consumer_queue_" + mq.consumerID

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
		"",
		mq.exchange,
		false,
		nil,
	); err != nil {
		return nil, err
	}

	msgs, err := mq.consCh.Consume(
		q.Name,
		mq.consumerID,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	out := make(chan []byte)

	go func() {
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
				_ = msg.Ack(false)
			}
		}
	}()

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
