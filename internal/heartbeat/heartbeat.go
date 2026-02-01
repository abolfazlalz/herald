package heartbeat

import (
	"encoding/json"

	"github.com/abolfazlalz/herald/internal/message"
	"github.com/abolfazlalz/herald/internal/security"
)

type HeartbeatPayload struct {
	Type    string `json:"type"`
	Service string `json:"service"`
}

// InitiateHeartbeat creates a heartbeat Envelope and signs it
func InitiateHeartbeat(selfID string, kp *security.KeyPair) (*message.Envelope, error) {
	hb := HeartbeatPayload{
		Type:    "heartbeat",
		Service: selfID,
	}
	payload, err := json.Marshal(hb)
	if err != nil {
		return nil, err
	}
	return message.NewEnvelope(message.EventHeartbeat, selfID, "", payload), nil
}
