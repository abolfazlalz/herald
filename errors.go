package herald

import "errors"

var (
	ErrUnknownPeer      = errors.New("unknown peer")
	ErrInvalidSignature = errors.New("invalid signature")
	ErrUnauthorized     = errors.New("unauthorized")
	ErrSelfMessage      = errors.New("self message")
	ErrInvalidMessage   = errors.New("invalid message")
	ErrMessageTimeout   = errors.New("message timeout")
	ErrAckTimeout       = errors.New("ack timeout")
	ErrInvalidEventType = errors.New("invalid event type")
)
