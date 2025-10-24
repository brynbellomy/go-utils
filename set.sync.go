package utils

import (
	"sync"
)

type SyncMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

func NewSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{
		m: make(map[K]V),
	}
}

func (sm *SyncMap[K, V]) Get(key K) (V, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	val, ok := sm.m[key]
	return val, ok
}

func (sm *SyncMap[K, V]) MustGet(key K) V {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	val, ok := sm.m[key]
	if !ok {
		panic("invariant violation")
	}
	return val
}

func (sm *SyncMap[K, V]) Set(key K, value V) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.m[key] = value
}

func (sm *SyncMap[K, V]) Delete(key K) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.m, key)
}

func (sm *SyncMap[K, V]) Len() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.m)
}

func (sm *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	for k, v := range sm.m {
		if !f(k, v) {
			break
		}
	}
}

func (sm *SyncMap[K, V]) Iter() func(yield func(k K, v V) bool) {
	return func(yield func(k K, v V) bool) {
		sm.mu.RLock()
		defer sm.mu.RUnlock()
		for k, v := range sm.m {
			if !yield(k, v) {
				break
			}
		}
	}
}

type SyncSet[T comparable] struct {
	mu sync.RWMutex
	m  Set[T]
}

func NewSyncSet[T comparable]() *SyncSet[T] {
	return &SyncSet[T]{
		mu: sync.RWMutex{},
		m:  NewSet[T](),
	}
}

func (m *SyncSet[T]) Has(item T) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.m.Has(item)
}

func (m *SyncSet[T]) Add(item T) (exists bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.m.Add(item)
}

func (m *SyncSet[T]) Remove(item T) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.m.Remove(item)
}
