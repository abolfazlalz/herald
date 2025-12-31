package main

import (
	"context"
	"fmt"
	"log"

	"github.com/abolfazlalz/herald"
	"github.com/abolfazlalz/herald/internal/security"
)

type Data struct {
	Key     string `json:"key"`
	Message string `json:"message"`
	UUID    string `json:"uuid"`
}

func main() {
	transport, err := herald.NewRabbitMQ("amqp://guest:guest@localhost:5672/", "events")
	if err != nil {
		log.Fatalf("error during connect to RabbitMQ: %v", err)
	}
	defer func() {
		transport.Close()
	}()

	ctx := context.Background()

	kv, err := security.Generate()
	if err != nil {
		log.Fatalf("error during generate security: %v", err)
	}

	h := herald.New(transport, kv.Private)
	fmt.Println("Cluster: ", h.ID())
	if err := h.Start(ctx); err != nil {
		log.Fatalf("error during listen to herald: %v", err)
	}
}
