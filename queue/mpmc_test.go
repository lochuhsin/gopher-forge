package queue

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
)

// Shared helpers used by both MPMC and MPSC test files.

func retryEnqueue(q Queue[int], v int) {
	for !q.Enqueue(v) {
		runtime.Gosched()
	}
}

func verifyQMultiset(t *testing.T, seen map[int]int, totalExpected int) {
	t.Helper()
	expectedFound := 0
	for v, c := range seen {
		if v < 0 || v >= totalExpected {
			t.Errorf("phantom: value %d outside [0,%d)", v, totalExpected)
			continue
		}
		expectedFound++
		if c > 1 {
			t.Errorf("duplicate: value %d appeared %d times", v, c)
		}
	}
	if expectedFound < totalExpected {
		t.Errorf("lost: %d/%d expected values missing", totalExpected-expectedFound, totalExpected)
	}
}

func chaosQGOMAXPROCS() []int { return []int{1, 2, 4, 8} }

// runQConservation runs numProducers enqueuers (each pushing a disjoint
// integer range) concurrent with numConsumers dequeuers, then verifies
// that every enqueued value appears exactly once across (dequeued ∪ remaining).
//
//	MPMC: runQConservation(q, N, N, ops)
//	MPSC: runQConservation(q, N, 1, ops)
//	SPSC: runQConservation(q, 1, 1, ops)
func runQConservation(q Queue[int], numProducers, numConsumers, opsPerProducer int) []string {
	var enqWg, deqWg sync.WaitGroup
	enqDone := make(chan struct{})

	for w := range numProducers {
		enqWg.Add(1)
		go func(id int) {
			defer enqWg.Done()
			base := id * opsPerProducer
			for i := range opsPerProducer {
				retryEnqueue(q, base+i)
			}
		}(w)
	}

	dequeuedCh := make(chan []int, numConsumers)
	for range numConsumers {
		deqWg.Go(func() {
			local := []int{}
			for {
				if v, ok := q.Dequeue(); ok {
					local = append(local, v)
					continue
				}
				select {
				case <-enqDone:
					dequeuedCh <- local
					return
				default:
					runtime.Gosched()
				}
			}
		})
	}

	enqWg.Wait()
	close(enqDone)
	deqWg.Wait()
	close(dequeuedCh)

	remaining := []int{}
	for {
		v, ok := q.Dequeue()
		if !ok {
			break
		}
		remaining = append(remaining, v)
	}

	dequeued := []int{}
	for l := range dequeuedCh {
		dequeued = append(dequeued, l...)
	}

	totalExpected := numProducers * opsPerProducer
	counts := make(map[int]int, totalExpected)
	for _, v := range dequeued {
		counts[v]++
	}
	for _, v := range remaining {
		counts[v]++
	}

	const maxErrs = 10
	var errs []string
	addErr := func(format string, args ...any) {
		if len(errs) < maxErrs {
			errs = append(errs, fmt.Sprintf(format, args...))
		}
	}

	expectedFound := 0
	for v, c := range counts {
		if v < 0 || v >= totalExpected {
			addErr("phantom: value %d outside expected [0,%d)", v, totalExpected)
			continue
		}
		expectedFound++
		if c > 1 {
			addErr("duplicate: value %d appeared %d times", v, c)
		}
	}
	if expectedFound < totalExpected {
		addErr("lost: %d/%d expected values missing", totalExpected-expectedFound, totalExpected)
	}
	return errs
}

// Conservation workload (no concurrent drain): total ops must stay
// ≤ BoundedQueueSize so Enqueue does not block on retries.
const (
	mpmcConvNumWorkers   = 4
	mpmcConvOpsPerWorker = 25
)

// Chaos workload runs enqueue and dequeue together, so total ops may
// exceed capacity — full queues retry until consumers drain.
const (
	mpmcChaosNumWorkers   = 8
	mpmcChaosOpsPerWorker = 200
	mpmcChaosRounds       = 30
)

func TestMPMCQueueEmpty(t *testing.T) {
	for name, factory := range mpmcQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			v, ok := q.Dequeue()
			if ok {
				t.Errorf("Dequeue on empty: got (%d, true), want (_, false)", v)
			}
		})
	}
}

func TestMPMCSequentialFIFO(t *testing.T) {
	for name, factory := range mpmcQueueFactories {
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

func TestMPMCConcurrentEnqueueOnly(t *testing.T) {
	for name, factory := range mpmcQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			var wg sync.WaitGroup
			for w := range mpmcConvNumWorkers {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					base := id * mpmcConvOpsPerWorker
					for i := range mpmcConvOpsPerWorker {
						retryEnqueue(q, base+i)
					}
				}(w)
			}
			wg.Wait()

			total := mpmcConvNumWorkers * mpmcConvOpsPerWorker
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

func TestMPMCConcurrentDequeueOnly(t *testing.T) {
	for name, factory := range mpmcQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			total := mpmcConvNumWorkers * mpmcConvOpsPerWorker
			for i := range total {
				if !q.Enqueue(i) {
					t.Fatalf("pre-fill Enqueue(%d) returned false; total=%d may exceed cap", i, total)
				}
			}

			var wg sync.WaitGroup
			dequeuedCh := make(chan []int, mpmcConvNumWorkers)
			for range mpmcConvNumWorkers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					local := []int{}
					for {
						v, ok := q.Dequeue()
						if !ok {
							dequeuedCh <- local
							return
						}
						local = append(local, v)
					}
				}()
			}
			wg.Wait()
			close(dequeuedCh)

			seen := make(map[int]int, total)
			for l := range dequeuedCh {
				for _, v := range l {
					seen[v]++
				}
			}
			verifyQMultiset(t, seen, total)
		})
	}
}

func TestMPMCConcurrentEnqueueDequeue(t *testing.T) {
	for name, factory := range mpmcQueueFactories {
		t.Run(name, func(t *testing.T) {
			q := factory()
			errs := runQConservation(q, mpmcConvNumWorkers, mpmcConvNumWorkers, mpmcConvOpsPerWorker)
			for _, e := range errs {
				t.Error(e)
			}
		})
	}
}

func TestMPMCConcurrentChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in -short mode")
	}
	for _, procs := range chaosQGOMAXPROCS() {
		t.Run(fmt.Sprintf("GOMAXPROCS=%d", procs), func(t *testing.T) {
			prev := runtime.GOMAXPROCS(procs)
			t.Cleanup(func() { runtime.GOMAXPROCS(prev) })

			for name, factory := range mpmcQueueFactories {
				t.Run(name, func(t *testing.T) {
					for r := range mpmcChaosRounds {
						q := factory()
						errs := runQConservation(q, mpmcChaosNumWorkers, mpmcChaosNumWorkers, mpmcChaosOpsPerWorker)
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

// Meta-test: verify the conservation check actually detects known bugs
// (buggy queues defined in buggy_test.go).
func TestMPMCConservation_CatchesBugs(t *testing.T) {
	const maxRounds = 30
	for name, factory := range buggyQueueFactories {
		t.Run(name, func(t *testing.T) {
			for r := range maxRounds {
				q := factory()
				errs := runQConservation(q, mpmcConvNumWorkers, mpmcConvNumWorkers, mpmcConvOpsPerWorker)
				if len(errs) > 0 {
					t.Logf("caught bug in round %d: %s", r, errs[0])
					return
				}
			}
			t.Errorf("conservation test FAILED to catch known bug in %s after %d rounds — test may be too weak",
				name, maxRounds)
		})
	}
}
