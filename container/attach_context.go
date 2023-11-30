package container

import (
	"context"
	"sync"
)

// attachContext is the context used for for attach calls.
type attachContext struct {
	mu         sync.Mutex
	ctx        context.Context
	cancelFunc context.CancelFunc
}

// init returns the context for attach calls. It creates a new context
// if no context is created yet.
func (ac *attachContext) init() context.Context {
	ac.mu.Lock()
	defer ac.mu.Unlock()
	if ac.ctx == nil {
		ac.ctx, ac.cancelFunc = context.WithCancel(context.Background())
	}
	return ac.ctx
}

// cancelFunc cancels the attachContext. All attach calls should detach
// after this call.
func (ac *attachContext) cancel() {
	ac.mu.Lock()
	if ac.ctx != nil {
		ac.cancelFunc()
		ac.ctx = nil
	}
	ac.mu.Unlock()
}
