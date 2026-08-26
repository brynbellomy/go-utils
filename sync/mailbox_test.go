package bsync

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMailbox(t *testing.T) {
	var (
		expected  = []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
		toDeliver = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	)

	const capacity = 10
	m := NewMailbox[int](capacity)

	// Queue deliveries
	for i, d := range toDeliver {
		atCapacity := m.Deliver(d)
		if atCapacity && i < capacity {
			t.Errorf("mailbox at capacity %d", i)
		} else if !atCapacity && i >= capacity {
			t.Errorf("mailbox below capacity %d", i)
		}
	}

	// Retrieve them
	var recvd []int
	chDone := make(chan struct{})
	go func() {
		defer close(chDone)
		for range m.Notify() {
			for {
				x, exists := m.Retrieve()
				if !exists {
					break
				}
				recvd = append(recvd, x)
			}
		}
	}()

	close(m.chNotify)
	<-chDone

	require.Equal(t, expected, recvd)
}

func TestMailbox_RetrieveAll(t *testing.T) {
	var (
		expected  = []int{2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
		toDeliver = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	)

	const capacity = 10
	m := NewMailbox[int](capacity)

	// Queue deliveries
	for i, d := range toDeliver {
		atCapacity := m.Deliver(d)
		if atCapacity && i < capacity {
			t.Errorf("mailbox at capacity %d", i)
		} else if !atCapacity && i >= capacity {
			t.Errorf("mailbox below capacity %d", i)
		}
	}

	require.Equal(t, expected, m.RetrieveAll())
}

func TestMailbox_RetrieveLatestAndClear(t *testing.T) {
	var (
		expected  = 11
		toDeliver = []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}
	)

	const capacity = 10
	m := NewMailbox[int](capacity)

	// Queue deliveries
	for i, d := range toDeliver {
		atCapacity := m.Deliver(d)
		if atCapacity && i < capacity {
			t.Errorf("mailbox at capacity %d", i)
		} else if !atCapacity && i >= capacity {
			t.Errorf("mailbox below capacity %d", i)
		}
	}

	require.Equal(t, expected, m.RetrieveLatestAndClear())
	require.Len(t, m.RetrieveAll(), 0)
}

func TestMailbox_NoEmptyReceivesWhenCapacityIsTwo(t *testing.T) {
	m := NewMailbox[int](2)

	var (
		recvd         []int
		emptyReceives []int
	)

	chDone := make(chan struct{})
	go func() {
		defer close(chDone)
		for range m.Notify() {
			x, exists := m.Retrieve()
			if !exists {
				emptyReceives = append(emptyReceives, recvd[len(recvd)-1])
			} else {
				recvd = append(recvd, x)
			}
		}
	}()

	for i := 0; i < 100000; i++ {
		m.Deliver(i)
	}
	close(m.chNotify)

	<-chDone
	require.Len(t, emptyReceives, 0)
}

// intRange returns [from, to).
func intRange(from, to int) []int {
	out := make([]int, 0, to-from)
	for i := from; i < to; i++ {
		out = append(out, i)
	}
	return out
}

// TestMailbox_FIFO verifies that both Retrieve and RetrieveAll observe
// deliveries in FIFO order, across bounded/unbounded mailboxes, overflow,
// ring-buffer growth, and growth from a wrapped (non-zero start) state.
func TestMailbox_FIFO(t *testing.T) {
	tests := []struct {
		name        string
		capacity    uint64
		preDeliver  []int
		preRetrieve int
		deliver     []int
		want        []int
	}{
		{"unbounded-simple", 0, nil, 0, intRange(0, 5), intRange(0, 5)},
		{"bounded-under-capacity", 10, nil, 0, intRange(0, 3), intRange(0, 3)},
		{"bounded-exact-capacity", 5, nil, 0, intRange(0, 5), intRange(0, 5)},
		{"bounded-overflow-drops-oldest", 3, nil, 0, intRange(0, 5), intRange(2, 5)},
		{"capacity-1-keeps-newest", 1, nil, 0, intRange(0, 3), []int{2}},
		{"bounded-wraparound", 4, intRange(0, 4), 2, intRange(4, 7), intRange(3, 7)},
		{"unbounded-growth", 0, nil, 0, intRange(0, 350), intRange(0, 350)},
		{"unbounded-growth-from-wrapped-state", 0, intRange(0, 150), 50, intRange(150, 350), intRange(50, 350)},
	}

	for _, tt := range tests {
		for _, drain := range []string{"retrieve", "retrieveAll"} {
			t.Run(tt.name+"/"+drain, func(t *testing.T) {
				m := NewMailbox[int](tt.capacity)
				for _, d := range tt.preDeliver {
					m.Deliver(d)
				}
				for i := 0; i < tt.preRetrieve; i++ {
					_, ok := m.Retrieve()
					require.True(t, ok)
				}
				for _, d := range tt.deliver {
					m.Deliver(d)
				}

				var got []int
				switch drain {
				case "retrieve":
					for {
						x, ok := m.Retrieve()
						if !ok {
							break
						}
						got = append(got, x)
					}
				case "retrieveAll":
					got = m.RetrieveAll()
				}
				require.Equal(t, tt.want, got)
				require.Zero(t, m.queueLen.Load())
			})
		}
	}
}

// TestMailbox_MixedOpSequences is a scripted table-driven machine covering the
// mailbox's semantic contract step by step: FIFO order, drop-oldest overflow
// with wasOverCapacity=true, RetrieveLatestAndClear returning the newest,
// queueLen consistency after every operation, and the single-slot non-blocking
// Notify behavior — including the capacity-1 spurious-empty-Retrieve case
// documented on the struct.
func TestMailbox_MixedOpSequences(t *testing.T) {
	const (
		opDeliver        = "deliver"        // deliver val; assert wasOverCapacity == wantOver
		opRetrieve       = "retrieve"       // assert Retrieve returns (val, true)
		opRetrieveEmpty  = "retrieveEmpty"  // assert Retrieve returns (_, false)
		opRetrieveAll    = "retrieveAll"    // assert RetrieveAll returns wantAll
		opLatestAndClear = "latestAndClear" // assert RetrieveLatestAndClear returns val
		opNotifyConsume  = "notifyConsume"  // assert a notify token is pending, and consume it
		opNotifyNone     = "notifyNone"     // assert no notify token is pending
	)
	type step struct {
		op       string
		val      int
		wantOver bool
		wantAll  []int
		wantLen  int64 // expected queueLen after the step
	}

	tests := []struct {
		name     string
		capacity uint64
		steps    []step
	}{
		{
			name:     "bounded-wraparound-overflow",
			capacity: 4,
			steps: []step{
				{op: opDeliver, val: 1, wantLen: 1},
				{op: opDeliver, val: 2, wantLen: 2},
				{op: opDeliver, val: 3, wantLen: 3},
				{op: opDeliver, val: 4, wantLen: 4},
				{op: opRetrieve, val: 1, wantLen: 3},
				{op: opRetrieve, val: 2, wantLen: 2},
				{op: opDeliver, val: 5, wantLen: 3}, // wraps into freed slots
				{op: opDeliver, val: 6, wantLen: 4},
				{op: opDeliver, val: 7, wantOver: true, wantLen: 4}, // drops oldest (3)
				{op: opRetrieveAll, wantAll: []int{4, 5, 6, 7}, wantLen: 0},
				{op: opRetrieveEmpty, wantLen: 0},
			},
		},
		{
			name:     "overflow-then-drain-then-reuse",
			capacity: 2,
			steps: []step{
				{op: opDeliver, val: 1, wantLen: 1},
				{op: opDeliver, val: 2, wantLen: 2},
				{op: opDeliver, val: 3, wantOver: true, wantLen: 2}, // drops 1
				{op: opRetrieve, val: 2, wantLen: 1},
				{op: opRetrieve, val: 3, wantLen: 0},
				{op: opRetrieveEmpty, wantLen: 0},
				{op: opDeliver, val: 4, wantLen: 1},
				{op: opLatestAndClear, val: 4, wantLen: 0},
				{op: opRetrieveEmpty, wantLen: 0},
			},
		},
		{
			name:     "latest-and-clear-returns-newest-wrapped",
			capacity: 3,
			steps: []step{
				{op: opDeliver, val: 1, wantLen: 1},
				{op: opDeliver, val: 2, wantLen: 2},
				{op: opDeliver, val: 3, wantLen: 3},
				{op: opRetrieve, val: 1, wantLen: 2},
				{op: opDeliver, val: 4, wantLen: 3},                 // wraps
				{op: opDeliver, val: 5, wantOver: true, wantLen: 3}, // drops 2; queue is [3,4,5]
				{op: opLatestAndClear, val: 5, wantLen: 0},
				{op: opRetrieveEmpty, wantLen: 0},
				{op: opRetrieveAll, wantAll: nil, wantLen: 0},
			},
		},
		{
			name:     "latest-and-clear-on-empty-returns-zero-value",
			capacity: 3,
			steps: []step{
				{op: opLatestAndClear, val: 0, wantLen: 0},
				{op: opRetrieveEmpty, wantLen: 0},
			},
		},
		{
			name:     "drain-via-retrieveall-then-refill",
			capacity: 0,
			steps: []step{
				{op: opDeliver, val: 1, wantLen: 1},
				{op: opDeliver, val: 2, wantLen: 2},
				{op: opRetrieveAll, wantAll: []int{1, 2}, wantLen: 0},
				{op: opDeliver, val: 3, wantLen: 1},
				{op: opRetrieve, val: 3, wantLen: 0},
			},
		},
		{
			name:     "notify-single-slot-coalesces-tokens",
			capacity: 4,
			steps: []step{
				{op: opNotifyNone, wantLen: 0},
				{op: opDeliver, val: 1, wantLen: 1},
				{op: opDeliver, val: 2, wantLen: 2}, // second token coalesced away
				{op: opDeliver, val: 3, wantLen: 3},
				{op: opNotifyConsume, wantLen: 3}, // exactly one token pending
				{op: opNotifyNone, wantLen: 3},
				{op: opDeliver, val: 4, wantLen: 4}, // re-arms after drain
				{op: opNotifyConsume, wantLen: 4},
				{op: opNotifyNone, wantLen: 4},
			},
		},
		{
			// The case documented on the Mailbox struct: with capacity 1, a
			// notification can outlive its message, yielding an empty Retrieve.
			name:     "capacity-1-spurious-empty-retrieve",
			capacity: 1,
			steps: []step{
				{op: opDeliver, val: 1, wantLen: 1},
				{op: opNotifyConsume, wantLen: 1},
				{op: opDeliver, val: 2, wantOver: true, wantLen: 1}, // drops 1, posts a second token
				{op: opRetrieve, val: 2, wantLen: 0},
				{op: opNotifyConsume, wantLen: 0}, // token pending, but queue already drained...
				{op: opRetrieveEmpty, wantLen: 0}, // ...so this Retrieve comes up empty
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMailbox[int](tt.capacity)
			for i, s := range tt.steps {
				switch s.op {
				case opDeliver:
					require.Equal(t, s.wantOver, m.Deliver(s.val), "step %d (%s %d)", i, s.op, s.val)
				case opRetrieve:
					got, ok := m.Retrieve()
					require.True(t, ok, "step %d (%s)", i, s.op)
					require.Equal(t, s.val, got, "step %d (%s)", i, s.op)
				case opRetrieveEmpty:
					_, ok := m.Retrieve()
					require.False(t, ok, "step %d (%s)", i, s.op)
				case opRetrieveAll:
					require.Equal(t, s.wantAll, m.RetrieveAll(), "step %d (%s)", i, s.op)
				case opLatestAndClear:
					require.Equal(t, s.val, m.RetrieveLatestAndClear(), "step %d (%s)", i, s.op)
				case opNotifyConsume:
					select {
					case <-m.Notify():
					default:
						t.Fatalf("step %d: expected a pending notify token, found none", i)
					}
				case opNotifyNone:
					select {
					case <-m.Notify():
						t.Fatalf("step %d: expected no pending notify token, found one", i)
					default:
					}
				default:
					t.Fatalf("step %d: unknown op %q", i, s.op)
				}
				require.Equal(t, s.wantLen, m.queueLen.Load(), "step %d (%s): queueLen", i, s.op)
			}
		})
	}
}

// TestMailbox_DeliverNeverBlocksWithoutReader locks in the non-blocking
// producer contract: with nobody consuming Notify, a single goroutine can
// deliver indefinitely. If Deliver ever blocked on the notify channel, this
// test would hang and time out.
func TestMailbox_DeliverNeverBlocksWithoutReader(t *testing.T) {
	for _, capacity := range []uint64{0, 1, 100} {
		m := NewMailbox[int](capacity)
		for i := 0; i < 1000; i++ {
			m.Deliver(i)
		}
		if capacity == 0 {
			require.EqualValues(t, 1000, m.queueLen.Load())
		} else {
			require.EqualValues(t, capacity, m.queueLen.Load())
		}
	}
}

// TestMailbox_BoundedLazyAllocation pins the lazy-allocation contract: a
// bounded mailbox starts at defaultInitialCap slots (not its full capacity),
// grows by doubling only under load, never grows past capacity, and keeps
// exact drop-oldest FIFO semantics across the capped growth.
func TestMailbox_BoundedLazyAllocation(t *testing.T) {
	t.Run("large-capacity-starts-small", func(t *testing.T) {
		m := NewMailbox[int](10_000)
		require.Len(t, m.buf, defaultInitialCap)

		// Under-capacity load grows the buffer but stays proportional to
		// count, not capacity.
		for i := 0; i < 150; i++ {
			require.False(t, m.Deliver(i))
		}
		require.LessOrEqual(t, len(m.buf), 400)
		require.GreaterOrEqual(t, len(m.buf), 150)

		// Overflow the full capacity: buffer tops out at exactly capacity,
		// oldest 50 dropped, FIFO preserved.
		for i := 150; i < 10_050; i++ {
			over := m.Deliver(i)
			require.Equal(t, i >= 10_000, over, "delivery %d", i)
		}
		require.Len(t, m.buf, 10_000)
		require.Equal(t, intRange(50, 10_050), m.RetrieveAll())
	})

	t.Run("small-capacity-allocates-exactly", func(t *testing.T) {
		for _, capacity := range []uint64{1, 2, 99} {
			m := NewMailbox[int](capacity)
			require.Len(t, m.buf, int(capacity))
		}
	})

	t.Run("unbounded-starts-at-default", func(t *testing.T) {
		m := NewMailbox[int](0)
		require.Len(t, m.buf, defaultInitialCap)
	})
}

// TestMailbox_RetrieveShrinksDrainedRing pins the Retrieve-path shrink: a ring
// grown for a burst shrinks back as a Retrieve-only consumer drains it, instead
// of staying pinned at its high-water mark (clearLocked never runs on that
// consumption pattern). FIFO order must survive the mid-drain resizes.
func TestMailbox_RetrieveShrinksDrainedRing(t *testing.T) {
	t.Run("full-drain-rests-near-initial-cap", func(t *testing.T) {
		m := NewMailbox[int](0)
		const n = 100_000
		for i := 0; i < n; i++ {
			m.Deliver(i)
		}
		grown := len(m.buf)
		require.GreaterOrEqual(t, grown, n)

		var got []int
		for {
			x, ok := m.Retrieve()
			if !ok {
				break
			}
			got = append(got, x)
		}
		require.Equal(t, intRange(0, n), got)
		// Incremental shrink disarms once len(buf) <= 4*initialCap, so the
		// resting size is bounded by that constant — not the 100k high-water
		// mark. (clearLocked's shrink shares the same 4x threshold.)
		require.LessOrEqual(t, len(m.buf), 4*defaultInitialCap)
	})

	t.Run("partial-drain-shrinks-proportionally", func(t *testing.T) {
		m := NewMailbox[int](0)
		for i := 0; i < 10_000; i++ {
			m.Deliver(i)
		}
		grown := len(m.buf)
		// Drain to 1/8 of the grown size — well past the 1/4 trigger.
		drainTo := grown / 8
		for m.count > drainTo {
			_, ok := m.Retrieve()
			require.True(t, ok)
		}
		require.Less(t, len(m.buf), grown)
		require.GreaterOrEqual(t, len(m.buf), m.count)
		// The survivors are still FIFO-intact.
		require.Equal(t, intRange(10_000-m.count, 10_000), m.RetrieveAll())
	})

	t.Run("shrink-from-wrapped-state", func(t *testing.T) {
		m := NewMailbox[int](0)
		// Wrap the ring: fill past one doubling, drain some, refill so start > 0.
		for i := 0; i < 300; i++ {
			m.Deliver(i)
		}
		for i := 0; i < 200; i++ {
			_, ok := m.Retrieve()
			require.True(t, ok)
		}
		for i := 300; i < 5_000; i++ {
			m.Deliver(i)
		}
		var got []int
		for {
			x, ok := m.Retrieve()
			if !ok {
				break
			}
			got = append(got, x)
		}
		require.Equal(t, intRange(200, 5_000), got)
		require.LessOrEqual(t, len(m.buf), 4*defaultInitialCap)
	})
}

func TestMailbox_load(t *testing.T) {
	for _, tt := range []struct {
		name     string
		capacity uint64
		deliver  []int
		exp      float64

		retrieve int
		exp2     float64

		all bool
	}{
		{"single-all", 1, []int{1}, 100, 0, 100, true},
		{"single-latest", 1, []int{1}, 100, 0, 100, false},
		{"ten-low", 10, []int{1}, 10, 1, 0.0, false},
		{"ten-full-all", 10, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 100, 5, 50, true},
		{"ten-full-latest", 10, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, 100, 5, 50, false},
		{"ten-overflow", 10, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11}, 100, 5, 50, false},
		{"nine", 9, []int{1, 2, 3}, 100.0 / 3.0, 2, 100.0 / 9.0, true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMailbox[int](tt.capacity)

			// Queue deliveries
			for i, d := range tt.deliver {
				atCapacity := m.Deliver(d)
				if atCapacity && i < int(tt.capacity) {
					t.Errorf("mailbox at capacity %d", i)
				} else if !atCapacity && i >= int(tt.capacity) {
					t.Errorf("mailbox below capacity %d", i)
				}
			}
			gotCap, gotLoad := m.load()
			require.Equal(t, gotCap, tt.capacity)
			require.Equal(t, gotLoad, tt.exp)

			// Retrieve some
			for i := 0; i < tt.retrieve; i++ {
				_, ok := m.Retrieve()
				require.True(t, ok)
			}
			gotCap, gotLoad = m.load()
			require.Equal(t, gotCap, tt.capacity)
			require.Equal(t, gotLoad, tt.exp2)

			// Drain it
			if tt.all {
				m.RetrieveAll()
			} else {
				m.RetrieveLatestAndClear()
			}
			gotCap, gotLoad = m.load()
			require.Equal(t, gotCap, tt.capacity)
			require.Equal(t, gotLoad, 0.0)
		})
	}
}
