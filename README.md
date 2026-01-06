# Herald Library Documentation

# Overview

**Herald** is a Go-based peer-to-peer messaging library designed to facilitate secure, reliable, and decentralized communication between peers. It supports direct peer-to-peer messaging, broadcast messaging, heartbeats for online status, and peer lifecycle hooks.

The library uses **Ed25519 cryptography** for signing and verifying messages, and supports pluggable transport layers (such as RabbitMQ).

# Installation

```bash
go get github.com/abolfazlalz/herald
```

# Core Concepts

## Peer

A peer is any instance of Herald running in the network. Peers have:

- A unique ID (`string`)
- Public key
- Optional routing key (for transport)

## Message Types

Herald defines several types of messages:

- `MessageTypeAnnounce`: Announces a new peer.
- `MessageTypeMessage`: Standard message between peers.
- `MessageTypeACK`: Acknowledgment message.
- `MessageTypeOffline`: Indicates that a peer has gone offline. Other peers should update their registry/state accordingly.

## Envelope

All messages are wrapped in an `Envelope`:

```go
type Envelope struct {
    ID         string
    Version    int
    Type       EventType
    SenderID   string
    ReceiverID string
    Timestamp  int64
    Payload    []byte
    Signature  []byte
}
```

Envelopes are signed and verified using Ed25519 for authenticity.

# API

## Creating a Herald Instance

```go
import "github.com/abolfazlalz/herald/transport"

transport, _ := transport.NewRabbitMQ("amqp://guest:guest@localhost:5672/", "myQueue")
privateKey := []byte{/* your 64-byte Ed25519 private key */}
h := herald.New(transport, privateKey)
```

## Starting the Library

```go
ctx := context.Background()
err := h.Start(ctx)
if err != nil {
    log.Fatal(err)
}
```

This initializes the peer, starts heartbeats, listens for messages, and manages the peer lifecycle.

## Sending Messages

### Send to a specific peer

```go
h.SendToPeer(ctx, "peer-id", []byte("Hello!"))
```

### Broadcast message

```go
h.Broadcast(ctx, []byte("Hello everyone!"))
```

### Send and wait for ACK

```go
err := h.SendAndWait(ctx, "peer-id", payload, 5*time.Second)
if err != nil {
    log.Println("Failed to receive ack:", err)
}
```

## Subscribing to Messages

```go
sub := h.Subscribe(ctx, herald.MessageTypeMessage, 10)
for msg := range sub.C {
    fmt.Println("Received message:", msg.Payload)
}
```

## Hooks for Peer Lifecycle

```go
h.OnPeerJoin(func(ctx context.Context, peerID string) {
    fmt.Println(peerID, "joined")
})

h.OnPeerLeave(func(ctx context.Context, peerID string) {
    fmt.Println(peerID, "left")
})
```

## Closing a Peer

```go
h.Close(ctx)
```

This sends an offline message to peers and closes all channels.

# Middleware

Herald supports middleware for processing incoming messages:

- `VerifySignature()`: Checks signature of message.
- `UpdateLastOnline()`: Updates peer's last online timestamp.
- `CheckMessageAccess()`: Aborts processing if the message is not addressed to this peer.

# Transport Interface

Herald uses a pluggable `Transport` interface:

```go
type Transport interface {
    PublishBroadcast(ctx context.Context, data []byte) error
    PublishDirect(ctx context.Context, routingKey string, data []byte) error
    SubscribeBroadcast(ctx context.Context) (<-chan []byte, error)
    SubscribeDirect(ctx context.Context, peerID string) (<-chan []byte, error)
    Close() error
}
```

RabbitMQ is provided as a reference implementation (`transport/rabbitmq.go`).

# Security

Herald uses **Ed25519**:

- `KeyPair`: represents public/private keys.
- `Signer`: signs messages.
- `Verifier`: verifies messages.

# Heartbeat

Herald periodically sends heartbeat messages to indicate online status.

# Peer Registry

Manages the list of known peers and their last online timestamp.

# Example

```go
ctx := context.Background()
transport, _ := transport.NewRabbitMQ("amqp://guest:guest@localhost:5672/", "myQueue")
h, _ := herald.New(transport)

h.OnPeerJoin(func(ctx context.Context, id string){ fmt.Println("Peer joined:", id) })

go h.Start(ctx)

h.Broadcast(ctx, []byte("Hello world!"))
```

# License

MIT License
