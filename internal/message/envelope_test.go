package message

import (
	"testing"

	"github.com/abolfazlalz/herald/internal/security"
)

func TestEnvelopeSignAndVerify(t *testing.T) {
	kp, err := security.Generate()
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	signer, err := security.NewSigner(kp)
	if err != nil {
		t.Fatalf("NewSigner returned error: %v", err)
	}
	verifier := security.NewVerifier()

	env := NewEnvelope(EventMessage, "sender", "receiver", []byte("hello"))
	if err := env.Sign(signer); err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	if len(env.Signature) == 0 {
		t.Fatal("Sign left envelope signature empty")
	}
	if err := env.Verify(verifier, kp.Public); err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
}

func TestEnvelopeVerifyFailsAfterTampering(t *testing.T) {
	kp, err := security.Generate()
	if err != nil {
		t.Fatalf("Generate returned error: %v", err)
	}
	signer, err := security.NewSigner(kp)
	if err != nil {
		t.Fatalf("NewSigner returned error: %v", err)
	}

	env := NewEnvelope(EventMessage, "sender", "receiver", []byte("hello"))
	if err := env.Sign(signer); err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}

	env.Payload = []byte("goodbye")
	if err := env.Verify(security.NewVerifier(), kp.Public); err == nil {
		t.Fatal("Verify returned nil error after payload tampering")
	}
}
