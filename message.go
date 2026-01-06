package herald

import "github.com/abolfazlalz/herald/internal/message"

type MessageType string

const (
	MessageTypeAnnounce MessageType = "announce"
	MessageTypeMessage  MessageType = "message"
	MessageTypeACK      MessageType = "ack"
)

func (mt MessageType) GetType() message.EventType {
	return map[MessageType]message.EventType{
		MessageTypeAnnounce: message.EventAnnounce,
		MessageTypeACK:      message.EventAck,
		MessageTypeMessage:  message.EventMessage,
	}[mt]
}

type Message struct {
	ID      string
	From    string
	To      string
	Type    MessageType
	Payload map[string]any
}
