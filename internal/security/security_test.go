package security

import (
	"crypto/ed25519"
	"encoding/base64"
	"testing"
)

func TestGenerateCreatesValidKeyPair(t *testing.T) {
	kp, err := Generate()
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	if err := kp.Validate(); err != nil {
		t.Fatalf("generated keypair did not validate: %v", err)
	}
	if len(kp.PublicBytes()) == 0 {
		t.Fatal("PublicBytes returned empty data")
	}
	if _, err := base64.StdEncoding.DecodeString(string(kp.PublicBytes())); err != nil {
		t.Fatalf("PublicBytes returned invalid base64: %v", err)
	}
	if kp.Fingerprint() == "" {
		t.Fatal("Fingerprint returned empty string")
	}
}

func TestLoadFromBytesRoundTripsPrivateKey(t *testing.T) {
	kp, err := Generate()
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}

	loaded, err := LoadFromBytes(kp.Private)
	if err != nil {
		t.Fatalf("LoadFromBytes returned error: %v", err)
	}
	if string(loaded.Public) != string(kp.Public) {
		t.Fatal("loaded public key does not match original public key")
	}
}

func TestSignerAndVerifier(t *testing.T) {
	kp, err := Generate()
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	signer, err := NewSigner(kp)
	if err != nil {
		t.Fatalf("NewSigner returned error: %v", err)
	}

	data := []byte("message")
	signature, err := signer.Sign(data)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	if len(signature) != ed25519.SignatureSize {
		t.Fatalf("signature length = %d, want %d", len(signature), ed25519.SignatureSize)
	}
	if err := NewVerifier().Verify(data, signature, kp.Public); err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
}

func TestSignerRejectsInvalidInput(t *testing.T) {
	if _, err := NewSigner(nil); err == nil {
		t.Fatal("NewSigner(nil) returned nil error")
	}

	var signer *Ed25519Signer
	if _, err := signer.Sign([]byte("message")); err == nil {
		t.Fatal("nil signer returned nil error")
	}

	kp, err := Generate()
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	signer, err = NewSigner(kp)
	if err != nil {
		t.Fatalf("NewSigner returned error: %v", err)
	}
	if _, err := signer.Sign(nil); err == nil {
		t.Fatal("Sign(nil) returned nil error")
	}
}

func TestVerifierRejectsInvalidInput(t *testing.T) {
	kp, err := Generate()
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	signer, err := NewSigner(kp)
	if err != nil {
		t.Fatalf("NewSigner returned error: %v", err)
	}
	signature, err := signer.Sign([]byte("message"))
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	verifier := NewVerifier()
	if err := verifier.Verify(nil, signature, kp.Public); err == nil {
		t.Fatal("Verify accepted empty data")
	}
	if err := verifier.Verify([]byte("message"), nil, kp.Public); err == nil {
		t.Fatal("Verify accepted nil signature")
	}
	if err := verifier.Verify([]byte("message"), signature, nil); err == nil {
		t.Fatal("Verify accepted nil public key")
	}
	if err := verifier.Verify([]byte("other"), signature, kp.Public); err == nil {
		t.Fatal("Verify accepted signature for different data")
	}
}
