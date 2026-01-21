package bsync

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestContextFromChan_ChannelClosed tests that the context is canceled when the channel is closed
func TestContextFromChan_ChannelClosed(t *testing.T) {
	chCancel := make(chan struct{})
	ctx, cancel := ContextFromChan(chCancel)
	defer cancel()

	// Context should not be done initially
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done initially")
	default:
	}

	// Close the channel
	close(chCancel)

	// Context should be done now
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("context should be done after channel is closed")
	}

	assert.Error(t, ctx.Err())
}

// TestContextFromChan_ChannelReceives tests that the context is canceled when the channel receives
func TestContextFromChan_ChannelReceives(t *testing.T) {
	chCancel := make(chan struct{})
	ctx, cancel := ContextFromChan(chCancel)
	defer cancel()

	// Send to the channel
	chCancel <- struct{}{}

	// Context should be done
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("context should be done after channel receives")
	}

	assert.Error(t, ctx.Err())
}

// TestContextFromChan_CancelFunc tests that calling the cancel function cancels the context
func TestContextFromChan_CancelFunc(t *testing.T) {
	chCancel := make(chan struct{})
	ctx, cancel := ContextFromChan(chCancel)

	// Call cancel
	cancel()

	// Context should be done
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("context should be done after cancel is called")
	}

	assert.Error(t, ctx.Err())
}

// TestChanContext_Deadline tests that ChanContext returns zero deadline
func TestChanContext_Deadline(t *testing.T) {
	ch := make(ChanContext)
	deadline, ok := ch.Deadline()
	assert.False(t, ok)
	assert.True(t, deadline.IsZero())
}

// TestChanContext_Done tests that ChanContext returns the channel
func TestChanContext_Done(t *testing.T) {
	ch := make(ChanContext)
	done := ch.Done()
	assert.Equal(t, (<-chan struct{})(ch), done)
}

// TestChanContext_Err_NotClosed tests that Err returns nil when channel is not closed
func TestChanContext_Err_NotClosed(t *testing.T) {
	ch := make(ChanContext)
	err := ch.Err()
	assert.Nil(t, err)
}

// TestChanContext_Err_Closed tests that Err returns context.Canceled when channel is closed
func TestChanContext_Err_Closed(t *testing.T) {
	ch := make(ChanContext)
	close(ch)
	err := ch.Err()
	assert.Equal(t, context.Canceled, err)
}

// TestChanContext_Value tests that Value always returns nil
func TestChanContext_Value(t *testing.T) {
	ch := make(ChanContext)
	val := ch.Value("some-key")
	assert.Nil(t, val)
}

// TestCombinedContext_NoSignals tests CombinedContext with no signals
func TestCombinedContext_NoSignals(t *testing.T) {
	ctx, cancel := CombinedContext()
	defer cancel()

	// Should not be done
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done without signals")
	default:
	}
}

// TestCombinedContext_SingleContext tests with a single context signal
func TestCombinedContext_SingleContext(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx, cancel := CombinedContext(ctx1)
	defer cancel()

	// Cancel the input context
	cancel1()

	// Combined context should be done
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("combined context should be done when input context is canceled")
	}
}

// TestCombinedContext_MultipleContexts tests with multiple context signals
func TestCombinedContext_MultipleContexts(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	ctx, cancel := CombinedContext(ctx1, ctx2)
	defer cancel()

	// Cancel the first context
	cancel1()

	// Combined context should be done
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("combined context should be done when any input context is canceled")
	}
}

// TestCombinedContext_Channel tests with a channel signal
func TestCombinedContext_Channel(t *testing.T) {
	ch := make(chan struct{})
	ctx, cancel := CombinedContext(ch)
	defer cancel()

	// Close the channel
	close(ch)

	// Combined context should be done
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("combined context should be done when channel is closed")
	}
}

// TestCombinedContext_ReceiveOnlyChannel tests with a receive-only channel signal
func TestCombinedContext_ReceiveOnlyChannel(t *testing.T) {
	ch := make(chan struct{})
	var chRecvOnly <-chan struct{} = ch
	ctx, cancel := CombinedContext(chRecvOnly)
	defer cancel()

	// Close the channel
	close(ch)

	// Combined context should be done
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("combined context should be done when receive-only channel is closed")
	}
}

// TestCombinedContext_Duration tests with a duration signal
func TestCombinedContext_Duration(t *testing.T) {
	ctx, cancel := CombinedContext(50 * time.Millisecond)
	defer cancel()

	// Context should not be done immediately
	select {
	case <-ctx.Done():
		t.Fatal("context should not be done immediately")
	default:
	}

	// Wait for the timeout
	time.Sleep(100 * time.Millisecond)

	// Context should be done
	select {
	case <-ctx.Done():
		// Expected
	default:
		t.Fatal("combined context should be done after duration expires")
	}
}

// TestCombinedContext_MixedSignals tests with a mix of different signal types
func TestCombinedContext_MixedSignals(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	ch := make(chan struct{})

	ctx, cancel := CombinedContext(ctx1, ch, 1*time.Second)
	defer cancel()

	// Close the channel
	close(ch)

	// Combined context should be done
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("combined context should be done when any signal triggers")
	}
}

// TestCombinedContext_CancelFunc tests that calling cancel on combined context works
func TestCombinedContext_CancelFunc(t *testing.T) {
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()

	ctx, cancel := CombinedContext(ctx1)

	// Call cancel
	cancel()

	// Context should be done
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("combined context should be done when cancel is called")
	}
}

// TestCombinedContext_InvalidSignalPanics tests that invalid signal types cause panic
func TestCombinedContext_InvalidSignalPanics(t *testing.T) {
	assert.Panics(t, func() {
		CombinedContext("invalid signal type")
	}, "should panic on invalid signal type")

	assert.Panics(t, func() {
		CombinedContext(123)
	}, "should panic on invalid signal type")

	assert.Panics(t, func() {
		CombinedContext(struct{}{})
	}, "should panic on invalid signal type")
}

// TestCombinedContext_TimeoutCleanup tests that timeout contexts are properly cleaned up
func TestCombinedContext_TimeoutCleanup(t *testing.T) {
	ctx, cancel := CombinedContext(1 * time.Second)

	// Immediately cancel
	cancel()

	// Context should be done
	select {
	case <-ctx.Done():
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("combined context should be done when cancel is called")
	}

	// Sleep a bit to ensure cleanup happens
	time.Sleep(50 * time.Millisecond)
}

// TestCombinedContext_MultipleTimeouts tests with multiple duration signals
func TestCombinedContext_MultipleTimeouts(t *testing.T) {
	ctx, cancel := CombinedContext(50*time.Millisecond, 100*time.Millisecond, 200*time.Millisecond)
	defer cancel()

	// Should complete after the shortest timeout (50ms)
	start := time.Now()
	<-ctx.Done()
	elapsed := time.Since(start)

	// Should be around 50ms, not 100ms or 200ms
	require.Less(t, elapsed, 80*time.Millisecond, "should finish after shortest timeout")
	require.Greater(t, elapsed, 40*time.Millisecond, "should not finish too early")
}

// TestCombinedContext_ContextInterface tests that the returned context implements context.Context
func TestCombinedContext_ContextInterface(t *testing.T) {
	ctx, cancel := CombinedContext()
	defer cancel()

	var _ context.Context = ctx
}
