package bsync

import (
	"context"
	"sync"
)

// CollectChan collects up to n items from a channel, stopping early if the channel closes
// or the context is cancelled. It returns a slice containing the items collected.
func CollectChan[T any](ctx context.Context, n int, ch <-chan T) []T {
	items := make([]T, 0, n)
	for range n {
		select {
		case item, open := <-ch:
			if !open {
				return items
			}
			items = append(items, item)
		case <-ctx.Done():
			return items
		}
	}
	return items
}

// WaitGroupChan returns a channel that closes when the WaitGroup completes.
// The returned channel can be used in select statements to wait for the WaitGroup
// alongside other channel operations.
//
// Example usage:
//
//	var wg sync.WaitGroup
//	wg.Add(2)
//	go func() { defer wg.Done(); /* work */ }()
//	go func() { defer wg.Done(); /* work */ }()
//
//	select {
//	case <-WaitGroupChan(&wg):
//	    // WaitGroup is done
//	case <-time.After(timeout):
//	    // Timeout
//	case <-ctx.Done():
//	    // Context cancelled
//	}
func WaitGroupChan(wg *sync.WaitGroup) <-chan struct{} {
	ch := make(chan struct{})
	go func() {
		wg.Wait()
		close(ch)
	}()
	return ch
}
