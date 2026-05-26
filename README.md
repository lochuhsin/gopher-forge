# gopher-forge

> A from-scratch forge for low-level **synchronization primitives** in Go — locks, lock-free queues and stacks, barriers, latches, semaphores, and the memory-ordering machinery underneath them.

<p>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.26+-00ADD8?logo=go&logoColor=white">
  <img alt="Tested with -race" src="https://img.shields.io/badge/tested%20with-%60go%20test%20--race%60-success">
  <img alt="Status" src="https://img.shields.io/badge/status-work%20in%20progress-orange">
</p>

This repository re-implements the building blocks of concurrent programming **from the primitive up** — not to replace `sync` / `golang.org/x/sync`, but to understand *why* they are built the way they are. Each primitive is rebuilt directly on atomics and the runtime, benchmarked against a mutex baseline, and stress-tested until the race detector and chaos tests stop complaining.

---

## Why this exists

Most concurrency bugs come from a fuzzy mental model of the layer beneath the abstraction: what `Acquire`/`Release` ordering actually guarantees, where false sharing eats your throughput, why a Treiber stack needs more than a CAS to be safe. The goal here is to **build the layer by hand** so the model stops being fuzzy.

The recurring pattern across the repo:

- **Multiple implementations of the same primitive**, side by side — e.g. a mutex-backed queue, an unpadded lock-free queue, and a cache-line-padded lock-free queue — so the trade-off is measurable, not theoretical.
- **A mutex/channel baseline for everything**, because "this is faster" means nothing without the boring version to beat.
- **Deliberately buggy reference implementations** (see [`stack/buggy_test.go`](stack/buggy_test.go)) used to verify the concurrency tests actually *detect* the races they claim to.
- **Citations in the source** — files name the paper or engineering write-up they implement (Mellor-Crummey & Scott 1991 for MCS locks, Vyukov for the bounded MPMC ring, etc.).

---

## Design principles

| Principle | What it means in practice |
|---|---|
| **Atomics first** | Primitives are built on `sync/atomic` and, where instructive, `//go:linkname` into the runtime's `sema` / `notifyList` — not on top of `sync.Mutex`. |
| **Pad against false sharing** | Hot producer/consumer fields are separated onto their own cache lines (`_ [112]byte` padding around `atomic.Uint64` slots). |
| **Race + chaos + bench** | Every structure runs under `-race`, a multi-`GOMAXPROCS` chaos test, and a benchmark that sweeps `GOMAXPROCS=1,2,4,8` under varying contention. |
| **Generics throughout** | Containers are `[T any]`; primitives are usable at their zero value where possible (`var mu syncx.SpinLock`). |
| **No backward-compat tax** | This is a learning forge, not a published API. Names and signatures change freely to stay close to the literature. |

---

## Quick start

```bash
git clone <this-repo> gopher-forge
cd gopher-forge
go test -race ./...        # correctness (recommended default)
make help                  # see all test/bench targets
```

```go
package main

import (
	"fmt"

	"forge/queue"
	"forge/syncx"
)

func main() {
	// A bounded, lock-free MPMC ring buffer (Vyukov-style sequence counters).
	q := queue.NewLockFreeMPMC[int]()
	q.Enqueue(42)
	if v, ok := q.Dequeue(); ok {
		fmt.Println(v) // 42
	}

	// Locks are usable at their zero value — no constructor required.
	var mu syncx.SpinLock
	mu.Lock()
	// ... critical section ...
	mu.Unlock()
}
```

> Module path is `forge`, so imports are `forge/queue`, `forge/syncx`, `forge/stack`, …

---

## Package map

| Package | What's inside | Status |
|---|---|:---:|
| [`syncx/`](syncx/) | Locks, barriers, latches & events, semaphores, condition variables, channel patterns | ✅ core built |
| [`queue/`](queue/) | MPSC / MPMC / cache-line-padded MPMC lock-free ring buffers + mutex baselines | ✅ mostly built |
| [`stack/`](stack/) | Treiber lock-free stack, elimination-backoff stack, mutex baselines | ✅ built |
| [`memory/`](memory/) | Memory-ordering exercises: acquire/release, fences, publication safety | 📋 planned |
| [`hazard/`](hazard/) | Hazard pointers for safe lock-free reclamation | 📋 planned |
| [`reclamation/`](reclamation/) | Epoch- / quiescent-state-based reclamation (EBR / QSBR) | 📋 planned |
| [`rcu/`](rcu/) | Read-Copy-Update | 📋 planned |
| [`map/`](map/) | Concurrent map: `sync.Map` re-impl + sharded mutex map | 📋 planned |
| [`scope/`](scope/) | Cancellation tokens + structured concurrency (nursery / errgroup) | 📋 planned |
| [`parallel/`](parallel/) | Parallel map/reduce/scan/pipeline + AllReduce algorithms | 📋 planned |
| [`ratelimit/`](ratelimit/) | Token bucket, sliding window, leaky bucket, circuit breaker | 📋 planned |
| [`deque/`](deque/) | Chase-Lev work-stealing deque | 📋 planned |
| [`park/`](park/) | Park / unpark goroutine blocking primitives | 📋 planned |
| [`actor/`](actor/) | Actor model | 📋 planned |
| [`arena/`](arena/) | Arena / bump allocator | 📋 planned |
| [`clock/`](clock/) | Logical clocks: Lamport / vector / hybrid logical clock | 📋 planned |
| [`crdt/`](crdt/) | Conflict-free replicated data types | 📋 planned |
| [`_lab/`](_lab/) | Architectural patterns, verification tools, classic concurrency puzzles | 📋 planned |

---

## What's built today

### `syncx/` — synchronization primitives

- **Locks** ([`lock.go`](syncx/lock.go)) — `SpinLock`, `TicketLock`, `MCSLock`, `CLHLock`, `Seqlock`, `MutexLock`, `RWMutexLock`, `ReentrantMutex`, `StampedLock`, `BrLock` (big-reader). A guided tour from the simplest busy-wait CAS up to queue-based locks that spin on a *local* cache line to kill contention.
- **Barriers** ([`barriers.go`](syncx/barriers.go)) — `CountingBarrier`, `SenseReversingBarrier`, `StaticTreeBarrier`, `TournamentBarrier`, `DisseminationBarrier`, `ButterflyBarrier`, `CombiningTreeBarrier`. The same N-way rendezvous, from one shared counter up to the log-depth tree/butterfly schemes that map onto NCCL-style collectives.
- **Latches & events** ([`latch.go`](syncx/latch.go)) — `CountDownLatch`, `WaitGroup`, `SpinLatch`, `ChanLatch`, `SemaLatch`, `NotifyListLatch`, `AutoResetEvent`, `ManualResetEvent`, `Notify`, `Once`.
- **Semaphores** ([`semaphore.go`](syncx/semaphore.go)) — `ChannelSemaphore`, `MutexSemaphore`, `CondSemaphore`, `LockfreeSemaphore`, `RuntimeSemaphore`, `CountingSemaphore`, `WeightedSemaphore` — five different substrates behind one `Semaphore` interface.
- **Condition variables** ([`cond.go`](syncx/cond.go)) — `MesaQueueCond`, `MesaNotifyListCond` (Mesa-semantics monitor variants).
- **Channel patterns** ([`channel.go`](syncx/channel.go)) — ordered/unordered fan-in and a ring-buffered channel.

### `queue/` — concurrent queues

Bounded lock-free ring buffers with Vyukov-style per-slot sequence numbers, plus mutex baselines for comparison:

- `LockFreeMPSC`, `LockFreeMPMC`, `LockFreePaddedMPMC` (cache-line-padded), `MutexMPSC`, `MutexMPMC`.
- A linearizability-flavored test suite and a `Mutex vs Unpadded vs Padded` benchmark that demonstrates the cost of false sharing.

### `stack/` — concurrent stacks

- `LockFreeMPMC` (Treiber stack), `EliminationBackoffMPMC` (elimination-array backoff to cut CAS contention), `MutexLinkedMPMC`, `MutexSliceMPMC`.

---

## Testing & benchmarking

Everything is wired through the [`Makefile`](Makefile):

```bash
make test          # go test -race ./...          — correctness, the recommended default
make test-stress   # -race -count=5               — shake out flaky/probabilistic races
make test-chaos    # multi-GOMAXPROCS chaos test   — race bugs surface under oversubscription

make bench-queue   # queue benchmarks, GOMAXPROCS=1,2,4,8 × contention levels
make bench-stack   # stack benchmarks
make bench-syncx   # syncx benchmarks
make bench-mpmc-queue   # Mutex vs Unpadded vs Padded MPMC head-to-head

make bench-full    # all benchmarks + cpu.out + mem.out
make cpu-prof      # open cpu.out in the pprof web UI
```

The benchmarks intentionally oversubscribe the scheduler (up to ~64 logical workers per CPU) to expose contention and cache effects that single-threaded microbenchmarks hide.

---

## Roadmap

Built in dependency order: foundations first, then the structures that build on them, then the distributed/parallel layer on top. Checked items have a working implementation under test.

### Concurrent data structures

- [x] `queue/` — MPSC lock-free ring buffer
- [x] `queue/` — MPMC lock-free ring buffer (Vyukov bounded)
- [x] `queue/` — cache-line-padded MPMC (false-sharing benchmark)
- [x] `queue/` — mutex baselines (MPSC / MPMC)
- [ ] `queue/` — SPSC ring buffer + LMAX Disruptor
- [x] `stack/` — Treiber lock-free stack
- [x] `stack/` — elimination-backoff stack
- [x] `stack/` — mutex baselines (linked / slice)
- [ ] `map/` — `sync.Map` re-implementation + sharded mutex map
- [ ] `map/` — open-addressing CAS map (Robin Hood hashing)
- [ ] `deque/` — Chase-Lev work-stealing deque

### Locks & low-level synchronization

- [x] Spin / TTAS / Ticket locks
- [x] MCS & CLH queue locks (spin on a local cache line)
- [x] Seqlock (writer never blocks)
- [x] Mutex / RWMutex / Reentrant / Stamped / big-reader locks
- [ ] RWMutex variants: reader-preferring / writer-preferring / fair

### Coordination primitives

- [x] Barriers — counting, sense-reversing, static-tree, tournament, dissemination, butterfly, combining-tree
- [x] Latches & events — CountDownLatch, WaitGroup, manual/auto-reset events, Once
- [x] Semaphores — channel / mutex / cond / lock-free / runtime / weighted
- [x] Condition variables — Mesa-semantics monitor variants
- [x] Channel patterns — ordered/unordered fan-in, ring-buffered channel
- [ ] Channel patterns — pipeline with backpressure + cancellation
- [ ] `syncx/Future` + `Promise` — split, shared, composable (Then/Map/WhenAll/WhenAny)
- [ ] `scope/` — cancellation tokens, nursery, errgroup, deadline scheduler

### Memory model & safe reclamation

- [ ] `memory/` — acquire/release pairing, fences, publication safety, double-checked locking
- [ ] `hazard/` — hazard pointers
- [ ] `reclamation/` — epoch-based (EBR) & quiescent-state-based (QSBR) reclamation
- [ ] `rcu/` — read-copy-update
- [ ] `arena/` — arena / bump allocator (hot-path pre-allocation)

### Parallel, distributed & resilience

- [ ] `parallel/` — parallel map / reduce / scan (Hillis-Steele vs Blelloch)
- [ ] `parallel/` — pipeline with backpressure
- [ ] `parallel/` — AllReduce (ring / tree / dissemination) + parallel BFS
- [ ] `ratelimit/` — token bucket, sliding window, leaky bucket
- [ ] `ratelimit/` — circuit breaker + bulkhead
- [ ] `actor/` — actor model
- [ ] `clock/` — Lamport / vector / hybrid logical clocks
- [ ] `crdt/` — conflict-free replicated data types
- [ ] `syncx/STM` — software transactional memory (Block-STM style)

### Lab

- [ ] `_lab/pattern/` — architectural patterns (Disruptor, …)
- [ ] `_lab/verify/` — linearizability & litmus-test tooling
- [ ] `_lab/excercise/` — classic concurrency puzzles

> A weighted, prioritized version of this roadmap (with rationale per item) lives in [ROADMAP.md](ROADMAP.md).

---

## References

Source files cite their sources inline. Recurring touchstones:

- Mellor-Crummey & Scott, *Algorithms for Scalable Synchronization on Shared-Memory Multiprocessors* (1991) — MCS / CLH locks, tree & combining barriers.
- Dmitry Vyukov — bounded MPMC queue with per-slot sequence numbers.
- Treiber — lock-free stack; Hendler, Shavit & Yerushalmi — elimination backoff.
- LMAX Disruptor — ring buffer + wait strategies.

---

## License

No formal license yet — treat this as source-available for learning and reference.