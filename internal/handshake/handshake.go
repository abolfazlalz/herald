package handshake

import (
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/abolfazlalz/herald/internal/message"
	"github.com/abolfazlalz/herald/internal/registry"
	"github.com/abolfazlalz/herald/internal/security"
)

type HandshakePayload struct {
	Type     string `json:"type"`
	Service  string `json:"service"`
	PubKey   string `json:"pub_key"`
	RouteKey string `json:"route_key"`
}

// InitiateHandshake creates a Hello Envelope and signs it
func InitiateHandshake(selfID string, kp *security.KeyPair, signer security.Signer) (*message.Envelope, error) {
	hs := HandshakePayload{
		Type:     "hello",
		Service:  selfID,
		PubKey:   string(kp.PublicBytes()),
		RouteKey: selfID,
	}
	payload, err := json.Marshal(hs)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}
	env := message.NewEnvelope(message.EventAnnounce, selfID, "", payload)
	return env, nil
}

// RespondHandshake verifies incoming Hello and stores peer public key
func RespondHandshake(env *message.Envelope, verifier security.Verifier, registry *registry.PeerRegistry) error {
	var hs HandshakePayload
	err := json.Unmarshal(env.Payload, &hs)
	if err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	// extract pub_key from payload
	pubKey, err := base64.StdEncoding.DecodeString(hs.PubKey)
	if err != nil {
		return err
	}

	// verify message
	if err := env.Verify(verifier, pubKey); err != nil {
		return fmt.Errorf("handshake verify failed: %w", err)
	}

	registry.Add(env.SenderID, pubKey, hs.RouteKey)
	return nil
}
