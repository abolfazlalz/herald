package message

import (
	"time"

	"github.com/abolfazlalz/herald/internal/security"
	"github.com/google/uuid"
)

type EventType string

const (
	EventAnnounce  EventType = "announce"
	EventHeartbeat EventType = "heartbeat"
	EventMessage   EventType = "message"
	EventOffline   EventType = "offline"
	EventAck       EventType = "ack"
)

// Envelope structure
type Envelope struct {
	CorrelationID string    `json:"id"`
	Version       int       `json:"version"`
	Type          EventType `json:"type"`
	SenderID      string    `json:"sender_id"`
	ReceiverID    string    `json:"receiver_id,omitempty"`
	Timestamp     int64     `json:"timestamp"`
	Payload       []byte    `json:"payload"`
	Signature     []byte    `json:"signature,omitempty"`
}

// NewEnvelope creates a new Envelope
func NewEnvelope(eventType EventType, senderID string, receiverID string, payload []byte) *Envelope {
	return &Envelope{
		CorrelationID: uuid.New().String(),
		Version:       1,
		Type:          eventType,
		SenderID:      senderID,
		ReceiverID:    receiverID,
		Timestamp:     time.Now().Unix(),
		Payload:       payload,
	}
}

// Sign the envelope using a signer
func (e *Envelope) Sign(signer security.Signer) error {
	// canonicalize payload + metadata without signature
	data, err := Canonicalize(map[string]any{
		"version":   e.Version,
		"type":      e.Type,
		"sender_id": e.SenderID,
		"timestamp": e.Timestamp,
		"payload":   e.Payload,
	})
	if err != nil {
		return err
	}

	sig, err := signer.Sign(data)
	if err != nil {
		return err
	}

	e.Signature = sig
	return nil
}

// Verify the envelope using a verifier and sender's public key
func (e *Envelope) Verify(verifier security.Verifier, pubKey []byte) error {
	// canonicalize same fields
	data, err := Canonicalize(map[string]any{
		"version":   e.Version,
		"type":      e.Type,
		"sender_id": e.SenderID,
		"timestamp": e.Timestamp,
		"payload":   e.Payload,
	})
	if err != nil {
		return err
	}

	return verifier.Verify(data, e.Signature, pubKey)
}
