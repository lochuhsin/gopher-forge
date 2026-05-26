package syncx

import (
	"forge/queue"
	"sync"
	"sync/atomic"
	_ "unsafe"
)

// Semaphore controls concurrent access by issuing at most N permits.
//
//   - Acquire blocks until a permit is available.
//   - Release returns immediately and wakes one waiter (if any).
//   - Calling Release without a prior Acquire panics.
//   - Permits are interchangeable across goroutines (one goroutine may
//     Acquire and another may Release).
type Semaphore interface {
	Acquire()
	Release()
}

// ChannelSemaphore is a counting semaphore built on a buffered channel of
// capacity N. Acquire sends a sentinel into the channel (blocking when full);
// Release receives one out (blocking when empty).
//
// Pattern: "channel as semaphore" — the idiomatic Go primitive. A buffered
// channel of capacity N is operationally equivalent to a counting semaphore
// initialized to N: the channel's internal hchan already contains a mutex and
// FIFO sudog queues for senders/receivers, so the runtime handles the parking
// and waking for free.
//
// Pros:
//   - Idiomatic Go, trivial to read and review.
//   - Composes with select for free: timeouts via time.After, cancellation via
//     context.Done(), TryAcquire via a default branch.
//   - Backed by the runtime's optimized chan (no spin, FIFO sudog queue).
//
// Cons:
//   - Heaviest of the implementations: every Acquire/Release crosses the
//     channel's internal mutex and pays for a sudog on the slow path.
//   - The send=Acquire direction reads backwards from the usual semaphore
//     mental model. An alternative idiom is to pre-fill the channel and use
//     receive=Acquire / send=Release, which reads more naturally.
type ChannelSemaphore struct {
	sem chan struct{}
}

func NewChannelSemaphore(init int) *ChannelSemaphore {
	return &ChannelSemaphore{
		sem: make(chan struct{}, init),
	}
}

func (c *ChannelSemaphore) Acquire() {
	c.sem <- struct{}{}
}

func (c *ChannelSemaphore) Release() {
	<-c.sem
}

// MutexSemaphore guards a permit counter with a mutex and parks blocked
// waiters by handing each one a personal "gate" mutex that is pre-locked. To
// block: create a Mutex, Lock it (uncontended, fast), enqueue it, then Lock it
// again — the second Lock blocks until somebody else Unlocks the gate.
//
// Pattern: "hand-off via pre-locked mutex." This is how you build a
// park/unpark primitive without sync.Cond or runtime help — locking an
// already-locked mutex parks the goroutine, and another goroutine unlocks it
// to wake exactly one waiter, morally equivalent to futex_wait/futex_wake.
// Release here also does *direct handoff*: it does NOT decrement the counter
// when it wakes a waiter; the slot is passed straight from Releaser to
// dequeued waiter, so a newly-arriving Acquirer cannot barge in front of an
// already-queued waiter.
//
// Pros:
//   - Strict FIFO fairness — waiters are unblocked in enqueue order.
//   - No spinning; the underlying mutex parks blocked goroutines.
//   - Direct handoff eliminates the wake-then-lose-race / thundering-herd
//     issue that plagues notify-based designs (see CondSemaphore below).
//
// Cons:
//   - One sync.Mutex allocated per blocked waiter on the slow path.
//   - The waiters slice grows with append + reslice; the backing array's
//     capacity never shrinks, so worst-case memory persists for the life of
//     the semaphore (the TODO already calls this out — a ring buffer or
//     container/list avoids the leak).
//   - More moving parts than the channel- or cond-based designs.
type MutexSemaphore struct {
	mu      sync.Mutex
	waiters []*sync.Mutex // memory leak, probably use ring buffer
	count   int
	max     int
}

func NewMutexSemaphore(init int) *MutexSemaphore {
	return &MutexSemaphore{max: init}
}

func (m *MutexSemaphore) Acquire() {
	m.mu.Lock()

	if m.count < m.max {
		m.count++
		m.mu.Unlock()
		return
	}
	gate := &sync.Mutex{}
	gate.Lock()

	m.waiters = append(m.waiters, gate)

	m.mu.Unlock()
	gate.Lock()
}

func (m *MutexSemaphore) Release() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.count == 0 {
		panic("syncx: release without acquire")
	}

	if len(m.waiters) > 0 {
		gate := m.waiters[0]
		m.waiters[0] = nil // gc collect
		m.waiters = m.waiters[1:]
		gate.Unlock()
		return
	}
	m.count--
}

// CondSemaphore is the textbook monitor: one mutex guards the count, one
// sync.Cond parks and wakes blocked waiters.
//
// Pattern: "condition variable with while-loop predicate." The canonical
// cond.Wait usage — ALWAYS inside a `for predicate` loop, never an `if` —
// because (a) Signal/Broadcast does not guarantee the predicate still holds
// when the woken waiter runs (another waiter may have consumed the permit
// first), and (b) spurious wakeups are allowed by the spec. The loop re-checks
// the predicate under the mutex after each wake.
//
// Pros:
//   - Standard monitor pattern that maps cleanly to any language with
//     mutex+condvar (pthread_cond, Java Object.wait, etc.).
//   - Signal wakes exactly one waiter — no thundering herd from Broadcast.
//   - Code is short and the invariant is obvious.
//
// Cons:
//   - Notify-based, not handoff: a woken waiter must re-acquire the mutex and
//     re-check the predicate, so it can be barged by an Acquirer that arrived
//     between Signal and the waiter being scheduled — no strict FIFO under
//     contention.
//   - Single mutex serializes ALL operations, even when permits are plentiful;
//     there is no fast path for uncontended Acquire.
//   - sync.Cond gotchas: cannot be copied after first use, no built-in timeout
//     (you'd need a watchdog goroutine + Broadcast, or switch to a channel).
type CondSemaphore struct {
	cond  *sync.Cond
	mu    *sync.Mutex
	count int
	max   int
}

func NewCondSemaphore(init int) *CondSemaphore {
	mu := &sync.Mutex{}
	return &CondSemaphore{
		mu:   mu,
		max:  init,
		cond: sync.NewCond(sync.Locker(mu)),
	}
}

func (c *CondSemaphore) Acquire() {
	c.mu.Lock()

	for c.count >= c.max {
		c.cond.Wait()
	}
	c.count++

	c.mu.Unlock()
}

func (c *CondSemaphore) Release() {
	c.mu.Lock()

	c.count--
	c.cond.Signal()

	c.mu.Unlock()
}

// LockfreeSemaphore avoids mutexes entirely. Permits are an atomic counter
// allowed to go negative — the negative magnitude encodes the number of parked
// waiters — and blocked waiters queue themselves in a lock-free MPMC queue.
// Each waiter owns a single-bit gate (atomic.Bool); Release dequeues the next
// gate and flips it to true, and the waiter spins on gate.Load() until it sees
// true.
//
// Pattern: "fast path / slow path with signed atomic counter." The uncontended
// path is one CAS — Acquire never touches the queue when permits are
// available. Only when the post-decrement value would go negative does the
// goroutine fall through to enqueue + spin. Release symmetrically only
// consults the queue when the post-increment value indicates a waiter was
// already in deficit. Same shape as futex-based mutexes and as sync.Mutex
// itself (atomic state word + runtime park on the slow path) —
// RuntimeSemaphore below is this exact pattern with proper parking
// substituted for the spin.
//
// Pros:
//   - No mutex anywhere; lock-free progress in the global sense.
//   - Hot path is one atomic op with no allocation; competitive with
//     RuntimeSemaphore when uncontended.
//   - Good worked example of composing a signed counter with a wait queue —
//     the canonical building block of a counting semaphore.
//
// Cons:
//   - Slow path spins on gate.Load(), burning a CPU core per blocked goroutine
//     and preventing the runtime from reclaiming the P for other work. This is
//     exactly what RuntimeSemaphore fixes by calling into the runtime to park
//     properly.
//   - Subtle Acquire/Release ordering: Acquire decrements *before* enqueueing
//     its gate, so a concurrent Release can observe permits<=0, Dequeue an
//     empty queue, and have to retry. The for-loop in Release handles this,
//     but it means Release can also spin under unfavorable interleavings.
//   - "Lock-free" is not "contention-free": the MPMC queue still does CAS
//     retries on head/tail, and the permits counter is a cache-line hot spot
//     under heavy load.
//   - The CAS loop in Acquire is overkill — the new value is always val-1, so
//     a single permits.Add(-1) would suffice. The CAS-retry idiom is only
//     meaningful when the new value depends on the old in a way that needs to
//     invalidate on concurrent writers.
type LockfreeSemaphore struct {
	permits *atomic.Int64
	waiters queue.Queue[*atomic.Bool]
}

func NewLockfreeSemaphore(init int) *LockfreeSemaphore {
	p := atomic.Int64{}
	p.Store(int64(init))

	return &LockfreeSemaphore{
		permits: &p,
		waiters: queue.NewLockFreeMPMC[*atomic.Bool](),
	}
}

func (l *LockfreeSemaphore) Acquire() {
	var val int64

	for {
		val = l.permits.Load()
		if l.permits.CompareAndSwap(val, val-1) {
			if val <= 0 {
				gate := &atomic.Bool{}
				l.waiters.Enqueue(gate)

				// park, cpu burn
				for !gate.Load() {
				}
			}
			break
		}
	}
}

func (l *LockfreeSemaphore) Release() {
	after := l.permits.Add(1)
	if after <= 0 {
		for {
			// slow path
			if gate, ok := l.waiters.Dequeue(); ok {
				gate.Store(true)
				break
			}
		}
	}
}

//go:linkname runtime_Semacquire sync.runtime_Semacquire
func runtime_Semacquire(s *uint32)

//go:linkname runtime_Semrelease sync.runtime_Semrelease
func runtime_Semrelease(s *uint32, handoff bool, skipframes int)

// RuntimeSemaphore is the production-grade design: an atomic counter on the
// fast path, and Go's runtime semaphore (linkname'd from package sync) for
// true parking on the slow path. This is essentially the skeleton of
// sync.Mutex and golang.org/x/sync/semaphore minus cancellation plumbing.
//
// Pattern: "atomic counter + runtime park/unpark." When permits >= 1, Acquire
// is one CAS and returns. When the decrement would go negative,
// runtime_Semacquire parks the goroutine onto the runtime's per-address sudog
// treap (semroot), and the scheduler is free to run other goroutines on the P
// — no CPU is wasted. Release calls runtime_Semrelease only when the
// post-increment value indicates a parked waiter, so uncontended release is
// also a single atomic add.
//
// Pros:
//   - True parking: blocked goroutines consume zero CPU and free the P for
//     other work. This is the decisive win over LockfreeSemaphore's spin.
//   - Fast path is one atomic op — identical cost to the lockfree version
//     when uncontended.
//   - Reuses the runtime's battle-tested wait machinery (semroot, sudog pool,
//     GC-aware parking) and its fairness model.
//
// Cons:
//   - Depends on go:linkname into the runtime — an unstable internal API (the
//     `_ "unsafe"` blank import at the top of the file is what authorizes it).
//     The Go team has been progressively tightening linkname access; a future
//     release can break this. Production code should use
//     golang.org/x/sync/semaphore (which wraps a channel) or accept the
//     API-stability risk for the perf win.
//   - The handoff parameter to runtime_Semrelease is hard-coded to false.
//     Setting it true enables direct-handoff fairness — the same mechanism
//     sync.Mutex switches to in starvation mode (when a waiter has been
//     blocked >1ms). Worth understanding before copying this design.
type RuntimeSemaphore struct {
	permits atomic.Int64
	sema    uint32
}

func NewRuntimeSemaphore(init int) *RuntimeSemaphore {
	r := &RuntimeSemaphore{}
	r.permits.Store(int64(init))
	return r
}

func (r *RuntimeSemaphore) Acquire() {
	for {
		val := r.permits.Load()
		if r.permits.CompareAndSwap(val, val-1) {
			if val <= 0 {
				runtime_Semacquire(&r.sema)
			}
			return
		}
	}
}

func (r *RuntimeSemaphore) Release() {
	if r.permits.Add(1) <= 0 {
		runtime_Semrelease(&r.sema, false, 0)
	}
}
