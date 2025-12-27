package rabbitmq

import (
	"encoding/json"
	"log"

	"github.com/abolfazlalz/herald/internal/message"
	"github.com/abolfazlalz/herald/internal/transport"
	amqp "github.com/rabbitmq/amqp091-go"
)

type RabbitMQAdapter struct {
	conn    *amqp.Connection
	channel *amqp.Channel
	queue   string
}

func New(url string, serviceID string) (*RabbitMQAdapter, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, err
	}

	exchange := "heartbeat_exchange"
	queueName := "heartbeat." + serviceID + ".queue"

	// exchange
	if err := ch.ExchangeDeclare(
		exchange,
		"fanout",
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return nil, err
	}

	// UNIQUE queue per service
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

	// bind
	if err := ch.QueueBind(
		q.Name,
		"",
		exchange,
		false,
		nil,
	); err != nil {
		return nil, err
	}

	return &RabbitMQAdapter{
		conn:    conn,
		channel: ch,
		queue:   q.Name,
	}, nil
}

// Publish implements Transport
func (r *RabbitMQAdapter) Publish(exchange, routingKey string, env *message.Envelope) error {
	body, _ := json.Marshal(env)
	return r.channel.Publish(exchange, routingKey, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

// Subscribe implements Transport
func (r *RabbitMQAdapter) Subscribe(queue string, handler func(*message.Envelope)) error {
	msgs, err := r.channel.Consume(queue, "", true, false, false, false, nil)
	if err != nil {
		return err
	}

	go func() {
		for d := range msgs {
			var env message.Envelope
			if err := json.Unmarshal(d.Body, &env); err != nil {
				log.Println("Failed to unmarshal envelope:", err)
				continue
			}
			handler(&env)
		}
	}()

	return nil
}

// Close connection
func (r *RabbitMQAdapter) Close() error {
	r.channel.Close()
	return r.conn.Close()
}

// Ensure RabbitMQAdapter satisfies Transport interface
var _ transport.Transport = (*RabbitMQAdapter)(nil)
