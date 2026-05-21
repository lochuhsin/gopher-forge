package queue

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
)

// MPSC tests: multiple producers race on Enqueue, a single consumer
// owns Dequeue. There is no ConcurrentDequeueOnly counterpart because
// concurrent Dequeue would violate the MPSC contract.

// Conservation workload: total ops must stay ≤ BoundedQueueSize since
// the consumer does not drain concurrently.
const (
	mpscConvNumProducers   = 4
	mpscConvOpsPerProducer = 25
)

const (
	mpscChaosNumProducers   = 8
	mpscChaosOpsPerProducer = 200
	mpscChaosRounds         = 30
)

func TestMPSCQueueEmpty(t *testing.T) {
	for name, factory := range mpscQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			v, ok := q.Dequeue()
			if ok {
				t.Errorf("Dequeue on empty: got (%d, true), want (_, false)", v)
			}
		})
	}
}

func TestMPSCSequentialFIFO(t *testing.T) {
	for name, factory := range mpscQueueFactories {
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

func TestMPSCConcurrentEnqueueOnly(t *testing.T) {
	for name, factory := range mpscQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			var wg sync.WaitGroup
			for w := range mpscConvNumProducers {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					base := id * mpscConvOpsPerProducer
					for i := range mpscConvOpsPerProducer {
						retryEnqueue(q, base+i)
					}
				}(w)
			}
			wg.Wait()

			total := mpscConvNumProducers * mpscConvOpsPerProducer
			seen := make(map[int]int, total)
			for {
				v, ok := q.Dequeue()
				if !ok {
					break
				}
				seen[v]++
			}
			verifyQMultiset(t, seen, total)
		})
	}
}

func TestMPSCConcurrentEnqueueDequeue(t *testing.T) {
	for name, factory := range mpscQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			errs := runQConservation(q, mpscConvNumProducers, 1, mpscConvOpsPerProducer)
			for _, e := range errs {
				t.Error(e)
			}
		})
	}
}

func TestMPSCConcurrentChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in -short mode")
	}
	for _, procs := range chaosQGOMAXPROCS() {
		t.Run(fmt.Sprintf("GOMAXPROCS=%d", procs), func(t *testing.T) {
			prev := runtime.GOMAXPROCS(procs)
			t.Cleanup(func() { runtime.GOMAXPROCS(prev) })

			for name, factory := range mpscQueueFactories {
				t.Run(name, func(t *testing.T) {
					for r := range mpscChaosRounds {
						q := factory()
						errs := runQConservation(q, mpscChaosNumProducers, 1, mpscChaosOpsPerProducer)
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
