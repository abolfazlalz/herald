package security

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
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

func LoadFromBytes(priv []byte) (*KeyPair, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, errors.New("invalid private key size")
	}

	private := ed25519.PrivateKey(priv)
	public := private.Public().(ed25519.PublicKey)

	return &KeyPair{
		Private: private,
		Public:  public,
	}, nil
}

func (k *KeyPair) PublicBytes() []byte {
	return []byte(base64.StdEncoding.EncodeToString(k.Public))
}

func (k *KeyPair) Fingerprint() string {
	sum := sha256.Sum256(k.Public)
	return base64.StdEncoding.EncodeToString(sum[:6])
}

func (k *KeyPair) Validate() error {
	if len(k.Public) != ed25519.PublicKeySize {
		return errors.New("invalid public key")
	}
	if len(k.Private) != ed25519.PrivateKeySize {
		return errors.New("invalid private key")
	}
	return nil
}
