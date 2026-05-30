package memory

import (
	"sync"
	"testing"
)

// Compares PaddedCounters vs UnpaddedCounters under two scenarios:
//
//  1. Single-writer: one goroutine incrementing IncA only. Padding
//     has no contention to relieve here, so the two variants should
//     be within noise of each other. This is the control — it proves
//     the padding bytes themselves are not the cost.
//  2. Two-writer contended: one goroutine hammers IncA, another
//     hammers IncB, in parallel. UnpaddedCounters places a and b on
//     the same 64- or 128-byte cache line, so every Add invalidates
//     the other CPU's copy (MESI thrash). PaddedCounters separates
//     them by CacheLineSize so the two writers own disjoint lines
//     and proceed without interfering — the difference between the
//     two benchmarks is the false-sharing tax.
//
// In the contended benchmarks each goroutine performs b.N
// increments concurrently; wall time ≈ b.N * per-op cost as seen by
// one contending writer, so the ns/op figure is directly comparable
// to the single-writer case.

func BenchmarkUnpaddedCountersSingleWriter(b *testing.B) {
	var c UnpaddedCounters
	b.ResetTimer()
	for range b.N {
		c.IncA()
	}
}

func BenchmarkPaddedCountersSingleWriter(b *testing.B) {
	var c PaddedCounters
	b.ResetTimer()
	for range b.N {
		c.IncA()
	}
}

func BenchmarkUnpaddedCountersTwoWriters(b *testing.B) {
	var c UnpaddedCounters
	var wg sync.WaitGroup
	wg.Add(2)
	b.ResetTimer()
	go func() {
		defer wg.Done()
		for range b.N {
			c.IncA()
		}
	}()
	go func() {
		defer wg.Done()
		for range b.N {
			c.IncB()
		}
	}()
	wg.Wait()
}

func BenchmarkPaddedCountersTwoWriters(b *testing.B) {
	var c PaddedCounters
	var wg sync.WaitGroup
	wg.Add(2)
	b.ResetTimer()
	go func() {
		defer wg.Done()
		for range b.N {
			c.IncA()
		}
	}()
	go func() {
		defer wg.Done()
		for range b.N {
			c.IncB()
		}
	}()
	wg.Wait()
}
