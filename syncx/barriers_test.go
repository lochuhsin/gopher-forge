package syncx

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testBarrier is the minimal one-shot barrier contract shared by all
// implementations in the family. Reusable barriers (sense-reversing,
// cyclic) satisfy this too — these tests only exercise the one-shot path.
type testBarrier interface {
	Wait()
}

var barrierImpls = []struct {
	name string
	new  func(n int32) testBarrier
}{
	{"Counting", func(n int32) testBarrier { return NewCountingBarrier(n) }},
	{"SenseReversing", func(n int32) testBarrier { return NewSenseReversingBarrier(n) }},
}

// Subset of barrierImpls that supports being reused across multiple rounds.
// CountingBarrier is excluded because its docstring marks it one-shot.
var reusableBarrierImpls = []struct {
	name string
	new  func(n int32) testBarrier
}{
	{"SenseReversing", func(n int32) testBarrier { return NewSenseReversingBarrier(n) }},
}

// Constructors are tested separately from `barrierImpls.new` because the
// panic/positive-size contract belongs to each concrete constructor —
// the factories above swallow the type information.
var barrierConstructors = []struct {
	name string
	call func(n int32)
}{
	{"Counting", func(n int32) { _ = NewCountingBarrier(n) }},
	{"SenseReversing", func(n int32) { _ = NewSenseReversingBarrier(n) }},
}

func TestBarrierNewPanicsOnNonPositiveSize(t *testing.T) {
	for _, ctor := range barrierConstructors {
		for _, c := range []struct {
			name string
			n    int32
		}{
			{"zero", 0},
			{"negative", -1},
			{"large_negative", -1000},
		} {
			t.Run(ctor.name+"/"+c.name, func(t *testing.T) {
				defer func() {
					if r := recover(); r == nil {
						t.Fatalf("%s(%d) did not panic", ctor.name, c.n)
					}
				}()
				ctor.call(c.n)
			})
		}
	}
}

func TestBarrierNewAcceptsPositiveSize(t *testing.T) {
	for _, ctor := range barrierConstructors {
		t.Run(ctor.name, func(t *testing.T) {
			for _, n := range []int32{1, 2, 8, 64} {
				ctor.call(n) // must not panic
			}
		})
	}
}

// n=1: a barrier of one is a no-op — Wait must return immediately.
func TestBarrierSinglePartyReleasesImmediately(t *testing.T) {
	for _, impl := range barrierImpls {
		t.Run(impl.name, func(t *testing.T) {
			b := impl.new(1)

			done := make(chan struct{})
			go func() {
				b.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("single-party barrier did not release")
			}
		})
	}
}

// Core contract — two halves:
//  1. N-1 arrivals: none of them may escape (the "no premature release" invariant).
//  2. Nth arrival:  all N must release.
func TestBarrierBlocksUntilAllArrive(t *testing.T) {
	for _, impl := range barrierImpls {
		t.Run(impl.name, func(t *testing.T) {
			const n int32 = 4
			b := impl.new(n)

			released := make(chan struct{}, n)
			for range n - 1 {
				go func() {
					b.Wait()
					released <- struct{}{}
				}()
			}

			// (1) None of the N-1 may escape before the Nth arrives.
			select {
			case <-released:
				t.Fatal("waiter released before all parties arrived")
			case <-time.After(50 * time.Millisecond):
			}

			// (2) Nth arrival → everyone goes.
			go func() {
				b.Wait()
				released <- struct{}{}
			}()

			done := make(chan struct{})
			go func() {
				for range n {
					<-released
				}
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatalf("only %d/%d released after Nth arrival", len(released), n)
			}
		})
	}
}

// All N goroutines start near-simultaneously through a `start` channel;
// the Nth-to-decrement is non-deterministic. Verifies the release fires
// exactly once regardless of arrival order.
func TestBarrierConcurrentArrivalAllReleased(t *testing.T) {
	for _, impl := range barrierImpls {
		t.Run(impl.name, func(t *testing.T) {
			const n int32 = 16
			b := impl.new(n)

			start := make(chan struct{})
			var released atomic.Int32
			var wg sync.WaitGroup
			for range n {
				wg.Go(func() {
					<-start
					b.Wait()
					released.Add(1)
				})
			}
			close(start)

			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("released %d/%d goroutines before timeout", released.Load(), n)
			}
			if got := released.Load(); got != n {
				t.Errorf("released = %d, want %d", got, n)
			}
		})
	}
}

// Pin down happens-before: every waiter must see writes that happened
// before *any* other waiter's Wait(). If the barrier lacks a release
// fence, this race detector run would flag (or values would be lost).
func TestBarrierEstablishesHappensBeforeOnRelease(t *testing.T) {
	for _, impl := range barrierImpls {
		t.Run(impl.name, func(t *testing.T) {
			const n int32 = 8
			b := impl.new(n)

			// Each goroutine writes its id into shared[i] *before* Wait,
			// then reads every other slot *after* Wait. With a correct
			// barrier, all writes are visible to all readers.
			shared := make([]int32, n)
			var wg sync.WaitGroup
			var mismatches atomic.Int32
			for i := range n {
				wg.Go(func() {
					shared[i] = i + 1 // write pre-barrier
					b.Wait()
					for j := range n {
						if shared[j] != j+1 {
							mismatches.Add(1)
						}
					}
				})
			}
			wg.Wait()

			if got := mismatches.Load(); got != 0 {
				t.Errorf("%d goroutines saw stale writes across the barrier", got)
			}
		})
	}
}

// Reusable barriers only: each round must be isolated. Within a round,
// no goroutine may exit Wait() until all N have entered it — even though
// the same barrier object served previous rounds.
//
// This is exactly the property sense-reversing was invented to provide:
// it forbids a slow goroutine from round R from being confused with a
// fast goroutine in round R+1 that already reset the counter.
func TestBarrierReusableEachRoundIsolated(t *testing.T) {
	for _, impl := range reusableBarrierImpls {
		t.Run(impl.name, func(t *testing.T) {
			const (
				n      int32 = 8
				rounds int32 = 20
			)
			b := impl.new(n)

			var earlyExits atomic.Int32

			for range rounds {
				var arrived atomic.Int32
				var wg sync.WaitGroup
				for range n {
					wg.Go(func() {
						arrived.Add(1)
						b.Wait()
						// Post-barrier: all N must have hit the counter
						// in *this* round. If <N, this goroutine escaped early.
						if arrived.Load() < n {
							earlyExits.Add(1)
						}
					})
				}
				wg.Wait()
			}

			if got := earlyExits.Load(); got != 0 {
				t.Errorf("%d goroutines exited Wait() before all N arrived in their round", got)
			}
		})
	}
}

// Reusable barriers only: stress the cross-round overlap. Fast goroutines
// race ahead into the next round while slow goroutines from the prior
// round may still be spinning. With sense reversing, the stale waiter sees
// the changed `release` flag and exits cleanly; without it, the reset
// counter would race against the spin check.
func TestBarrierReusableManyRoundsRace(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped under -short")
	}
	for _, impl := range reusableBarrierImpls {
		t.Run(impl.name, func(t *testing.T) {
			const (
				n      int32 = 8
				rounds       = 500
			)
			b := impl.new(n)

			var wg sync.WaitGroup
			for range n {
				wg.Go(func() {
					for range rounds {
						b.Wait()
					}
				})
			}

			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()
			select {
			case <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("barrier deadlocked across rounds")
			}
		})
	}
}

// Stress under -race: many fresh barriers, many goroutines each.
// Each iteration uses a NEW barrier because the centralized variant
// is not reusable.
func TestBarrierStressRace(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped under -short")
	}
	for _, impl := range barrierImpls {
		t.Run(impl.name, func(t *testing.T) {
			const n int32 = 16
			const runs = 200

			for range runs {
				b := impl.new(n)

				var wg sync.WaitGroup
				for range n {
					wg.Go(func() {
						b.Wait()
					})
				}

				done := make(chan struct{})
				go func() {
					wg.Wait()
					close(done)
				}()

				select {
				case <-done:
				case <-time.After(5 * time.Second):
					t.Fatal("barrier deadlocked under stress")
				}
			}
		})
	}
}
