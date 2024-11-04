package utils

import (
	"sync"
)

type Signal[T any] struct {
	*Mailbox[T]
	latest T
	mu     *sync.RWMutex
}

func NewSignal[T any](capacity uint64) *Signal[T] {
	return &Signal[T]{
		Mailbox: NewMailbox[T](capacity),
		mu:      &sync.RWMutex{},
	}
}

func (s *Signal[T]) Deliver(x T) (wasOverCapacity bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.latest = x
	return s.Mailbox.Deliver(x)
}

func (s *Signal[T]) Latest() T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.latest
}
