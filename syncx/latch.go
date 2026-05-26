package syncx

import (
	"sync/atomic"
)

type SpinLatch struct {
	count atomic.Int32
}

func NewSpinLatch(n int32) *SpinLatch {
	if n <= 0 {
		panic("n should be positive")
	}
	l := &SpinLatch{}
	l.count.Store(n)
	return l
}

func (l *SpinLatch) Done() {
	l.count.Add(-1)
}

func (l *SpinLatch) Wait() {
	// Spin
	for l.count.Load() > 0 {
	}
}

type ChanLatch struct {
	count atomic.Int32
	done  chan struct{}
}

func NewChanLatch(n int32) *ChanLatch {
	if n <= 0 {
		panic("n should be positive")
	}

	l := &ChanLatch{
		done: make(chan struct{}),
	}

	l.count.Store(n)
	return l
}
func (l *ChanLatch) Done() {
	if l.count.Add(-1) == 0 {
		close(l.done)
	}
}

func (l *ChanLatch) Wait() {
	<-l.done
}

// SemaLatch is a CountDownLatch using runtime semaphore park (educational).
//
// Builds 1-to-N broadcast manually on top of runtime_Semrelease (which only
// wakes ONE goroutine at a time). The whole point is to feel the race that
// notifyList / channel close encapsulate away.
//
// Primitives available (already linkname'd in semaphore.go, just call them):
//
//	runtime_Semacquire(s *uint32)
//	runtime_Semrelease(s *uint32, handoff bool, skipframes int)
//
// State (recommended):
//
//	count   atomic.Int32   // counts down; latch opens at 0
//	waiters atomic.Int32   // # of waiters about to park or parked
//	sema    uint32         // shared parking slot for ALL waiters
//
// Done() sketch:
//
//	if count.Add(-1) == 0:
//	    n := waiters.Load()
//	    for i := 0; i < n; i++:
//	        runtime_Semrelease(&sema, false, 0)
//
// Wait() sketch:
//
//	if count.Load() == 0: return                 // fast path
//	waiters.Add(1)                                // register intent
//	if count.Load() == 0:                         // double-check
//	    ???                                       // ← YOU decide
//	runtime_Semacquire(&sema)
//
// The ??? race: between waiters.Add(1) and the second count.Load(), Done()
// may fire. If Done's waiters.Load saw your +1, a Semrelease is coming
// (you SHOULD Semacquire). If not, no Semrelease (you MUST NOT Semacquire,
// or deadlock).
//
// Two viable strategies for ???:
//
//	(1) Accept a bounded Semrelease leak — return without Semacquire.
//	    Cost: at most N leaked Semreleases in sema counter; harmless for
//	    one-shot latch, GC'd with the struct.
//	(2) Use sequential consistency of atomics to PROVE which case you're
//	    in by inspecting the order of count.Add / count.Load / waiters.Add
//	    / waiters.Load.
//
// This race is structurally the same as LockfreeSemaphore V1's CAS↔Enqueue
// window. It exists because broadcast is being faked with a 1-to-1 primitive.
type SemaLatch struct {
	// TODO: count atomic.Int32; waiters atomic.Int32; sema uint32
}

func NewSemaLatch(n int32) *SemaLatch {
	if n <= 0 {
		panic("n should be positive")
	}
	l := &SemaLatch{}
	return l
}

func (s *SemaLatch) Done() {
	// TODO
}

func (s *SemaLatch) Wait() {
	// TODO
}

// NotifyListLatch is a CountDownLatch using runtime notifyList (production-style).
//
// Same engine sync.Cond.Broadcast uses internally. Ticket-based registration
// eliminates the broadcast race that SemaLatch has to fight by hand.
//
// Primitives — add these go:linkname declarations to this file:
//
// And mirror the notifyList struct (Go 1.21+ layout — check your Go version
// in go.mod; layout has changed historically):
//
// State:
//
//	count atomic.Int32
//	nl    notifyList   // ★ embed BY VALUE — struct has internal pointers,
//	                   //   copying it splits the wait list and corrupts state
//
// Done() sketch:
//
//	if count.Add(-1) == 0:
//	    notifyListNotifyAll(&nl)
//
// Wait() sketch:
//
//	if count.Load() == 0: return                  // fast path
//	for count.Load() > 0:                          // loop handles spurious wake
//	    t := notifyListAdd(&nl)                    // register, get ticket
//	    if count.Load() == 0:                      // re-check after register
//	        notifyListWait(&nl, t)                 // returns fast (t < notify)
//	        return
//	    notifyListWait(&nl, t)                     // real park
//	    // loop body: re-check count after wake
//
// Why cleaner than SemaLatch:
//   - notifyListNotifyAll wakes ALL parked waiters in ONE call.
//     No "how many to wake?" problem.
//   - notifyListAdd returns a ticket — atomic registration WITHOUT parking.
//   - If NotifyAll fires between your Add and your Wait, your ticket is
//     already < notify, and Wait returns immediately. NO LOST WAKEUP.
//   - The broadcast race is encapsulated INSIDE notifyList.
//
// Open questions for YOU (think before coding):
//  1. Why embed nl by VALUE, not pointer? (Hint: sync.Cond's noCopy)
//  2. Should NotifyListLatch carry a copyChecker like sync.Cond? What's
//     the worst that happens if a user accidentally copies the struct?
//  3. Order of count.Add(-1) vs notifyListNotifyAll in Done() — does it
//     matter? (Recall release-after-prepare from Sense-Reversing Barrier.)
//  4. The Wait() for-loop re-checks count after wake. But count is
//     monotonic (latch is one-shot). Is the loop necessary? (Hint: runtime
//     can deliver spurious wakeups even without a NotifyAll.)
type NotifyListLatch struct {
	// TODO: count atomic.Int32; nl notifyList
}

func NewNotifyListLatch(n int32) *NotifyListLatch {
	if n <= 0 {
		panic("n should be positive")
	}
	l := &NotifyListLatch{}
	return l
}

func (l *NotifyListLatch) Done() {
	// TODO
}

func (l *NotifyListLatch) Wait() {
	// TODO
}

// WaitGroup is a fixed-N counting latch with a dynamic Add. Add sets or
// adjusts the counter, Done decrements, and Wait blocks until the counter
// reaches 0. State is packed into a single atomic.Uint64:
//
//	high 32 bits: counter      (Add / Done modify)
//	low  32 bits: waiter count (Wait modifies on park)
//
// One atomic CAS updates both halves at once, so registering as a waiter
// and reading the counter happen as a single observable step. Broadcast is
// N runtime_Semrelease calls when the counter hits 0.
//
// Pros:
//   - Single-word atomic state eliminates the broadcast race that any
//     two-counter design (e.g. SemaLatch) has to fight. The last Add(-1)
//     and any concurrent Wait registration are linearized by CAS on the
//     same word, so Add cannot miss a freshly-registered waiter.
//   - Zero allocations; zero-value usable, no constructor needed.
//   - Reusable: after broadcast, state resets to 0 and a new round can
//     begin (subject to the misuse rule below).
//   - Wait's fast path is a single Load — no CAS when counter is already 0.
//
// Cons:
//   - Counter capped at int32 max (~2 billion); waiter count capped at
//     uint32 max. Not practical limits, but worth noting.
//   - Broadcast is O(N) sequential Semrelease calls. notifyList-based
//     designs (sync.Cond.Broadcast) batch this into O(1) at the call site
//     by walking a runtime-side wait list.
//   - Reuse rule: all Adds for round k+1 must happen-after Wait of round k
//     returns. Violations are detected at runtime via panic, not statically.
//   - "Add concurrent with Wait" detection costs an extra Load on the
//     broadcast path.
//
// Mental model — three flows:
//
//	Add(delta):
//	    state.Add(delta << 32)
//	    split -> counter, waiters
//	    if counter < 0:                                 panic
//	    if waiters>0 && delta>0 && counter==delta:      panic (Add raced Wait)
//	    if counter > 0 || waiters == 0:                 return
//	    re-Load state, verify unchanged (else panic)
//	    Store(0); Semrelease N times
//
//	Done():
//	    Add(-1)
//
//	Wait():
//	    loop:
//	        Load state; if counter == 0: return         (fast path)
//	        CAS(state, state+1)                          (register self)
//	            success: Semacquire; verify state==0; return
//	            failure: retry
type WaitGroup struct {
	state atomic.Uint64
	sema  uint32
}

func (w *WaitGroup) Add(delta int) {
	state := w.state.Add(uint64(delta) << 32)
	counter := int32(state >> 32)
	waiters := uint32(state)

	if counter < 0 {
		panic("syncx: negative WaitGroup counter")
	}

	if waiters != 0 && delta > 0 && counter == int32(delta) {
		panic("syncx: WaitGroup misuse: Add called concurrently with Wait")
	}

	if counter > 0 || waiters == 0 {
		return
	}

	if w.state.Load() != state {
		panic("syncx: WaitGroup misuse: Add called concurrently with Wait")
	}

	w.state.Store(0)
	for ; waiters != 0; waiters-- {
		runtime_Semrelease(&w.sema, false, 0)
	}
}

func (w *WaitGroup) Done() {
	w.Add(-1)
}

func (w *WaitGroup) Wait() {
	for {
		state := w.state.Load()
		counter := int32(state >> 32)
		if counter == 0 {
			return
		}

		if w.state.CompareAndSwap(state, state+1) {
			runtime_Semacquire(&w.sema)

			if w.state.Load() != 0 {
				panic("syncx: WaitGroup is reused before previous Wait has returned")
			}
			return
		}
	}
}
