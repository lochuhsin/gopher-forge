package syncx

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// testCountDown is the shared contract for CountDownLatch-style primitives:
// constructed with an initial count, Done() decrements, Wait() blocks until
// the count reaches zero, then releases all current and future waiters.
type testCountDown interface {
	Done()
	Wait()
}

var countDownImpls = []struct {
	name string
	new  func(n int32) testCountDown
}{
	{"Spin", func(n int32) testCountDown { return NewSpinLatch(n) }},
	{"Chan", func(n int32) testCountDown { return NewChanLatch(n) }},
	{"Sema", func(n int32) testCountDown { return NewSemaLatch(n) }},
	{"NotifyList", func(n int32) testCountDown { return NewNotifyListLatch(n) }},
}

// Constructors are tested separately because the panic/positive-size contract
// belongs to each concrete constructor — the factories above swallow it.
var countDownConstructors = []struct {
	name string
	call func(n int32)
}{
	{"Spin", func(n int32) { _ = NewSpinLatch(n) }},
	{"Chan", func(n int32) { _ = NewChanLatch(n) }},
	{"Sema", func(n int32) { _ = NewSemaLatch(n) }},
	{"NotifyList", func(n int32) { _ = NewNotifyListLatch(n) }},
}

func TestLatchNewPanicsOnNonPositiveSize(t *testing.T) {
	for _, ctor := range countDownConstructors {
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

func TestLatchNewAcceptsPositiveSize(t *testing.T) {
	for _, ctor := range countDownConstructors {
		t.Run(ctor.name, func(t *testing.T) {
			for _, n := range []int32{1, 2, 8, 64} {
				ctor.call(n) // must not panic
			}
		})
	}
}

// n=1: a single Done releases Wait. Also pins down the negative half — Wait
// must NOT return before that Done is observed.
func TestLatchSingleDoneReleases(t *testing.T) {
	for _, impl := range countDownImpls {
		t.Run(impl.name, func(t *testing.T) {
			l := impl.new(1)

			released := make(chan struct{})
			go func() {
				l.Wait()
				close(released)
			}()

			select {
			case <-released:
				t.Fatal("Wait returned before any Done")
			case <-time.After(50 * time.Millisecond):
			}

			l.Done()

			select {
			case <-released:
			case <-time.After(time.Second):
				t.Fatal("Wait did not return after Done")
			}
		})
	}
}

// Core contract — two halves:
//  1. N-1 Dones: Wait must still block (no premature release).
//  2. Nth Done:  Wait must release.
func TestLatchBlocksUntilCountZero(t *testing.T) {
	for _, impl := range countDownImpls {
		t.Run(impl.name, func(t *testing.T) {
			const n int32 = 4
			l := impl.new(n)

			released := make(chan struct{})
			go func() {
				l.Wait()
				close(released)
			}()

			for range n - 1 {
				l.Done()
			}

			// (1) N-1 Dones must not be enough.
			select {
			case <-released:
				t.Fatal("Wait returned before counter reached zero")
			case <-time.After(50 * time.Millisecond):
			}

			// (2) Nth Done → Wait returns.
			l.Done()
			select {
			case <-released:
			case <-time.After(time.Second):
				t.Fatal("Wait did not return after Nth Done")
			}
		})
	}
}

// Broadcast property: every waiter currently parked must wake when the
// counter hits zero — not just one of them. A latch is fundamentally
// "fire once, wake all", which distinguishes it from a semaphore.
func TestLatchMultipleWaitersAllReleased(t *testing.T) {
	for _, impl := range countDownImpls {
		t.Run(impl.name, func(t *testing.T) {
			const (
				n       int32 = 3
				waiters       = 10
			)
			l := impl.new(n)

			var released atomic.Int32
			var wg sync.WaitGroup
			for range waiters {
				wg.Go(func() {
					l.Wait()
					released.Add(1)
				})
			}

			for range n {
				l.Done()
			}

			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("only %d/%d waiters released", released.Load(), waiters)
			}
		})
	}
}

// Publication safety: once the latch has fired, any LATER Wait() must
// return immediately. This is the property that lets a latch act as a
// one-shot "is X ready?" gate that newly-spawned goroutines can poll
// without coordinating with the original waiters.
func TestLatchLateWaitReturnsImmediately(t *testing.T) {
	for _, impl := range countDownImpls {
		t.Run(impl.name, func(t *testing.T) {
			const n int32 = 4
			l := impl.new(n)

			for range n {
				l.Done()
			}

			done := make(chan struct{})
			go func() {
				l.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("late Wait did not return immediately after counter zero")
			}
		})
	}
}

// All N Done()s fire concurrently from distinct goroutines. The counter
// must reach zero exactly once and Wait must release exactly then — even
// though it is nondeterministic which Done observes the zero transition.
// This is the test that exposes a race in the "Done && counter==0 ⇒ close"
// path: only the goroutine that drives the counter to zero may close the
// channel.
func TestLatchConcurrentDones(t *testing.T) {
	for _, impl := range countDownImpls {
		t.Run(impl.name, func(t *testing.T) {
			const n int32 = 32
			l := impl.new(n)

			released := make(chan struct{})
			go func() {
				l.Wait()
				close(released)
			}()

			start := make(chan struct{})
			var wg sync.WaitGroup
			for range n {
				wg.Go(func() {
					<-start
					l.Done()
				})
			}
			close(start)
			wg.Wait()

			select {
			case <-released:
			case <-time.After(time.Second):
				t.Fatal("Wait did not return after all concurrent Dones")
			}
		})
	}
}

// Each goroutine writes its slot BEFORE Done(); the observer reads every
// slot AFTER Wait() returns. With a correct latch, Done is a release and
// Wait is an acquire — all writes must be visible. Run under -race to
// catch torn synchronization.
func TestLatchEstablishesHappensBefore(t *testing.T) {
	for _, impl := range countDownImpls {
		t.Run(impl.name, func(t *testing.T) {
			const n int32 = 8
			l := impl.new(n)

			shared := make([]int32, n)
			var wg sync.WaitGroup
			for i := range n {
				wg.Go(func() {
					shared[i] = i + 1 // write pre-Done
					l.Done()
				})
			}

			l.Wait()

			var mismatches int
			for j := range n {
				if shared[j] != j+1 {
					mismatches++
				}
			}
			wg.Wait()

			if mismatches != 0 {
				t.Errorf("Wait saw %d stale slots — no happens-before with Done", mismatches)
			}
		})
	}
}

func TestLatchStressRace(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped under -short")
	}
	for _, impl := range countDownImpls {
		t.Run(impl.name, func(t *testing.T) {
			const (
				n    int32 = 16
				runs       = 200
			)
			for range runs {
				l := impl.new(n)

				released := make(chan struct{})
				go func() {
					l.Wait()
					close(released)
				}()

				var wg sync.WaitGroup
				for range n {
					wg.Go(func() {
						l.Done()
					})
				}
				wg.Wait()

				select {
				case <-released:
				case <-time.After(5 * time.Second):
					t.Fatal("latch deadlocked under stress")
				}
			}
		})
	}
}

// --- WaitGroup (self-rolled, teaching variant) ---
//
// Different API from the CountDown family (Add + Done + Wait), so it gets a
// dedicated test set. Contract follows Go's sync.WaitGroup: Add(delta) must
// happen-before Wait(); Done is Add(-1); when the counter transitions to
// zero, all parked Wait()s release; the WaitGroup is reusable as long as a
// new Add(>0) happens after the previous Wait() has fully returned.

func TestWaitGroupBasicAddDoneWait(t *testing.T) {
	var wg WaitGroup
	wg.Add(3)

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("Wait returned before any Done")
	case <-time.After(50 * time.Millisecond):
	}

	wg.Done()
	wg.Done()

	select {
	case <-done:
		t.Fatal("Wait returned with counter still > 0")
	case <-time.After(50 * time.Millisecond):
	}

	wg.Done()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Wait did not return after final Done")
	}
}

func TestWaitGroupMultipleWaitersAllRelease(t *testing.T) {
	var wg WaitGroup
	wg.Add(2)

	const waiters = 10
	var released atomic.Int32
	var ww sync.WaitGroup
	for range waiters {
		ww.Go(func() {
			wg.Wait()
			released.Add(1)
		})
	}

	wg.Done()
	wg.Done()

	done := make(chan struct{})
	go func() {
		ww.Wait()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("only %d/%d waiters released", released.Load(), waiters)
	}
}

// Reusable across rounds: after Wait returns, a fresh Add must start a new
// generation cleanly. This is what makes WaitGroup useful for repeated
// fork/join (worker pool, batch processing) — unlike a one-shot latch.
func TestWaitGroupReusable(t *testing.T) {
	var wg WaitGroup
	for range 5 {
		wg.Add(2)
		go wg.Done()
		go wg.Done()
		wg.Wait()
	}
}

func TestWaitGroupStressRace(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped under -short")
	}
	const (
		n    = 16
		runs = 200
	)
	for range runs {
		var wg WaitGroup
		wg.Add(n)

		for range n {
			go wg.Done()
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("WaitGroup deadlocked under stress")
		}
	}
}
