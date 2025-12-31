# Herald

**Herald** is a lightweight, secure peer-to-peer (P2P) messaging library for Go. It allows services or clients to form a network of peers that can exchange messages securely, verify identities via digital signatures, and track each other's online status.

---

## Features

* **Secure Messaging**: All messages are signed using Ed25519 keys and can be verified for authenticity.
* **Peer Registry**: Track online/offline status of peers and monitor network health.
* **Automatic Handshake**: Discover and register new peers via public key exchange.
* **Heartbeat System**: Periodically monitor peer availability through heartbeat messages.
* **Flexible Middleware**: Add custom logic such as logging, message filtering, or additional validations.
* **Pluggable Transport**: Initial implementation uses RabbitMQ, with support for other transport layers.
* **Canonical JSON**: Ensures consistent message encoding for signing and verification.

---

## Getting Started

### Installation

```bash
go get github.com/abolfazlalz/herald
```

### Basic Usage

```go
package main

import (
    "context"
    "log"

    "github.com/abolfazlalz/herald"
    "github.com/abolfazlalz/herald/internal/security"
)

func main() {
    transport, err := herald.NewRabbitMQ("amqp://guest:guest@localhost:5672/", "events")
    if err != nil {
        log.Fatalf("failed to create transport: %v", err)
    }
    defer transport.Close()

    kp, err := security.Generate()
    if err != nil {
        log.Fatalf("failed to generate keys: %v", err)
    }

    h := herald.New(transport, kp.Private)

    ctx := context.Background()
    if err := h.Start(ctx); err != nil {
        log.Fatalf("failed to start herald: %v", err)
    }
}
```

---

## Use Cases

* Distributed systems requiring secure P2P notifications
* Microservices exchanging verified messages
* IoT networks for device status and real-time control

---

## Contributing

Contributions are welcome! Please follow Go best practices and maintain clean, modular code. Open an issue or a pull request for any bug fixes or enhancements.

---

## License

MIT License © 2025 Abolfazl Alizadeh
