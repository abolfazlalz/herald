package transport

import "github.com/abolfazlalz/herald/internal/message"

// Transport defines the generic messaging operations
type Transport interface {
	Publish(exchange string, routingKey string, env *message.Envelope) error
	Subscribe(queue string, handler func(*message.Envelope)) error
	Close() error
}
