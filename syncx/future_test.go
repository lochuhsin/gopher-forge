package syncx

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Future contract (a write-once, read-many result cell):
//
//   - NewFuture(fn) starts computing fn() in the background and returns
//     immediately, without blocking the caller.
//   - Get() returns the value produced by fn().
//   - If Get() is called before fn() has finished, it BLOCKS until the
//     result is ready (it must not return a zero/partial value early).
//   - Once resolved, Get() returns immediately on every later call
//     (fast path), and is non-destructive: repeated Gets all see the value.
//   - Get() is safe to call from many goroutines at once; ALL of them must
//     receive the result — a Future is "compute once, broadcast to all",
//     not "hand the result to one waiter".
//   - fn() runs exactly once regardless of how many times Get() is called.
//   - Get() happens-after fn() returns: every write fn made before returning
//     is visible to the Get caller, with no data race (verify under -race).

// getWithin runs f.Get() in a separate goroutine and fails if it does not
// return within d. Without it, a Future whose parking/broadcast is broken
// would block the test goroutine forever and we'd only learn about it from
// the package-wide test timeout — with no indication of which test hung.
// The buffered channel lets the spawned goroutine finish its send even if we
// already timed out, so a slow-but-eventually-correct Get doesn't leak a
// blocked-on-send goroutine.
func getWithin[T any](t *testing.T, f *DemoFuture[T], d time.Duration) T {
	t.Helper()
	var v T
	var err error
	done := make(chan struct{}, 1)
	go func() {
		v, err = f.Get()
		done <- struct{}{}
	}()
	select {
	case <-done:
		if err != nil {
			t.Fatalf("Get returned unexpected error: %v", err)
		}
		return v
	case <-time.After(d):
		t.Fatalf("Get did not return within %v", d)
		var zero T
		return zero
	}
}

// Happy path: the value handed back is exactly what fn computed.
func TestFutureReturnsResult(t *testing.T) {
	f := NewDemoFuture(func() (int, error) { return 42, nil })
	if got := getWithin(t, f, time.Second); got != 42 {
		t.Fatalf("Get() = %d, want 42", got)
	}
}

// Core blocking contract, in two halves:
//  1. While fn is still running, Get MUST stay blocked (no early zero value).
//  2. Once fn returns, Get MUST release with the computed value.
//
// fn is held hostage on a channel so the test controls exactly when the
// result becomes available. This is the test that fails if Get does not
// actually wait for the producer (e.g. if it deposits a permit and returns
// instead of consuming one and sleeping).
func TestFutureGetBlocksUntilResolved(t *testing.T) {
	release := make(chan struct{})
	f := NewDemoFuture(func() (int, error) {
		<-release
		return 7, nil
	})

	got := make(chan int, 1)
	go func() { v, _ := f.Get(); got <- v }()

	// (1) fn not released yet → Get must still be blocked.
	select {
	case v := <-got:
		t.Fatalf("Get returned %d before fn completed", v)
	case <-time.After(50 * time.Millisecond):
	}

	// (2) release fn → Get must now return the real value.
	close(release)
	select {
	case v := <-got:
		if v != 7 {
			t.Fatalf("Get() = %d, want 7", v)
		}
	case <-time.After(time.Second):
		t.Fatal("Get did not return after fn completed")
	}
}

// Non-destructive + fast path: the first Get may park, but once resolved the
// result stays available — every later Get returns the same value without
// re-running fn or consuming the result.
func TestFutureRepeatedGetsSameValue(t *testing.T) {
	f := NewDemoFuture(func() (int, error) { return 99, nil })
	for i := range 5 {
		if got := getWithin(t, f, time.Second); got != 99 {
			t.Fatalf("Get() #%d = %d, want 99", i, got)
		}
	}
}

// Broadcast property: many goroutines call Get BEFORE the result is ready, so
// every one of them parks. A correct Future wakes ALL of them when fn
// resolves. A design that can only wake a single waiter (one Unpark with no
// cascade, or any 1-to-1 wakeup faking a 1-to-N broadcast) leaves the rest
// parked forever — caught here as a timeout.
func TestFutureConcurrentGets(t *testing.T) {
	const readers = 50
	release := make(chan struct{})
	f := NewDemoFuture(func() (int, error) {
		<-release
		return 1234, nil
	})

	var wg sync.WaitGroup
	var bad atomic.Int32
	start := make(chan struct{})
	for range readers {
		wg.Go(func() {
			<-start
			if v, err := f.Get(); err != nil || v != 1234 {
				bad.Add(1)
			}
		})
	}
	close(start)

	// Let the readers actually reach Get and park before the result lands,
	// so the single-waiter failure mode is exercised deterministically rather
	// than hidden by readers taking the fast path.
	time.Sleep(20 * time.Millisecond)
	close(release)

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("not every concurrent Get returned — broadcast/parking is broken")
	}
	if n := bad.Load(); n != 0 {
		t.Fatalf("%d readers observed the wrong value", n)
	}
}

// fn must execute exactly once no matter how many readers there are: a Future
// computes once and serves the cached result to all.
func TestFutureRunsFnOnce(t *testing.T) {
	var calls atomic.Int32
	f := NewDemoFuture(func() (int, error) {
		calls.Add(1)
		return 0, nil
	})

	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() { f.Get() })
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("fn ran %d times, want exactly 1", got)
	}
}

// Publication / happens-before: fn fills a slice with plain (non-atomic)
// writes and returns it; readers obtain it ONLY through Get. If Get correctly
// synchronizes with the producer (atomic state load = acquire, or Park =
// acquire, pairing with the producer's release), every element is fully
// written and visible. Run under -race to catch a Future that reads f.v
// without a synchronizing edge.
func TestFutureEstablishesHappensBefore(t *testing.T) {
	const n = 64
	f := NewDemoFuture(func() ([]int, error) {
		s := make([]int, n)
		for i := range n {
			s[i] = i + 1
		}
		return s, nil
	})

	var wg sync.WaitGroup
	var mismatches atomic.Int32
	for range 8 {
		wg.Go(func() {
			s, _ := f.Get()
			for i := range n {
				if s[i] != i+1 {
					mismatches.Add(1)
				}
			}
		})
	}
	wg.Wait()

	if m := mismatches.Load(); m != 0 {
		t.Errorf("%d stale/torn reads — Get does not happen-after fn", m)
	}
}

// Repeated rounds with many concurrent readers, to shake out lost-wakeup and
// deadlock races that a single run won't reliably hit. Run with -race.
func TestFutureStressRace(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped under -short")
	}
	const (
		readers = 16
		runs    = 200
	)
	for r := range runs {
		want := r * 7
		release := make(chan struct{})
		f := NewDemoFuture(func() (int, error) {
			<-release
			return want, nil
		})

		var wg sync.WaitGroup
		var bad atomic.Int32
		for range readers {
			wg.Go(func() {
				if v, err := f.Get(); err != nil || v != want {
					bad.Add(1)
				}
			})
		}
		close(release)

		done := make(chan struct{})
		go func() { wg.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("run %d: Future deadlocked under stress", r)
		}
		if n := bad.Load(); n != 0 {
			t.Fatalf("run %d: %d readers saw the wrong value", r, n)
		}
	}
}
