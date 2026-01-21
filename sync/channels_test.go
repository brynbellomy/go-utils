package bsync

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCollectChan_CollectsNItems tests collecting exactly n items from a channel
func TestCollectChan_CollectsNItems(t *testing.T) {
	ctx := context.Background()
	ch := make(chan int)

	go func() {
		for i := 1; i <= 10; i++ {
			ch <- i
		}
		close(ch)
	}()

	items := CollectChan(ctx, 5, ch)
	assert.Equal(t, 5, len(items))
	assert.Equal(t, []int{1, 2, 3, 4, 5}, items)
}

// TestCollectChan_ChannelClosesEarly tests when channel closes before n items
func TestCollectChan_ChannelClosesEarly(t *testing.T) {
	ctx := context.Background()
	ch := make(chan int)

	go func() {
		for i := 1; i <= 3; i++ {
			ch <- i
		}
		close(ch)
	}()

	items := CollectChan(ctx, 10, ch)
	assert.Equal(t, 3, len(items))
	assert.Equal(t, []int{1, 2, 3}, items)
}

// TestCollectChan_ContextCancelled tests when context is cancelled before n items
func TestCollectChan_ContextCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan int)

	go func() {
		ch <- 1
		ch <- 2
		time.Sleep(10 * time.Millisecond)
		ch <- 3
		ch <- 4
	}()

	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	items := CollectChan(ctx, 10, ch)
	// Should collect 2 items before context is cancelled
	assert.LessOrEqual(t, len(items), 2)
	assert.Greater(t, len(items), 0)
}

// TestCollectChan_ZeroItems tests collecting 0 items
func TestCollectChan_ZeroItems(t *testing.T) {
	ctx := context.Background()
	ch := make(chan int)

	go func() {
		ch <- 1
		ch <- 2
		close(ch)
	}()

	items := CollectChan(ctx, 0, ch)
	assert.Equal(t, 0, len(items))
}

// TestCollectChan_EmptyChannel tests with an immediately closed channel
func TestCollectChan_EmptyChannel(t *testing.T) {
	ctx := context.Background()
	ch := make(chan int)
	close(ch)

	items := CollectChan(ctx, 5, ch)
	assert.Equal(t, 0, len(items))
}

// TestCollectChan_ContextAlreadyCancelled tests with a pre-cancelled context
func TestCollectChan_ContextAlreadyCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	ch := make(chan int)
	go func() {
		for i := 1; i <= 10; i++ {
			ch <- i
		}
		close(ch)
	}()

	items := CollectChan(ctx, 5, ch)
	assert.Equal(t, 0, len(items))
}

// TestCollectChan_WithStrings tests with string type
func TestCollectChan_WithStrings(t *testing.T) {
	ctx := context.Background()
	ch := make(chan string)

	go func() {
		ch <- "hello"
		ch <- "world"
		ch <- "foo"
		close(ch)
	}()

	items := CollectChan(ctx, 2, ch)
	assert.Equal(t, 2, len(items))
	assert.Equal(t, []string{"hello", "world"}, items)
}

// TestCollectChan_WithStructs tests with struct type
func TestCollectChan_WithStructs(t *testing.T) {
	type testStruct struct {
		ID   int
		Name string
	}

	ctx := context.Background()
	ch := make(chan testStruct)

	go func() {
		ch <- testStruct{1, "first"}
		ch <- testStruct{2, "second"}
		ch <- testStruct{3, "third"}
		close(ch)
	}()

	items := CollectChan(ctx, 3, ch)
	require.Equal(t, 3, len(items))
	assert.Equal(t, 1, items[0].ID)
	assert.Equal(t, "first", items[0].Name)
	assert.Equal(t, 2, items[1].ID)
	assert.Equal(t, "second", items[1].Name)
}

// TestCollectChan_SlowProducer tests with slow producer
func TestCollectChan_SlowProducer(t *testing.T) {
	ctx := context.Background()
	ch := make(chan int)

	go func() {
		for i := 1; i <= 3; i++ {
			time.Sleep(10 * time.Millisecond)
			ch <- i
		}
		close(ch)
	}()

	start := time.Now()
	items := CollectChan(ctx, 3, ch)
	elapsed := time.Since(start)

	assert.Equal(t, 3, len(items))
	assert.Equal(t, []int{1, 2, 3}, items)
	assert.GreaterOrEqual(t, elapsed, 30*time.Millisecond)
}

// TestWaitGroupChan_CompletesWhenWaitGroupDone tests basic functionality
func TestWaitGroupChan_CompletesWhenWaitGroupDone(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)

	ch := WaitGroupChan(&wg)

	// Channel should not be closed initially
	select {
	case <-ch:
		t.Fatal("channel should not be closed yet")
	default:
	}

	// Complete the wait group
	wg.Done()
	wg.Done()

	// Channel should be closed now
	select {
	case <-ch:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("channel should be closed after WaitGroup is done")
	}
}

// TestWaitGroupChan_AlreadyDone tests with a WaitGroup that's already done
func TestWaitGroupChan_AlreadyDone(t *testing.T) {
	var wg sync.WaitGroup
	// Don't add anything, WaitGroup is already done

	ch := WaitGroupChan(&wg)

	// Channel should be closed immediately
	select {
	case <-ch:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("channel should be closed immediately for completed WaitGroup")
	}
}

// TestWaitGroupChan_WithGoroutines tests with actual goroutines
func TestWaitGroupChan_WithGoroutines(t *testing.T) {
	var wg sync.WaitGroup
	counter := 0
	var mu sync.Mutex

	wg.Add(3)

	for range 3 {
		go func() {
			defer wg.Done()
			time.Sleep(20 * time.Millisecond)
			mu.Lock()
			counter++
			mu.Unlock()
		}()
	}

	ch := WaitGroupChan(&wg)

	// Wait for completion
	<-ch

	mu.Lock()
	defer mu.Unlock()
	assert.Equal(t, 3, counter)
}

// TestWaitGroupChan_InSelect tests using WaitGroupChan in a select statement
func TestWaitGroupChan_InSelect(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		time.Sleep(50 * time.Millisecond)
		wg.Done()
	}()

	ch := WaitGroupChan(&wg)

	select {
	case <-ch:
		// Expected
	case <-time.After(200 * time.Millisecond):
		t.Fatal("WaitGroup should complete before timeout")
	}
}

// TestWaitGroupChan_WithContext tests combining WaitGroupChan with context
func TestWaitGroupChan_WithContext(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// Don't call wg.Done() - let it timeout
	ch := WaitGroupChan(&wg)

	select {
	case <-ch:
		t.Fatal("WaitGroup should not complete")
	case <-ctx.Done():
		// Expected - context timeout
	}
}

// TestWaitGroupChan_MultipleReads tests that multiple readers can read from the channel
func TestWaitGroupChan_MultipleReads(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	ch := WaitGroupChan(&wg)

	done1 := false
	done2 := false

	go func() {
		<-ch
		done1 = true
	}()

	go func() {
		<-ch
		done2 = true
	}()

	time.Sleep(10 * time.Millisecond)
	wg.Done()
	time.Sleep(50 * time.Millisecond)

	assert.True(t, done1)
	assert.True(t, done2)
}

// TestWaitGroupChan_ChannelClosedOnce tests that channel is only closed once
func TestWaitGroupChan_ChannelClosedOnce(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	ch := WaitGroupChan(&wg)
	wg.Done()

	// First read
	<-ch

	// Second read should also work (closed channel returns zero value immediately)
	select {
	case <-ch:
		// Expected
	default:
		t.Fatal("should be able to read from closed channel")
	}
}

// TestCollectChan_BufferedChannel tests with a buffered channel
func TestCollectChan_BufferedChannel(t *testing.T) {
	ctx := context.Background()
	ch := make(chan int, 10)

	// Pre-fill the buffer
	for i := 1; i <= 5; i++ {
		ch <- i
	}
	close(ch)

	items := CollectChan(ctx, 3, ch)
	assert.Equal(t, 3, len(items))
	assert.Equal(t, []int{1, 2, 3}, items)
}

// TestCollectChan_ConcurrentReads tests that only one reader gets the items
func TestCollectChan_ConcurrentReads(t *testing.T) {
	ctx := context.Background()
	ch := make(chan int, 10)

	for i := 1; i <= 10; i++ {
		ch <- i
	}
	close(ch)

	items1 := CollectChan(ctx, 5, ch)
	items2 := CollectChan(ctx, 5, ch)

	assert.Equal(t, 5, len(items1))
	assert.Equal(t, 5, len(items2))
	// Together they should have collected all 10 items
	assert.Equal(t, 10, len(items1)+len(items2))
}
