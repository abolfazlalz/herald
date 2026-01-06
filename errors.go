package herald

import "errors"

var (
	ErrUnknownPeer      = errors.New("unknown peer")
	ErrInvalidSignature = errors.New("invalid signature")
	ErrUnauthorized     = errors.New("unauthorized")
)
