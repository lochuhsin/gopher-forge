package syncx

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ---------- Once ----------
//
// Contract:
//  1. f is invoked at most once across all Do calls on the same Once.
//  2. After the winning f returns (or panics), every later Do returns
//     without invoking f.
//  3. While the winning f is still running, other concurrent Do callers
//     MUST block until f completes — they cannot return early. This is
//     the publication-safety property that separates Once from a plain
//     CAS flag.
//  4. The completion of f happens-before the return of any later Do —
//     writes inside f are visible to any subsequent observer.
//  5. Panic inside f still marks the Once as done (sync.Once semantics)
//     and must release the internal mutex.

func TestOnceCallsExactlyOnce(t *testing.T) {
	var o Once
	var calls atomic.Int32

	const N = 64
	var wg sync.WaitGroup
	for range N {
		wg.Go(func() {
			o.Do(func() { calls.Add(1) })
		})
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("f invoked %d times, want 1", got)
	}
}

func TestOnceLateCallerReturnsImmediately(t *testing.T) {
	var o Once
	o.Do(func() {})

	done := make(chan struct{})
	go func() {
		o.Do(func() { t.Error("late Do invoked f") })
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("late Do did not return")
	}
}

// Publication safety — the key non-trivial property. While the winning
// goroutine is still inside f, every other Do caller must block on the
// mutex rather than returning early under the assumption "someone else
// is handling it".
func TestOnceConcurrentCallersBlockUntilFCompletes(t *testing.T) {
	var o Once
	var calls atomic.Int32

	fEntered := make(chan struct{})
	gate := make(chan struct{})

	const followers = 8
	returned := make(chan struct{}, followers+1)

	// Winner: enters f, signals, then parks on gate.
	go func() {
		o.Do(func() {
			calls.Add(1)
			close(fEntered)
			<-gate
		})
		returned <- struct{}{}
	}()

	<-fEntered

	// Followers: must all block while gate is closed.
	for range followers {
		go func() {
			o.Do(func() { calls.Add(1) })
			returned <- struct{}{}
		}()
	}

	// Negative half: nobody may return while f is still parked.
	select {
	case <-returned:
		t.Fatal("Do returned while winning f was still running")
	case <-time.After(50 * time.Millisecond):
	}

	close(gate)

	// Positive half: every Do returns; f ran exactly once.
	for i := range followers + 1 {
		select {
		case <-returned:
		case <-time.After(time.Second):
			t.Fatalf("only %d/%d Do calls returned after f finished", i, followers+1)
		}
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("f invoked %d times, want 1", got)
	}
}

// Run under -race. Writes inside f must be visible to any goroutine
// whose Do returned after the winner — the atomic.Store/Load on state
// (or the mutex on the slow path) is what publishes them.
func TestOnceEstablishesHappensBefore(t *testing.T) {
	var o Once
	const slots = 64
	shared := make([]int32, slots)

	const observers = 32
	var wg sync.WaitGroup
	for range observers {
		wg.Go(func() {
			o.Do(func() {
				for i := range shared {
					shared[i] = int32(i + 1)
				}
			})
			for i, v := range shared {
				if v != int32(i+1) {
					t.Errorf("slot %d = %d, want %d (no HB with winning f)", i, v, i+1)
					return
				}
			}
		})
	}
	wg.Wait()
}

// Panic in f marks Once as done (sync.Once semantics): the next Do does
// not invoke its f. Implicitly verifies the mutex is released — if the
// panic had left it locked, the second Do would deadlock and the
// timeout would fire.
func TestOncePanicStillMarksAsDone(t *testing.T) {
	var o Once
	var calls atomic.Int32

	func() {
		defer func() { _ = recover() }()
		o.Do(func() {
			calls.Add(1)
			panic("boom")
		})
	}()

	done := make(chan struct{})
	go func() {
		o.Do(func() { calls.Add(1) })
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("second Do deadlocked — panic likely left mutex locked")
	}

	if got := calls.Load(); got != 1 {
		t.Fatalf("f invoked %d times, want 1", got)
	}
}

func TestOnceStressRace(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped under -short")
	}
	const (
		runs       = 200
		goroutines = 32
	)
	for range runs {
		var o Once
		var calls atomic.Int32

		var wg sync.WaitGroup
		for range goroutines {
			wg.Go(func() {
				o.Do(func() { calls.Add(1) })
			})
		}
		wg.Wait()

		if got := calls.Load(); got != 1 {
			t.Fatalf("f invoked %d times, want 1", got)
		}
	}
}

// ---------- OnceCell[T] ----------
//
// Contract: Do(f) returns f()'s value on the first call; every later Do
// returns the same memoized value without invoking its f.

func TestOnceCellReturnsFirstValue(t *testing.T) {
	var c OnceCell[int]

	if got := c.Do(func() int { return 7 }); got != 7 {
		t.Fatalf("first Do = %d, want 7", got)
	}
	if got := c.Do(func() int {
		t.Error("second f invoked")
		return 99
	}); got != 7 {
		t.Fatalf("second Do = %d, want 7 (memoized)", got)
	}
}

func TestOnceCellConcurrentSameValue(t *testing.T) {
	var c OnceCell[int]
	var calls atomic.Int32

	const N = 64
	results := make([]int, N)
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			results[i] = c.Do(func() int {
				calls.Add(1)
				return 42
			})
		})
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("f invoked %d times, want 1", got)
	}
	for i, v := range results {
		if v != 42 {
			t.Fatalf("goroutine %d saw %d, want 42", i, v)
		}
	}
}

// ---------- OnceCells[T, K] ----------
//
// Contract: 2-tuple variant of OnceCell. The pair from the first f is
// memoized in full; f is invoked at most once.

func TestOnceCellsReturnsFirstPair(t *testing.T) {
	var c OnceCells[int, string]

	v, s := c.Do(func() (int, string) { return 7, "ok" })
	if v != 7 || s != "ok" {
		t.Fatalf("first Do = (%d, %q), want (7, \"ok\")", v, s)
	}

	v, s = c.Do(func() (int, string) {
		t.Error("second f invoked")
		return 99, "no"
	})
	if v != 7 || s != "ok" {
		t.Fatalf("second Do = (%d, %q), want (7, \"ok\") (memoized)", v, s)
	}
}

func TestOnceCellsConcurrentSamePair(t *testing.T) {
	var c OnceCells[int, string]
	var calls atomic.Int32

	const N = 64
	type pair struct {
		v int
		s string
	}
	results := make([]pair, N)
	var wg sync.WaitGroup
	for i := range N {
		wg.Go(func() {
			v, s := c.Do(func() (int, string) {
				calls.Add(1)
				return 42, "hello"
			})
			results[i] = pair{v, s}
		})
	}
	wg.Wait()

	if got := calls.Load(); got != 1 {
		t.Fatalf("f invoked %d times, want 1", got)
	}
	for i, r := range results {
		if r.v != 42 || r.s != "hello" {
			t.Fatalf("goroutine %d saw (%d, %q), want (42, \"hello\")", i, r.v, r.s)
		}
	}
}

// Pins down the (T, error) use case: error identity must round-trip so
// callers can use errors.Is on the memoized error.
func TestOnceCellsWorksWithError(t *testing.T) {
	errFoo := errors.New("foo")
	var c OnceCells[int, error]

	v, err := c.Do(func() (int, error) { return 0, errFoo })
	if v != 0 || !errors.Is(err, errFoo) {
		t.Fatalf("first Do = (%d, %v), want (0, errFoo)", v, err)
	}

	v, err = c.Do(func() (int, error) {
		t.Error("second f invoked")
		return 1, nil
	})
	if v != 0 || !errors.Is(err, errFoo) {
		t.Fatalf("second Do = (%d, %v), want (0, errFoo) (memoized)", v, err)
	}
}
