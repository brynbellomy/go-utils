package bsync

import (
	"sync"
	"sync/atomic"
)

type Mailbox[T any] struct {
	mu       sync.Mutex
	chNotify chan struct{}
	queueLen atomic.Int64 // atomic so monitor can read w/o blocking the queue

	// Ring buffer, oldest-first. `start` is the index of the oldest element,
	// `count` is the number of buffered elements. The i-th oldest element
	// lives at buf[(start+i) % len(buf)].
	buf   []T
	start int
	count int

	// capacity - number of items the mailbox can buffer (0 = unbounded)
	// NOTE: if the capacity is 1, it's possible that an empty Retrieve may occur after a notification.
	capacity uint64

	// initialCap is the ring buffer's starting size. A drained mailbox whose
	// buffer has grown far beyond this shrinks back down so a one-off burst
	// doesn't pin memory forever.
	initialCap int
}

// Creates a new mailbox instance. If name is non-empty, it must be unique and calling Start will launch
// prometheus metric monitor that periodically reports mailbox load until Close() is called.
func NewMailbox[T any](capacity uint64) *Mailbox[T] {
	queueCap := capacity
	if queueCap == 0 {
		queueCap = 100
	}
	return &Mailbox[T]{
		chNotify:   make(chan struct{}, 1),
		buf:        make([]T, queueCap),
		capacity:   capacity,
		initialCap: int(queueCap),
	}
}

// Notify returns the contents of the notify channel
func (m *Mailbox[T]) Notify() <-chan struct{} {
	return m.chNotify
}

func (m *Mailbox[T]) load() (capacity uint64, loadPercent float64) {
	capacity = m.capacity
	loadPercent = 100 * float64(m.queueLen.Load()) / float64(capacity)
	return
}

// Deliver appends to the queue and returns true if the queue was full, causing a message to be dropped.
// Amortized O(1): a bounded mailbox never allocates once at capacity; an
// unbounded one only reallocates on doubling growth.
func (m *Mailbox[T]) Deliver(x T) (wasOverCapacity bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.capacity > 0 && uint64(m.count) == m.capacity {
		// At capacity: drop the oldest element to make room.
		var zero T
		m.buf[m.start] = zero
		m.start = (m.start + 1) % len(m.buf)
		m.count--
		wasOverCapacity = true
	}
	if m.count == len(m.buf) {
		m.growLocked()
	}
	m.buf[(m.start+m.count)%len(m.buf)] = x
	m.count++
	if !wasOverCapacity {
		m.queueLen.Add(1)
	}

	select {
	case m.chNotify <- struct{}{}:
	default:
	}
	return
}

// Retrieve fetches one element from the queue.
func (m *Mailbox[T]) Retrieve() (t T, ok bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.count == 0 {
		return
	}
	t = m.buf[m.start]
	var zero T
	m.buf[m.start] = zero // don't pin references to retrieved elements
	m.start = (m.start + 1) % len(m.buf)
	m.count--
	m.queueLen.Add(-1)
	ok = true
	return
}

// RetrieveAll fetches all elements from the queue.
func (m *Mailbox[T]) RetrieveAll() []T {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.count == 0 {
		return nil
	}
	out := make([]T, m.count)
	n := copy(out, m.buf[m.start:min(m.start+m.count, len(m.buf))])
	copy(out[n:], m.buf[:m.count-n])
	m.clearLocked()
	return out
}

// RetrieveLatestAndClear fetch the latest value (or nil), and clears the rest of the queue (if any).
func (m *Mailbox[T]) RetrieveLatestAndClear() (t T) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.count == 0 {
		return
	}
	t = m.buf[(m.start+m.count-1)%len(m.buf)]
	m.clearLocked()
	return
}

// growLocked doubles the ring buffer, unwrapping the contents to the front of
// the new buffer. Caller must hold m.mu.
func (m *Mailbox[T]) growLocked() {
	newBuf := make([]T, max(2*len(m.buf), 1))
	n := copy(newBuf, m.buf[m.start:])
	copy(newBuf[n:], m.buf[:m.start])
	m.buf = newBuf
	m.start = 0
}

// clearLocked empties the queue, releasing any references held in the buffer.
// If the buffer has grown well beyond its initial size, it is shrunk back so
// a one-off burst doesn't pin memory indefinitely. Caller must hold m.mu.
func (m *Mailbox[T]) clearLocked() {
	if len(m.buf) > 4*m.initialCap {
		m.buf = make([]T, m.initialCap)
	} else if m.start+m.count <= len(m.buf) {
		clear(m.buf[m.start : m.start+m.count])
	} else {
		clear(m.buf[m.start:])
		clear(m.buf[:(m.start+m.count)%len(m.buf)])
	}
	m.start = 0
	m.count = 0
	m.queueLen.Store(0)
}
