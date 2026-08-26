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

	// initialCap is the ring buffer's starting size and its shrink target. A
	// mailbox whose buffer has grown far beyond this shrinks back down once
	// drained (or mostly drained) so a one-off burst doesn't pin memory
	// forever.
	initialCap int
}

// defaultInitialCap is the ring buffer's starting size when the capacity does
// not impose a smaller one. Bounded mailboxes do NOT allocate their full
// capacity up front — the buffer starts here and grows by doubling toward
// capacity only under load.
const defaultInitialCap = 100

// NewMailbox creates a mailbox that buffers up to capacity items, dropping the
// oldest on overflow. Capacity 0 means unbounded.
func NewMailbox[T any](capacity uint64) *Mailbox[T] {
	initialCap := uint64(defaultInitialCap)
	if capacity > 0 && capacity < initialCap {
		initialCap = capacity
	}
	return &Mailbox[T]{
		chNotify:   make(chan struct{}, 1),
		buf:        make([]T, initialCap),
		capacity:   capacity,
		initialCap: int(initialCap),
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
		newSize := max(2*len(m.buf), 1)
		if m.capacity > 0 {
			// A bounded buffer never grows past capacity. The at-capacity
			// branch above guarantees count < capacity here, so the capped
			// size still has room for this insert.
			newSize = min(newSize, int(m.capacity))
		}
		m.resizeLocked(newSize)
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
	// Shrink a ring that grew for a long-since-drained burst: Retrieve-only
	// consumers never pass through clearLocked, so without this the slot
	// array stays pinned at its high-water mark forever. The 1/4-occupancy
	// trigger with a 2x-count target keeps resizes amortized O(1).
	if len(m.buf) > 4*m.initialCap && m.count < len(m.buf)/4 {
		m.resizeLocked(max(2*m.count, m.initialCap))
	}
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

// resizeLocked replaces the ring buffer with one of newSize slots (which must
// be >= m.count), unwrapping the contents to the front of the new buffer.
// Caller must hold m.mu.
func (m *Mailbox[T]) resizeLocked(newSize int) {
	newBuf := make([]T, newSize)
	n := copy(newBuf, m.buf[m.start:min(m.start+m.count, len(m.buf))])
	copy(newBuf[n:], m.buf[:m.count-n])
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
