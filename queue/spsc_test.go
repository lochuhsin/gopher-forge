package queue

import (
	"fmt"
	"runtime"
	"testing"
)

// SPSC tests: exactly one producer owns Enqueue, one consumer owns
// Dequeue. There are deliberately no multi-producer or multi-consumer
// variants — either would violate the SPSC contract (tail is
// producer-private, head is consumer-private; a second writer on
// either side races with no CAS to protect it).

var spscQueueFactories = map[string]func() Queue[int]{
	"LockFreeSPSC":       func() Queue[int] { return NewSPSCQueue[int]() },
	"LockFreeCachedSPSC": func() Queue[int] { return NewCachedSPSCQueue[int]() },
}

// Conservation workload: 1 producer + 1 consumer drain concurrently, so
// total ops may exceed capacity — a full queue just spins until the
// consumer frees a slot.
const spscConvOps = 500

const (
	spscChaosOps    = 2000
	spscChaosRounds = 30
)

func TestSPSCQueueEmpty(t *testing.T) {
	for name, factory := range spscQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			v, ok := q.Dequeue()
			if ok {
				t.Errorf("Dequeue on empty: got (%d, true), want (_, false)", v)
			}
		})
	}
}

func TestSPSCSequentialFIFO(t *testing.T) {
	for name, factory := range spscQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			const n = 10
			for i := range n {
				if !q.Enqueue(i) {
					t.Fatalf("Enqueue(%d) returned false unexpectedly", i)
				}
			}
			for i := range n {
				v, ok := q.Dequeue()
				if !ok {
					t.Fatalf("Dequeue returned ok=false at i=%d", i)
				}
				if v != i {
					t.Errorf("Dequeue = %d, want %d (FIFO order)", v, i)
				}
			}
			if v, ok := q.Dequeue(); ok {
				t.Errorf("expected empty, got (%d, true)", v)
			}
		})
	}
}

// TestSPSCFull fills the queue to capacity and verifies Enqueue reports
// back-pressure (false) instead of overwriting, then that draining
// preserves FIFO order.
func TestSPSCFull(t *testing.T) {
	for name, factory := range spscQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			for i := range BoundedQueueSize {
				if !q.Enqueue(i) {
					t.Fatalf("Enqueue(%d) returned false before reaching capacity %d", i, BoundedQueueSize)
				}
			}
			if q.Enqueue(-1) {
				t.Errorf("Enqueue on full queue returned true, want false")
			}
			for i := range BoundedQueueSize {
				v, ok := q.Dequeue()
				if !ok {
					t.Fatalf("Dequeue returned ok=false at i=%d on full queue", i)
				}
				if v != i {
					t.Errorf("Dequeue = %d, want %d (FIFO order)", v, i)
				}
			}
			if v, ok := q.Dequeue(); ok {
				t.Errorf("expected empty after full drain, got (%d, true)", v)
			}
		})
	}
}

// TestSPSCWraparound interleaves enqueue/dequeue past BoundedQueueSize so
// the head/tail counters wrap the ring index (pos & queueMask) many
// times, checking FIFO order survives the wrap.
func TestSPSCWraparound(t *testing.T) {
	for name, factory := range spscQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			const (
				batch  = 100
				rounds = 50 // 100*50 = 5000 ops, well past cap 1024
			)
			next := 0
			for r := range rounds {
				for range batch {
					if !q.Enqueue(next) {
						t.Fatalf("round %d: Enqueue(%d) returned false unexpectedly", r, next)
					}
					next++
				}
				for i := range batch {
					want := r*batch + i
					v, ok := q.Dequeue()
					if !ok {
						t.Fatalf("round %d: Dequeue returned ok=false at i=%d", r, i)
					}
					if v != want {
						t.Errorf("round %d: Dequeue = %d, want %d (FIFO after wrap)", r, v, want)
					}
				}
			}
			if v, ok := q.Dequeue(); ok {
				t.Errorf("expected empty, got (%d, true)", v)
			}
		})
	}
}

// TestSPSCConcurrentEnqueueDequeue runs the one-producer/one-consumer
// conservation check: every enqueued value must appear exactly once
// across (dequeued ∪ remaining), with no duplicates or phantoms.
func TestSPSCConcurrentEnqueueDequeue(t *testing.T) {
	for name, factory := range spscQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			errs := runQConservation(q, 1, 1, spscConvOps)
			for _, e := range errs {
				t.Error(e)
			}
		})
	}
}

func TestSPSCConcurrentChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in -short mode")
	}
	for _, procs := range chaosQGOMAXPROCS() {
		t.Run(fmt.Sprintf("GOMAXPROCS=%d", procs), func(t *testing.T) {
			prev := runtime.GOMAXPROCS(procs)
			t.Cleanup(func() { runtime.GOMAXPROCS(prev) })

			for name, factory := range spscQueueFactories {
				t.Run(name, func(t *testing.T) {
					for r := range spscChaosRounds {
						q := factory()
						errs := runQConservation(q, 1, 1, spscChaosOps)
						if len(errs) > 0 {
							for _, e := range errs {
								t.Errorf("round %d: %s", r, e)
							}
							return
						}
					}
				})
			}
		})
	}
}
