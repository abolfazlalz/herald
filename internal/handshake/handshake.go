package handshake

import (
	"encoding/base64"
	"fmt"

	"github.com/abolfazlalz/herald/internal/message"
	"github.com/abolfazlalz/herald/internal/registry"
	"github.com/abolfazlalz/herald/internal/security"
)

// InitiateHandshake creates a Hello Envelope and signs it
func InitiateHandshake(selfID string, kp *security.KeyPair, signer security.Signer) (*message.Envelope, error) {
	payload := map[string]any{
		"type":    "hello",
		"service": selfID,
		"pub_key": string(kp.PublicBytes()),
	}
	env := message.NewEnvelope(message.EventAnnounce, selfID, payload)
	if err := env.Sign(signer); err != nil {
		return nil, fmt.Errorf("handshake sign failed: %v", err)
	}
	return env, nil
}

// RespondHandshake verifies incoming Hello and stores peer public key
func RespondHandshake(env *message.Envelope, verifier security.Verifier, registry *registry.PeerRegistry) error {
	// extract pub_key from payload
	pubKeyStr, ok := env.Payload["pub_key"].(string)
	if !ok {
		return fmt.Errorf("pub_key missing")
	}
	pubKey, err := base64.StdEncoding.DecodeString(pubKeyStr)
	if err != nil {
		return err
	}

	// verify message
	if err := env.Verify(verifier, pubKey); err != nil {
		return fmt.Errorf("handshake verify failed: %w", err)
	}

	registry.Add(env.SenderID, pubKey)
	fmt.Printf("✅ Handshake completed with peer %s\n", env.SenderID)
	return nil
}
