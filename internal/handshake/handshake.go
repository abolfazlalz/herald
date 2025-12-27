package handshake

import (
	"encoding/base64"
	"fmt"
	"log"

	"github.com/abolfazlalz/herald/internal/message"
	"github.com/abolfazlalz/herald/internal/security"
)

type PeerRegistry struct {
	peers map[string][]byte // peerID -> publicKey
}

func NewPeerRegistry() *PeerRegistry {
	return &PeerRegistry{
		peers: make(map[string][]byte),
	}
}

func (pr *PeerRegistry) Peers() map[string][]byte {
	return pr.peers
}

// InitiateHandshake creates a Hello Envelope and signs it
func InitiateHandshake(selfID string, kp *security.KeyPair, signer security.Signer) *message.Envelope {
	payload := map[string]any{
		"type":    "hello",
		"service": selfID,
		"pub_key": string(kp.PublicBytes()),
	}
	env := message.NewEnvelope(message.EventAnnounce, selfID, payload)
	if err := env.Sign(signer); err != nil {
		log.Fatal("handshake sign failed:", err)
	}
	return env
}

// RespondHandshake verifies incoming Hello and stores peer public key
func RespondHandshake(env *message.Envelope, verifier *security.Ed25519Verifier, registry *PeerRegistry) error {
	// extract pub_key from payload
	payloadMap, ok := env.Payload.(map[string]any)
	if !ok {
		return fmt.Errorf("invalid payload")
	}
	pubKeyStr, ok := payloadMap["pub_key"].(string)
	if !ok {
		return fmt.Errorf("pub_key missing")
	}
	pubKey, err := base64.StdEncoding.DecodeString(pubKeyStr)
	if err != nil {
		return fmt.Errorf("invalid public key: %w", err)
	}

	// verify message
	if err := env.Verify(verifier, pubKey); err != nil {
		return fmt.Errorf("handshake verify failed: %w", err)
	}

	// store in registry
	registry.peers[env.SenderID] = pubKey
	fmt.Printf("✅ Handshake completed with peer %s\n", env.SenderID)
	return nil
}
