package syncx

import (
	"sync"
	"sync/atomic"
	_ "unsafe"
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

	pred := m.tail.Swap(node)
	if pred != nil {
		pred.next.Store(node)
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

type RCULock struct{}

func (r *RCULock) Lock() {
}

func (r *RCULock) Unlock() {
}

type MutexLock struct {
	state atomic.Int32
	sema  uint32
}

func (m *MutexLock) Lock() {
}

func (m *MutexLock) Unlock() {
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
// This is the L1 minimum version: caller must ensure single writer. For
// multi-writer scenarios, wrap WriteLock/WriteUnlock with an external mutex.
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
//     pointers, slices, or maps can leak inconsistent state to reader
//     before validation catches it. Restrict to pure-value (Copy-style)
//     data.
//   - This minimal version assumes a single writer; concurrent WriteLock
//     calls will corrupt the seq parity.

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

// make sure that the start value is always an even number (meaning that we didn't catch a write mid-way through)
// if the end value is even, it means that we didn't catch a write mid-way through
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
