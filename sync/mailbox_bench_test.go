package bsync

import (
	"fmt"
	"testing"
)

// BenchmarkDeliver measures the per-delivery cost at a constant queue depth.
// Each mailbox is bounded and pre-filled to capacity, so every timed Deliver
// displaces the oldest element and the depth stays fixed at N. An O(1)
// implementation shows flat ns/op across depths; the old prepend-and-copy
// implementation scaled linearly with depth.
func BenchmarkDeliver(b *testing.B) {
	for _, depth := range []int{10, 1000, 100000} {
		b.Run(fmt.Sprintf("depth-%d", depth), func(b *testing.B) {
			m := NewMailbox[int](uint64(depth))
			for i := 0; i < depth; i++ {
				m.Deliver(i)
			}

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				m.Deliver(i)
			}
		})
	}
}

// BenchmarkDeliverUnbounded measures amortized delivery cost on an unbounded
// mailbox that grows from empty to b.N. Amortized O(1) shows flat ns/op and
// well under one allocation per op (growth doublings only).
func BenchmarkDeliverUnbounded(b *testing.B) {
	m := NewMailbox[int](0)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Deliver(i)
	}
}
