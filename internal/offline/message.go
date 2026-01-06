package offline

import (
	"encoding/json"

	"github.com/abolfazlalz/herald/internal/message"
)

type OfflinePayload struct {
	Reason string
}

func InitOffline(senderID, reason string) (*message.Envelope, error) {
	payload, err := json.Marshal(OfflinePayload{Reason: reason})
	if err != nil {
		return nil, err
	}

	return message.NewEnvelope(message.EventOffline, senderID, "", payload), nil
}
