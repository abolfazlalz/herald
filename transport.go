package herald

import "context"

type Transport interface {
	Publish(ctx context.Context, data []byte) error
	Subscribe(ctx context.Context) (<-chan []byte, error)
	Close() error
}
