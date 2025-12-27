package security

import (
	"crypto/ed25519"
	"errors"
)

// Verifier interface
type Verifier interface {
	Verify(data []byte, signature []byte, pubKey ed25519.PublicKey) error
}

// Ed25519Verifier implements Verifier
type Ed25519Verifier struct{}

func NewVerifier() *Ed25519Verifier {
	return &Ed25519Verifier{}
}

// Verify checks signature on canonicalized data
func (v *Ed25519Verifier) Verify(data []byte, signature []byte, pubKey ed25519.PublicKey) error {
	if len(data) == 0 {
		return errors.New("data is empty")
	}
	if signature == nil || len(signature) != ed25519.SignatureSize {
		return errors.New("invalid signature size")
	}
	if pubKey == nil || len(pubKey) != ed25519.PublicKeySize {
		return errors.New("invalid public key size")
	}

	if !ed25519.Verify(pubKey, data, signature) {
		return errors.New("signature verification failed")
	}

	return nil
}
