package acknowledge

import (
	"testing"

	"github.com/abolfazlalz/herald/internal/message"
)

func TestInitiateAcknowledgeBuildsAckEnvelope(t *testing.T) {
	env, err := InitiateAcknowledge("service-a", "service-b", "msg-1", "OK")
	if err != nil {
		t.Fatalf("InitiateAcknowledge returned error: %v", err)
	}
	if env.Type != message.EventAck {
		t.Fatalf("env.Type = %q, want %q", env.Type, message.EventAck)
	}
	if env.SenderID != "service-a" {
		t.Fatalf("env.SenderID = %q, want service-a", env.SenderID)
	}
	if env.ReceiverID != "service-b" {
		t.Fatalf("env.ReceiverID = %q, want service-b", env.ReceiverID)
	}

	payload, err := FromPayload(env.Payload)
	if err != nil {
		t.Fatalf("FromPayload returned error: %v", err)
	}
	if payload.Type != "acknowledge" || payload.Service != "service-a" || payload.AckTo != "msg-1" || payload.Status != "OK" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}

func TestFromPayloadRejectsInvalidJSON(t *testing.T) {
	if _, err := FromPayload([]byte("{")); err == nil {
		t.Fatal("FromPayload accepted invalid JSON")
	}
}
