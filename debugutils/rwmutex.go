package utils

import (
	"fmt"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ietxaniz/delock"
)

// LockInfo tracks information about a lock holder or waiter
type LockInfo struct {
	ID          int
	GoroutineID uint64
	Stack       string
	Timestamp   time.Time
	LockType    string // "write", "read", "write-waiting", "read-waiting"
}

// DebugRWMutex wraps delock.RWMutex with enhanced debugging
type DebugRWMutex struct {
	mu           *delock.RWMutex
	debugMu      sync.RWMutex
	holders      map[int]*LockInfo // Currently holding locks
	waiters      map[int]*LockInfo // Currently waiting for locks
	nextWaiterID int
	name         string // Optional name for the mutex
}

// NewDebugRWMutex creates a new debug-enabled RWMutex
func NewDebugRWMutex(name string) *DebugRWMutex {
	mu := &delock.RWMutex{}
	mu.SetTimeout(10 * time.Second) // Default timeout

	return &DebugRWMutex{
		mu:      mu,
		holders: make(map[int]*LockInfo),
		waiters: make(map[int]*LockInfo),
		name:    name,
	}
}

// SetTimeout sets the timeout for the underlying mutex
func (d *DebugRWMutex) SetTimeout(timeout time.Duration) {
	d.mu.SetTimeout(timeout)
}

// getGoroutineID extracts the goroutine ID from the stack trace
func getGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	stack := string(buf[:n])

	// Parse "goroutine 123 [running]:" or "goroutine 123 [chan receive]:"
	const prefix = "goroutine "
	if idx := strings.Index(stack, prefix); idx >= 0 {
		// Move past "goroutine "
		start := idx + len(prefix)
		line := stack[start:]

		// Find the space or bracket that ends the ID
		var end int
		for i, r := range line {
			if r == ' ' || r == '[' {
				end = i
				break
			}
		}

		if end > 0 {
			idStr := line[:end]
			var id uint64
			if n, err := fmt.Sscanf(idStr, "%d", &id); n == 1 && err == nil {
				return id
			}
		}
	}
	return 0
}

// getStack returns a formatted stack trace
func getStack() string {
	buf := make([]byte, 4096)
	n := runtime.Stack(buf, false)
	stack := string(buf[:n])

	// Clean up the stack trace - remove the first few frames which are internal
	lines := strings.Split(stack, "\n")
	if len(lines) > 6 {
		// Skip goroutine header, getStack, and the Lock/RLock method calls
		return strings.Join(lines[6:], "\n")
	}
	return stack
}

// Lock acquires a write lock with debugging
func (d *DebugRWMutex) Lock() (int, error) {
	gid := getGoroutineID()
	stack := getStack()

	// Record that we're waiting
	d.debugMu.Lock()
	waiterID := d.nextWaiterID
	d.nextWaiterID++
	d.waiters[waiterID] = &LockInfo{
		ID:          waiterID,
		GoroutineID: gid,
		Stack:       stack,
		Timestamp:   time.Now(),
		LockType:    "write-waiting",
	}
	d.debugMu.Unlock()

	// Try to acquire the lock
	lockID, err := d.mu.Lock()

	// Remove from waiters and update holders
	d.debugMu.Lock()
	delete(d.waiters, waiterID)

	if err != nil {
		// Lock failed - create enhanced error
		enhancedErr := d.createTimeoutError(err, "write")
		d.debugMu.Unlock()
		return 0, enhancedErr
	}

	// Lock succeeded - record as holder
	d.holders[lockID] = &LockInfo{
		ID:          lockID,
		GoroutineID: gid,
		Stack:       stack,
		Timestamp:   time.Now(),
		LockType:    "write",
	}
	d.debugMu.Unlock()

	return lockID, nil
}

// RLock acquires a read lock with debugging
func (d *DebugRWMutex) RLock() (int, error) {
	gid := getGoroutineID()
	stack := getStack()

	// Record that we're waiting
	d.debugMu.Lock()
	waiterID := d.nextWaiterID
	d.nextWaiterID++
	d.waiters[waiterID] = &LockInfo{
		ID:          waiterID,
		GoroutineID: gid,
		Stack:       stack,
		Timestamp:   time.Now(),
		LockType:    "read-waiting",
	}
	d.debugMu.Unlock()

	// Try to acquire the lock
	lockID, err := d.mu.RLock()

	// Remove from waiters and update holders
	d.debugMu.Lock()
	delete(d.waiters, waiterID)

	if err != nil {
		// Lock failed - create enhanced error
		enhancedErr := d.createTimeoutError(err, "read")
		d.debugMu.Unlock()
		return 0, enhancedErr
	}

	// Lock succeeded - record as holder
	d.holders[lockID] = &LockInfo{
		ID:          lockID,
		GoroutineID: gid,
		Stack:       stack,
		Timestamp:   time.Now(),
		LockType:    "read",
	}
	d.debugMu.Unlock()

	return lockID, nil
}

// Unlock releases a write lock
func (d *DebugRWMutex) Unlock(id int) {
	d.debugMu.Lock()
	delete(d.holders, id)
	d.mu.Unlock(id)
	d.debugMu.Unlock()
}

// RUnlock releases a read lock
func (d *DebugRWMutex) RUnlock(id int) {
	d.debugMu.Lock()
	delete(d.holders, id)
	d.mu.RUnlock(id)
	d.debugMu.Unlock()
}

// createTimeoutError creates an enhanced error with stack traces
func (d *DebugRWMutex) createTimeoutError(originalErr error, requestedLockType string) error {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Lock timeout for %s lock on mutex '%s': %v\n\n",
		requestedLockType, d.name, originalErr))

	// Add current holders
	sb.WriteString("=== CURRENT LOCK HOLDERS ===\n")
	if len(d.holders) == 0 {
		sb.WriteString("No locks currently held\n")
	} else {
		for _, holder := range d.holders {
			sb.WriteString(fmt.Sprintf("Holder ID: %d, Type: %s, Goroutine: %d, Held since: %v\n",
				holder.ID, holder.LockType, holder.GoroutineID, holder.Timestamp))
			sb.WriteString("Stack trace:\n")
			sb.WriteString(holder.Stack)
			sb.WriteString("\n---\n")
		}
	}

	// Add current waiters
	sb.WriteString("\n=== CURRENT WAITERS ===\n")
	if len(d.waiters) == 0 {
		sb.WriteString("No goroutines currently waiting\n")
	} else {
		for _, waiter := range d.waiters {
			sb.WriteString(fmt.Sprintf("Waiter ID: %d, Type: %s, Goroutine: %d, Waiting since: %v\n",
				waiter.ID, waiter.LockType, waiter.GoroutineID, waiter.Timestamp))
			sb.WriteString("Stack trace:\n")
			sb.WriteString(waiter.Stack)
			sb.WriteString("\n---\n")
		}
	}

	return fmt.Errorf(sb.String())
}

// GetDebugInfo returns current lock state for debugging
func (d *DebugRWMutex) GetDebugInfo() string {
	d.debugMu.RLock()
	defer d.debugMu.RUnlock()

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("=== DEBUG INFO for mutex '%s' ===\n", d.name))

	sb.WriteString(fmt.Sprintf("Holders: %d, Waiters: %d\n\n", len(d.holders), len(d.waiters)))

	sb.WriteString("HOLDERS:\n")
	for _, holder := range d.holders {
		sb.WriteString(fmt.Sprintf("  ID:%d Type:%s Goroutine:%d Since:%v\n",
			holder.ID, holder.LockType, holder.GoroutineID, holder.Timestamp))
	}

	sb.WriteString("\nWAITERS:\n")
	for _, waiter := range d.waiters {
		sb.WriteString(fmt.Sprintf("  ID:%d Type:%s Goroutine:%d Since:%v\n",
			waiter.ID, waiter.LockType, waiter.GoroutineID, waiter.Timestamp))
	}

	return sb.String()
}
