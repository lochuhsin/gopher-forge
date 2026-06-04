package syncx

import (
	"sync"
	"sync/atomic"
	_ "unsafe"

	"forge/park"
)

//go:linkname runtime_SemacquireMutex sync.runtime_SemacquireMutex
func runtime_SemacquireMutex(s *uint32, lifo bool, skipframes int)

// SpinLock is the simplest busy-wait mutual exclusion lock: one atomic flag
// marks whether the lock is held, and contenders repeatedly CAS the flag until
// they acquire it.
//
// Pros:
//   - Very small and fast at low contention: one atomic CAS to acquire and one
//     atomic store to release.
//   - No scheduler interaction, parking, or wake-up overhead, so it can be a
//     good fit for extremely short critical sections.
//   - Zero allocations and a compact memory footprint.
//
// Cons:
//   - No fairness guarantee; a goroutine can be repeatedly beaten by newer
//     contenders and starve.
//   - Burns CPU while waiting. If the critical section is long or the holder is
//     descheduled, all waiters waste cores spinning.
//   - Heavy contention causes cache-line bouncing on the single shared `state`
//     flag, so throughput degrades quickly as the number of waiters grows.
type SpinLock struct {
	state atomic.Bool
}

func (s *SpinLock) Lock() {
	for {
		if s.state.CompareAndSwap(false, true) {
			return
		}
	}
}

func (s *SpinLock) Unlock() {
	s.state.Store(false)
}

// TicketLock is a FIFO spinlock based on the bakery algorithm: each
// goroutine atomically takes a monotonically increasing ticket and spins
// until `serving` matches its ticket.
//
// Pros:
//   - Strict FIFO ordering; no starvation.
//   - Simple: two counters, fetch-and-add to acquire, single increment to
//     release.
//   - Unlock is uncontended — only the current holder ever writes `serving`,
//     so no CAS is needed on the release path.
//
// Cons:
//   - Cache-line bouncing under contention. Every waiter spins on the same
//     `serving` variable, so on Unlock the MESI protocol must invalidate that
//     cache line on all N-1 waiting cores before the writer can transition it
//     to Modified. Each waiter then re-fetches the line across the
//     interconnect.
//   - This yields O(N) coherence traffic per handoff and O(N^2) total to drain
//     a queue of N waiters. Performance degrades sharply as core count grows;
//     not suitable for highly contended locks on many-core machines (see
//     MCSLock for the queue-based fix).
type TicketLock struct {
	ticket  atomic.Uint64
	serving atomic.Uint64
}

func (t *TicketLock) Lock() {
	my := t.ticket.Add(1) - 1
	for my != t.serving.Load() {
	}
}

func (t *TicketLock) Unlock() {
	t.serving.Add(1)
}

type RCULock struct{}

func (r *RCULock) Lock() {
}

func (r *RCULock) Unlock() {
}

const (
	unlocked = iota
	lockedNoWait
	lockedWait
)

// MutexLock is a sleeping mutual-exclusion lock built on a three-state atomic
// word plus a park.Parker for the blocking slow path. It is the futex-style
// design used by Linux pthread mutexes and Go's own sync.Mutex: a lock-free
// fast path when uncontended, and real goroutine parking only when a waiter
// must block.
//
// The state word has three values:
//
//	unlocked     (0) no one holds the lock
//	lockedNoWait (1) held, and no goroutine is parked waiting
//	lockedWait   (2) held, and at least one goroutine is parked
//
// The third state is the key optimization: it lets Unlock know whether a
// waiter exists, so the common uncontended unlock is a single CAS(1->0) with
// no park/unpark and no wasted wakeup. A two-state lock cannot distinguish
// "held, someone waiting" from "held, nobody waiting" and would have to signal
// on every unlock.
//
// Fast path: CAS(unlocked -> lockedNoWait). One atomic op, no parking.
//
// Slow path: Swap(lockedWait), not CAS. Swap unconditionally marks the lock
// contended — including the crucial held-but-unmarked transition (1 -> 2) —
// and returns the prior value. If the prior value was unlocked, the lock was
// actually free and we just acquired it without parking; otherwise we park.
// Using CAS(unlocked, lockedWait) here is a deadlock bug: it never performs
// the 1 -> 2 transition, so a lock held via the fast path is never marked as
// having a waiter, and Unlock returns via CAS(1->0) without ever unparking the
// sleeper.
//
// The Lock loop re-checks after each wakeup because a freshly woken waiter can
// be barged by another goroutine acquiring via the fast path; on a lost race
// it parks again.
//
// Pros:
//   - Uncontended lock/unlock is two atomic CASes: zero syscalls, zero
//     park/unpark. The common case is cheap.
//   - Blocked waiters truly park (via the runtime semaphore inside
//     park.Parker) and consume no CPU, unlike a spinlock.
//
// Cons:
//   - Not fair/FIFO: a woken waiter can be barged by a newly arriving
//     goroutine on the fast path, so a waiter can be delayed under contention.
//   - No recursion, no TryLock/timeout, no ownership tracking.
//   - Depends on park.Parker, which uses go:linkname into the runtime sema.
//
// Mental model: the Lock/Unlock API looks like a same-goroutine pair (you
// lock, you unlock), but the contended path underneath is a cross-goroutine
// handoff — the blocked locker parks, and a different goroutine's Unlock
// unparks it.
//
// Substrate: sync/atomic state word + park.Parker (runtime-semaphore parking).
type MutexLock struct {
	state  atomic.Int32
	parker park.Parker
}

func (m *MutexLock) Lock() {
	// fast path
	if m.state.CompareAndSwap(unlocked, lockedNoWait) {
		return
	}

	// slow path
	for {
		if m.state.Swap(lockedWait) == unlocked {
			return
		}
		m.parker.Park()
	}
}

func (m *MutexLock) Unlock() {
	if m.state.CompareAndSwap(lockedNoWait, unlocked) {
		return
	}

	if m.state.CompareAndSwap(lockedWait, unlocked) {
		m.parker.Unpark()
	}
}

type RWMutexLock struct {
	mu sync.RWMutex
}

func (r *RWMutexLock) Lock() {
	r.mu.Lock()
}

func (r *RWMutexLock) Unlock() {
	r.mu.Unlock()
}

func (r *RWMutexLock) RLock() {
	r.mu.RLock()
}

func (r *RWMutexLock) RUnlock() {
	r.mu.RUnlock()
}

// SeqLock is an optimistic reader-writer synchronization primitive: writers
// bracket their critical section with seq +1 (even→odd→even), and readers
// snapshot the seq, do their reads, then re-check that seq hasn't changed.
//
// The sequence counter encodes lock state via parity (even = no writer
// active, odd = writer in progress) and version (every write produces a
// unique seq value), letting readers detect any concurrent write without
// acquiring any lock.
//
// Writers are serialized internally by writerMu: it's taken before the
// leading seq +1 and released after the trailing one, so concurrent
// WriteLock calls block each other but the seq parity invariant is always
// upheld. Readers remain wait-free against writers.
//
// Pros:
//   - Readers never block writers; writers never block readers. Both paths
//     have deterministic latency floors (no syscall, no scheduling).
//   - Reader's hot path is two atomic Loads + value copy — no CAS, no
//     cache-line bouncing. Cache line stays in Shared state across readers.
//   - Perfect for read-heavy, writer-rare workloads with pure-value data
//     (HFT market data quotes, Linux gettimeofday, in-memory config).
//
// Cons:
//   - Reader may retry under writer contention; livelock theoretically
//     possible if writer never drains (rare in practice).
//   - Snapshot consistency only — not memory safety. Data containing
//     pointers, slices, or maps can leak inconsistent state to the reader
//     before validation catches it. Restrict to pure-value (copy-style)
//     data.
//   - The write path pays a mutex cost via writerMu. If you can guarantee
//     a single writer, a leaner variant without the mutex is faster.
type SeqLock struct {
	seq      atomic.Uint64
	writerMu sync.Mutex
}

func (s *SeqLock) WriteLock() {
	s.writerMu.Lock()
	s.seq.Add(1)
}

func (s *SeqLock) WriteUnlock() {
	s.seq.Add(1)
	s.writerMu.Unlock()
}

func (s *SeqLock) ReadBegin() uint64 {
	return s.seq.Load()
}

func (s *SeqLock) ReadValidate(start uint64) bool {
	return (start&1) == 0 && s.seq.Load() == start
}

func (s *SeqLock) Read(f func()) {
	for {
		start := s.seq.Load()
		if start&1 != 0 {
			continue
		}
		f()
		if s.seq.Load() == start {
			return
		}
	}
}

// CLHLock is a queue-based FIFO spinlock (Craig 1993; Landin & Hagersten 1994).
// Each Lock call allocates a node, atomically appends it to an implicit queue
// via a single tail Swap, and spins on its predecessor's `state` flag. On
// Unlock, the holder writes only to its own node — the successor (already
// spinning on that flag) observes the change and proceeds.
//
// Unlike MCS, the queue is implicit: there is no `next` pointer. The chain of
// "who is watching whom" is the queue order, formed by each new locker
// capturing the previous tail as its predecessor. Each node is owned by
// exactly one goroutine, and only that goroutine ever writes to it on
// release — there are no cross-goroutine writes on the release path.
//
// Pros:
//   - Strict FIFO fairness, inherited from the order of tail Swap calls.
//   - The release path is minimal: one atomic Store on the unlocker's own
//     node. No successor lookup, no `next` pointer publication race, no
//     "last waiter leaving" CAS. Significantly simpler than MCS to reason
//     about and implement correctly.
//   - Each goroutine truly owns its node — only the owner mutates it on
//     release, so the ownership contract is clean.
//   - This shape underpins Java's AbstractQueuedSynchronizer (AQS); the
//     same skeleton powers ReentrantLock, ReentrantReadWriteLock, Semaphore,
//     and CountDownLatch in `java.util.concurrent`.
//
// Cons (cache locality — why MCS wins on big NUMA boxes):
//   - A CLH waiter spins on `pred.state`, where `pred` is the previous
//     locker's node. That cache line's "home" is the predecessor's core,
//     not the waiter's. An MCS waiter, by contrast, spins on `myNode.locked`
//     — a line homed on the waiter's own core. Both protocols transfer
//     exactly one cache line at handoff (one core's write, another core's
//     re-read), so on a single-socket, uniformly cache-coherent machine
//     (typical desktop / laptop / cloud VM), CLH and MCS perform essentially
//     the same and CLH's simpler code wins.
//   - On NUMA / multi-socket servers the picture changes. A CLH waiter's
//     spin variable may live on remote-socket memory; every cache miss
//     during spinning crosses the inter-socket interconnect (UPI / Infinity
//     Fabric), which is an order of magnitude slower than local L3. MCS
//     keeps the spin variable on the waiter's own NUMA node, so contention
//     reads stay local; only the one cross-socket write at handoff pays
//     the remote penalty. This is why Linux's `qspinlock` and most HFT
//     locks use MCS-style local-spin queues on large boxes — the saving
//     scales with both core count and socket count.
//   - Lock returns a `*clhNode` handle that the caller must hand back on
//     Unlock — slightly less ergonomic than a pure Mutex API.
//   - Heap-allocated node per Lock call (same constant cost as MCSLock).
//
// Summary: prefer CLH for code clarity when running on a single-socket
// cache-coherent machine; prefer MCS when scaling to many cores across
// sockets.
type CLHLock struct {
	tail atomic.Pointer[clhNode]
}

type clhNode struct {
	state atomic.Bool
}

func (l *CLHLock) Lock() *clhNode {
	node := &clhNode{}

	prev := l.tail.Swap(node)
	if prev != nil {
		for !prev.state.Load() { // spin
		}
	}

	return node
}

func (l *CLHLock) Unlock(node *clhNode) {
	if node == nil {
		panic("syncx: unlock of unlocked CLHLock")
	}
	node.state.Store(true)
}

// MCSLock is a queue-based FIFO spinlock (Mellor-Crummey & Scott, 1991).
// Contenders enqueue a node onto a lock-free linked list and spin on a flag
// local to their own node, so each waiter's spin variable lives on a cache
// line private to one core.
//
// Pros:
//   - Each waiter spins on its own cache line. Unlock invalidates exactly one
//     remote cache line — the next waiter's `locked` flag — giving O(1)
//     coherence traffic per handoff and O(N) total to drain a queue of N
//     waiters. Directly fixes TicketLock's O(N^2) cache storm.
//   - Strict FIFO fairness, inherited from the queue order.
//   - Scales well on many-core machines; this is the design behind Linux's
//     qspinlock and Java's AQS (CLH variant).
//
// Cons:
//   - More complex: a per-acquisition queue node plus careful handling of the
//     enqueue/dequeue races, in particular the "last waiter leaving" case
//     where the tail pointer must be CAS'd back to nil and the predecessor
//     must wait for a successor's `next` pointer to be published.
//   - Higher constant cost than TicketLock at low contention (extra atomics
//     and a heap-allocated node per Lock call).
type MCSLock struct {
	tail  atomic.Pointer[mcsNode]
	owner atomic.Pointer[mcsNode]
}

type mcsNode struct {
	next   atomic.Pointer[mcsNode]
	locked atomic.Bool
}

func (m *MCSLock) Lock() {
	node := &mcsNode{}
	node.locked.Store(true)

	prev := m.tail.Swap(node)
	if prev != nil {
		prev.next.Store(node)
		for node.locked.Load() {
		}
	}
	m.owner.Store(node)
}

func (m *MCSLock) Unlock() {
	node := m.owner.Load()
	if node == nil {
		panic("syncx: unlock of unlocked MCSLock")
	}

	next := node.next.Load()
	if next == nil {
		if m.tail.CompareAndSwap(node, nil) {
			m.owner.Store(nil)
			return
		}
		for next == nil {
			next = node.next.Load()
		}
	}

	m.owner.Store(nil)
	next.locked.Store(false)
}
