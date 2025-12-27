package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
)

type KeyPair struct {
	Private ed25519.PrivateKey
	Public  ed25519.PublicKey
}

func Generate() (*KeyPair, error) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}

	return &KeyPair{
		Private: priv,
		Public:  pub,
	}, nil
}

func LoadFromBytes(privateKey []byte) (*KeyPair, error) {
	if len(privateKey) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf(
			"invalid private key size: got %d, want %d",
			len(privateKey),
			ed25519.PrivateKeySize,
		)
	}

	priv := ed25519.PrivateKey(privateKey)
	pub, ok := priv.Public().(ed25519.PublicKey)
	if !ok {
		return nil, errors.New("failed to derive public key")
	}

	return &KeyPair{
		Private: priv,
		Public:  pub,
	}, nil
}

func (k *KeyPair) PublicBytes() []byte {
	return []byte(base64.StdEncoding.EncodeToString(k.Public))
}

func (k *KeyPair) PrivateBytes() []byte {
	return []byte(base64.StdEncoding.EncodeToString(k.Private))
}

func (k *KeyPair) Fingerprint() string {
	hash := sha256.Sum256(k.Public)

	return base64.RawStdEncoding.EncodeToString(hash[:6])
}

func (k *KeyPair) Validate() error {
	if k == nil {
		return errors.New("keypair is nil")
	}

	if len(k.Private) != ed25519.PrivateKeySize {
		return errors.New("invalid private key length")
	}

	if len(k.Public) != ed25519.PublicKeySize {
		return errors.New("invalid public key length")
	}

	derived := k.Private.Public().(ed25519.PublicKey)
	if !derived.Equal(k.Public) {
		return errors.New("public key does not match private key")
	}

	return nil
}
