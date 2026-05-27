package queue

import (
	"sync"
	"sync/atomic"
	"testing"
)

// Experiment: is cached SPSC slower than MPMC-Padded because of false
// sharing on its packed [BoundedQueueSize]int data array?
//
// When producer and consumer run in lockstep (a tight throughput loop),
// head and tail sit within ~16 ints of each other — i.e. the SAME 128-byte
// cache line — so the data line ping-pongs between the two cores on every
// op. MPMC pads each slot to its own line and sidesteps this.
//
// paddedElemCachedSPSC is the swapped/cached SPSC, but with each data
// element padded to its own 128-byte line (like MPMC's slot). If it
// matches MPMC-Padded throughput, packed-array false sharing is confirmed
// as the remaining gap.

type paddedElem[T any] struct {
	val T
	_   [120]byte // 128 - 8: one element per cache line
}

type paddedElemCachedSPSC[T any] struct {
	arr [BoundedQueueSize]paddedElem[T]

	_          [120]byte
	head       atomic.Uint64
	cachedTail uint64

	_          [120]byte
	tail       atomic.Uint64
	cachedHead uint64
}

func (s *paddedElemCachedSPSC[T]) Enqueue(v T) bool {
	t := s.tail.Load()
	if t-s.cachedHead >= BoundedQueueSize {
		s.cachedHead = s.head.Load()
		if t-s.cachedHead >= BoundedQueueSize {
			return false
		}
	}
	s.arr[t&queueMask].val = v
	s.tail.Store(t + 1)
	return true
}

func (s *paddedElemCachedSPSC[T]) Dequeue() (T, bool) {
	h := s.head.Load()
	if s.cachedTail-h == 0 {
		s.cachedTail = s.tail.Load()
		if s.cachedTail-h == 0 {
			var zero T
			return zero, false
		}
	}
	v := s.arr[h&queueMask].val
	s.head.Store(h + 1)
	return v, true
}

func BenchmarkSPSCDataPadding(b *testing.B) {
	targets := []struct {
		name string
		make func() Queue[int]
	}{
		{"MPMC-Padded", func() Queue[int] { return NewLockFreePaddedMPMC[int]() }},
		{"SPSC-Cached-Packed", func() Queue[int] { return NewCachedSPSCQueue[int]() }},
		{"SPSC-Cached-PaddedElem", func() Queue[int] { return &paddedElemCachedSPSC[int]{} }},
	}
	for _, t := range targets {
		b.Run(t.name, func(b *testing.B) {
			q := t.make()
			var wg sync.WaitGroup
			b.ResetTimer()
			wg.Add(1)
			go func() {
				defer wg.Done()
				for k := range b.N {
					for !q.Enqueue(k) {
					}
				}
			}()
			for range b.N {
				for {
					if _, ok := q.Dequeue(); ok {
						break
					}
				}
			}
			wg.Wait()
			b.StopTimer()
		})
	}
}

// instrumentedCachedSPSC counts how often each side does the "expensive"
// cross-core Load — i.e. how often the cache actually misses and has to
// read the other side's index. Hypothesis: in a balanced throughput loop
// the queue hovers near EMPTY (consumer keeps pace), so the consumer's
// cachedTail is almost always stale → it reloads the real tail nearly
// every Dequeue → tail ping-pongs every op. The producer, far from the
// FULL boundary, almost never reloads head. If so, the consumer-side cache
// is effectively defeated by this workload.
type instrumentedCachedSPSC[T any] struct {
	arr [BoundedQueueSize]T

	_          [120]byte
	head       atomic.Uint64
	cachedTail uint64

	_          [120]byte
	tail       atomic.Uint64
	cachedHead uint64

	headLoads atomic.Int64 // times Enqueue reloaded the real head
	tailLoads atomic.Int64 // times Dequeue reloaded the real tail
}

func (s *instrumentedCachedSPSC[T]) Enqueue(v T) bool {
	t := s.tail.Load()
	if t-s.cachedHead >= BoundedQueueSize {
		s.cachedHead = s.head.Load()
		s.headLoads.Add(1)
		if t-s.cachedHead >= BoundedQueueSize {
			return false
		}
	}
	s.arr[t&queueMask] = v
	s.tail.Store(t + 1)
	return true
}

func (s *instrumentedCachedSPSC[T]) Dequeue() (T, bool) {
	h := s.head.Load()
	if s.cachedTail-h == 0 {
		s.cachedTail = s.tail.Load()
		s.tailLoads.Add(1)
		if s.cachedTail-h == 0 {
			var zero T
			return zero, false
		}
	}
	v := s.arr[h&queueMask]
	s.head.Store(h + 1)
	return v, true
}

func BenchmarkSPSCCacheRefresh(b *testing.B) {
	q := &instrumentedCachedSPSC[int]{}
	var wg sync.WaitGroup
	b.ResetTimer()
	wg.Add(1)
	go func() {
		defer wg.Done()
		for k := range b.N {
			for !q.Enqueue(k) {
			}
		}
	}()
	for range b.N {
		for {
			if _, ok := q.Dequeue(); ok {
				break
			}
		}
	}
	wg.Wait()
	b.StopTimer()

	n := float64(b.N)
	b.ReportMetric(float64(q.headLoads.Load())/n, "headLoads/op")
	b.ReportMetric(float64(q.tailLoads.Load())/n, "tailLoads/op")
}
