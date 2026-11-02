package tui

import (
	"context"
	"sync"
)

// refreshCoordinator owns request cancellation and serializes filesystem work.
// Commands capture immutable workspace inputs; only Update applies results.
// A superseded command waiting for the worker lock exits before scanning.
type refreshCoordinator struct {
	mu     sync.Mutex
	worker sync.Mutex
	cancel context.CancelFunc
}

func (r *refreshCoordinator) request(parent context.Context) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	return ctx
}

func (r *refreshCoordinator) stop() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
		r.cancel = nil
	}
}
