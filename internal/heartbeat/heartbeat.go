package heartbeat

import (
	"github.com/abolfazlalz/herald/internal/message"
	"github.com/abolfazlalz/herald/internal/security"
)

// InitiateHeartbeat creates a heartbeat Envelope and signs it
func InitiateHeartbeat(selfID string, kp *security.KeyPair) (*message.Envelope, error) {
	payload := map[string]any{
		"type":    "heartbeat",
		"service": selfID,
	}
	return message.NewEnvelope(message.EventHeartbeat, selfID, "", payload), nil
}
