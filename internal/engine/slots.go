package engine

import (
	"context"
	"sync"
)

// slots is a resizable counting semaphore: the limit is read on every
// acquire, so an admin can change "max parallel generations" at runtime.
type slots struct {
	mu    sync.Mutex
	inUse int
	ch    chan struct{} // closed and replaced on every release
}

func newSlots() *slots { return &slots{ch: make(chan struct{})} }

// acquire blocks until a slot is free or ctx ends. limit() <= 0 = unlimited.
// It reports whether the caller had to wait.
func (s *slots) acquire(ctx context.Context, limit func() int) (waited bool, err error) {
	for {
		s.mu.Lock()
		lim := limit()
		if lim <= 0 || s.inUse < lim {
			s.inUse++
			s.mu.Unlock()
			return waited, nil
		}
		ch := s.ch
		s.mu.Unlock()
		waited = true
		select {
		case <-ch:
		case <-ctx.Done():
			return waited, ctx.Err()
		}
	}
}

func (s *slots) release() {
	s.mu.Lock()
	s.inUse--
	close(s.ch)
	s.ch = make(chan struct{})
	s.mu.Unlock()
}

func (s *slots) used() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inUse
}
