package herald

import (
	"context"
	"encoding/json"
	"errors"
	"sync"

	"github.com/google/uuid"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQ struct {
	conn       *amqp.Connection
	ch         *amqp.Channel
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

	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}

	if err := ch.ExchangeDeclare(
		exchange,
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		_ = ch.Close()
		_ = conn.Close()
		return nil, err
	}

	return &RabbitMQ{
		conn:       conn,
		ch:         ch,
		exchange:   exchange,
		consumerID: uuid.NewString(),
	}, nil
}

func (mq *RabbitMQ) Publish(ctx context.Context, data []byte) error {
	if mq.ch == nil {
		return errors.New("rabbitmq channel is nil")
	}

	msg := messageData{
		SenderID: mq.consumerID,
		Payload:  data,
	}

	body, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return mq.ch.PublishWithContext(
		ctx,
		mq.exchange,
		"",
		false,
		false,
		amqp.Publishing{
			ContentType:  "application/json",
			Body:         body,
			DeliveryMode: amqp.Persistent,
		},
	)
}

func (mq *RabbitMQ) Subscribe(ctx context.Context) (<-chan []byte, error) {
	queueName := "consumer_queue_" + mq.consumerID

	q, err := mq.ch.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto delete
		false, // exclusive
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	if err := mq.ch.QueueBind(
		q.Name,
		"",
		mq.exchange,
		false,
		nil,
	); err != nil {
		return nil, err
	}

	msgs, err := mq.ch.Consume(
		q.Name,
		mq.consumerID, // consumer tag
		false,         // manual ack
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

				var data messageData
				if err := json.Unmarshal(msg.Body, &data); err != nil {
					_ = msg.Nack(false, false)
					continue
				}

				out <- data.Payload
				_ = msg.Ack(false)
			}
		}
	}()

	return out, nil
}

func (mq *RabbitMQ) Close() error {
	var err error

	mq.closeOnce.Do(func() {
		if mq.ch != nil {
			_ = mq.ch.Close()
		}
		if mq.conn != nil {
			err = mq.conn.Close()
		}
	})

	return err
}
