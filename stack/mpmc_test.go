package stack

import (
	"fmt"
	"runtime"
	"sync"
	"testing"
)

const (
	convNumWorkers    = 8
	convPushPerWorker = 100
)

// Chaos uses a larger workload across multiple rounds and GOMAXPROCS
// settings; race bugs surface probabilistically.
const (
	chaosNumWorkers    = 16
	chaosPushPerWorker = 200
	chaosRounds        = 30
)

func chaosGOMAXPROCS() []int { return []int{1, 2, 4, 8} }

// runConservation runs numWorkers pushers (each pushing a disjoint
// integer range) concurrent with numWorkers poppers, then verifies that
// every pushed value appears exactly once across (popped ∪ remaining).
func runConservation(s Stack[int], numWorkers, pushPerWorker int) []string {
	var pushWg, popWg sync.WaitGroup
	pushDone := make(chan struct{})

	for w := range numWorkers {
		pushWg.Add(1)
		go func(id int) {
			defer pushWg.Done()
			base := id * pushPerWorker
			for i := range pushPerWorker {
				s.Push(base + i)
			}
		}(w)
	}

	poppedCh := make(chan []int, numWorkers)
	for range numWorkers {
		popWg.Add(1)
		go func() {
			defer popWg.Done()
			local := []int{}
			for {
				if v, ok := s.Pop(); ok {
					local = append(local, v)
					continue
				}
				// Only stop after all pushers have finished — otherwise we
				// might exit before a slow pusher's value lands.
				select {
				case <-pushDone:
					poppedCh <- local
					return
				default:
					runtime.Gosched()
				}
			}
		}()
	}

	pushWg.Wait()
	close(pushDone)
	popWg.Wait()
	close(poppedCh)

	// A correct stack should be empty here; buggy ones may have leftovers.
	remaining := []int{}
	for {
		v, ok := s.Pop()
		if !ok {
			break
		}
		remaining = append(remaining, v)
	}

	popped := []int{}
	for l := range poppedCh {
		popped = append(popped, l...)
	}

	totalExpected := numWorkers * pushPerWorker
	counts := make(map[int]int, totalExpected)
	for _, v := range popped {
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

func TestStackEmpty(t *testing.T) {
	for name, factory := range stackFactories {
		t.Run(name, func(t *testing.T) {
			s := factory()
			v, ok := s.Pop()
			if ok {
				t.Errorf("Pop on empty: got (%d, true), want (_, false)", v)
			}
		})
	}
}

func TestSequentialLIFO(t *testing.T) {
	for name, factory := range stackFactories {
		t.Run(name, func(t *testing.T) {
			s := factory()
			const n = 10
			for i := range n {
				s.Push(i)
			}
			for i := n - 1; i >= 0; i-- {
				v, ok := s.Pop()
				if !ok {
					t.Fatalf("Pop returned ok=false at i=%d", i)
				}
				if v != i {
					t.Errorf("Pop = %d, want %d", v, i)
				}
			}
			if v, ok := s.Pop(); ok {
				t.Errorf("expected empty, got (%d, true)", v)
			}
		})
	}
}

func TestConcurrentPushOnly(t *testing.T) {
	for name, factory := range stackFactories {
		t.Run(name, func(t *testing.T) {
			s := factory()
			var wg sync.WaitGroup
			for w := range convNumWorkers {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					base := id * convPushPerWorker
					for i := range convPushPerWorker {
						s.Push(base + i)
					}
				}(w)
			}
			wg.Wait()

			total := convNumWorkers * convPushPerWorker
			seen := make(map[int]int, total)
			for {
				v, ok := s.Pop()
				if !ok {
					break
				}
				seen[v]++
			}
			verifyMultiset(t, seen, total)
		})
	}
}

func TestConcurrentPopOnly(t *testing.T) {
	for name, factory := range stackFactories {
		t.Run(name, func(t *testing.T) {
			s := factory()
			total := convNumWorkers * convPushPerWorker
			for i := range total {
				s.Push(i)
			}

			var wg sync.WaitGroup
			poppedCh := make(chan []int, convNumWorkers)
			for range convNumWorkers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					local := []int{}
					for {
						v, ok := s.Pop()
						if !ok {
							poppedCh <- local
							return
						}
						local = append(local, v)
					}
				}()
			}
			wg.Wait()
			close(poppedCh)

			seen := make(map[int]int, total)
			for l := range poppedCh {
				for _, v := range l {
					seen[v]++
				}
			}
			verifyMultiset(t, seen, total)
		})
	}
}

func TestConcurrentPushPop(t *testing.T) {
	for name, factory := range stackFactories {
		t.Run(name, func(t *testing.T) {
			s := factory()
			errs := runConservation(s, convNumWorkers, convPushPerWorker)
			for _, e := range errs {
				t.Error(e)
			}
		})
	}
}

func TestConcurrentChaos(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping chaos test in -short mode")
	}
	for _, procs := range chaosGOMAXPROCS() {
		t.Run(fmt.Sprintf("GOMAXPROCS=%d", procs), func(t *testing.T) {
			prev := runtime.GOMAXPROCS(procs)
			t.Cleanup(func() { runtime.GOMAXPROCS(prev) })

			for name, factory := range stackFactories {
				t.Run(name, func(t *testing.T) {
					for r := range chaosRounds {
						s := factory()
						errs := runConservation(s, chaosNumWorkers, chaosPushPerWorker)
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

// Meta-test: verify the conservation check actually detects known bugs.
func TestConservation_CatchesBugs(t *testing.T) {
	const maxRounds = 30
	for name, factory := range buggyStackFactories {
		t.Run(name, func(t *testing.T) {
			for r := range maxRounds {
				s := factory()
				errs := runConservation(s, convNumWorkers, convPushPerWorker)
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

func verifyMultiset(t *testing.T, seen map[int]int, totalExpected int) {
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
