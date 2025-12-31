package herald

import (
	"context"
	"sync"
)

type MessageContext struct {
	context.Context
	mu    sync.Mutex
	abort bool
}

func NewMessageContext(ctx context.Context) *MessageContext {
	return &MessageContext{Context: ctx}
}

func (ctx *MessageContext) Abort() {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	ctx.abort = true
}

func (ctx *MessageContext) IsAborted() bool {
	ctx.mu.Lock()
	defer ctx.mu.Unlock()
	return ctx.abort
}
