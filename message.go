package herald

type MessageType string

const (
	MessageTypeAnnounce MessageType = "announce"
	MessageTypeMessage  MessageType = "message"
)

type Message struct {
	ID      string
	From    string
	To      string
	Type    MessageType
	Payload map[string]any
}
