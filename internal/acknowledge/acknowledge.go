package acknowledge

import (
	"encoding/json"

	"github.com/abolfazlalz/herald/internal/message"
)

type AcknowledgePayload struct {
	Type    string `json:"type"`
	Service string `json:"service"`
	AckTo   string `json:"ack_to"`
	Status  string `json:"status"`
}

func InitiateAcknowledge(selfID, toID, ackTo, status string) (*message.Envelope, error) {
	payload, err := json.Marshal(AcknowledgePayload{
		Type:    "acknowledge",
		Service: selfID,
		AckTo:   ackTo,
		Status:  status,
	})
	if err != nil {
		return nil, err
	}

	return message.NewEnvelope(message.EventAck, selfID, toID, payload), nil
}

func FromPayload(payload []byte) (*AcknowledgePayload, error) {
	var ackPayload AcknowledgePayload
	if err := json.Unmarshal(payload, &ackPayload); err != nil {
		return nil, err
	}
	return &ackPayload, nil
}
