package heartbeat

import (
	"fmt"

	"github.com/abolfazlalz/herald/internal/message"
	"github.com/abolfazlalz/herald/internal/security"
)

// InitiateHeartbeat creates a heartbeat Envelope and signs it
func InitiateHeartbeat(selfID string, kp *security.KeyPair, signer security.Signer) (*message.Envelope, error) {
	payload := map[string]any{
		"type":    "heartbeat",
		"service": selfID,
	}
	env := message.NewEnvelope(message.EventHeartbeat, selfID, payload)
	if err := env.Sign(signer); err != nil {
		return nil, fmt.Errorf("heartbeat sign failed: %v", err)
	}
	return env, nil
}
