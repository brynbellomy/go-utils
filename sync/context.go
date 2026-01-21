package bsync

import (
	"context"
	"reflect"
	"time"
)

// ContextFromChan creates a context that finishes when the provided channel
// receives or is closed. The caller must ensure either chCancel is eventually
// closed or the returned cancel function is called to prevent goroutine leaks.
func ContextFromChan(chCancel <-chan struct{}) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		select {
		case <-chCancel:
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

type ChanContext chan struct{}

var _ context.Context = ChanContext(nil)

func (ch ChanContext) Deadline() (deadline time.Time, ok bool) {
	return time.Time{}, false
}

func (ch ChanContext) Done() <-chan struct{} {
	return ch
}

func (ch ChanContext) Err() error {
	select {
	case <-ch:
		return context.Canceled
	default:
		return nil
	}
}

func (ch ChanContext) Value(key any) any {
	return nil
}

// CombinedContext creates a context that finishes when any of the provided
// signals finish.  A signal can be a `context.Context`, a `chan struct{}`, or
// a `time.Duration` (which is transformed into a `context.WithTimeout`).
func CombinedContext(signals ...any) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	if len(signals) == 0 {
		return ctx, cancel
	}
	signals = append(signals, ctx)

	var cases []reflect.SelectCase
	var otherCancels []context.CancelFunc
	for _, signal := range signals {
		var ch reflect.Value

		switch sig := signal.(type) {
		case context.Context:
			ch = reflect.ValueOf(sig.Done())
		case <-chan struct{}:
			ch = reflect.ValueOf(sig)
		case chan struct{}:
			ch = reflect.ValueOf(sig)
		case time.Duration:
			ctxTimeout, cancelTimeout := context.WithTimeout(ctx, sig)
			ch = reflect.ValueOf(ctxTimeout.Done())
			otherCancels = append(otherCancels, cancelTimeout)
		default:
			panic("invariant violation")
		}
		cases = append(cases, reflect.SelectCase{Chan: ch, Dir: reflect.SelectRecv})
	}

	go func() {
		defer cancel()
		for _, c := range otherCancels {
			defer c()
		}
		_, _, _ = reflect.Select(cases)
	}()

	return ctx, cancel
}
