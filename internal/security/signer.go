package security

import (
	"crypto/ed25519"
	"errors"
)

type Signer interface {
	Sign([]byte) ([]byte, error)
}

type Ed25519Signer struct {
	privateKey ed25519.PrivateKey
}

func NewSigner(kp *KeyPair) (*Ed25519Signer, error) {
	if kp == nil {
		return nil, errors.New("keypair is nil")
	}

	if err := kp.Validate(); err != nil {
		return nil, err
	}

	return &Ed25519Signer{
		privateKey: kp.Private,
	}, nil
}

func (s *Ed25519Signer) Sign(data []byte) ([]byte, error) {
	if s == nil {
		return nil, errors.New("signer is nil")
	}

	if len(data) == 0 {
		return nil, errors.New("cannot sign empty data")
	}

	signature := ed25519.Sign(s.privateKey, data)
	return signature, nil
}
