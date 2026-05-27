# gopher-forge

> A Go workbench for rebuilding concurrency and synchronization primitives
> from first principles: locks, semaphores, latches, barriers, queues, stacks,
> memory-ordering experiments, reclamation schemes, and the structures built on
> top of them.

<p>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.26.1-00ADD8?logo=go&logoColor=white">
  <img alt="Module" src="https://img.shields.io/badge/module-forge-555555">
  <img alt="Status" src="https://img.shields.io/badge/status-active%20workbench-orange">
</p>

`gopher-forge` is not a replacement for Go's `sync` package. It is a
from-scratch collection of primitives whose purpose is to make the machinery
visible: atomic state transitions, cache-line contention, blocking and wakeup
protocols, progress guarantees, and the test patterns needed to catch
concurrency bugs.

The repository is organized as a technical forge: each package has an
`Index.md` with the learning goal, implementation checklist, and notes; the Go
source then implements selected items with tests and benchmarks.

---

## Core ideas

| Principle | How it shows up here |
|---|---|
| Build the boring version first | Most structures have a mutex or channel baseline before the lock-free version. |
| Keep variants side by side | Compare spin locks, ticket locks, MCS locks, channel semaphores, runtime semaphores, mutex queues, padded queues, and more in one codebase. |
| Treat correctness as a workload | Tests include concurrent producers/consumers, conservation checks, race-focused stress tests, and deliberately buggy reference implementations. |
| Measure contention, not just throughput | Benchmarks vary worker counts, contention levels, latency percentiles, and allocation behavior. |
| Make incomplete work explicit | `[x]` means implemented, `[~]` means scaffold or partial exploration, and `[ ]` means planned in the package indexes. |

---

## Quick start

```bash
git clone <this-repo> gopher-forge
cd gopher-forge

go test -run=^$ ./...   # compile all packages and test files
make help               # list test, stress, benchmark, and profile targets
```

Example imports use the module path `forge`:

```go
package main

import (
	"fmt"

	"forge/queue"
	"forge/stack"
	"forge/syncx"
)

func main() {
	q := queue.NewLockFreeMPMC[int]()
	_ = q.Enqueue(42)

	if v, ok := q.Dequeue(); ok {
		fmt.Println(v)
	}

	s := stack.NewLockFreeMPMC[string]()
	s.Push("first")
	_, _ = s.Pop()

	var mu syncx.SpinLock
	mu.Lock()
	mu.Unlock()
}
```

This is an active workbench. Some tests intentionally exercise unfinished or
buggy implementations, so targeted package runs are often more useful than
treating the entire tree as a polished library release.

---

## Package map

| Package | Focus | Current state |
|---|---|---|
| [`syncx/`](syncx/) | Locks, semaphores, latches, barriers, channel helpers, condition-variable experiments | Mixed: several implemented primitives plus several explicit scaffolds |
| [`queue/`](queue/) | Bounded FIFO queues for SPSC, MPSC, and MPMC profiles | Mutex, MPSC, MPMC, and padded MPMC implementations; SPSC is scaffolded |
| [`stack/`](stack/) | LIFO stacks under CAS contention | Mutex-backed stacks, Treiber stack, and elimination-backoff stack |
| [`memory/`](memory/) | Memory ordering, publication safety, fences, false sharing, ABA | Index and package scaffold |
| [`hazard/`](hazard/) | Hazard pointers and safe lock-free reclamation | Index and package scaffold |
| [`reclamation/`](reclamation/) | Epoch, quiescent-state, and interval-based reclamation | Roadmap index |
| [`rcu/`](rcu/) | Read-Copy-Update and grace-period tracking | Roadmap index |
| [`map/`](map/) | Concurrent maps, sharding, copy-on-write, lock-free hashing | Roadmap index |
| [`deque/`](deque/) | Work-stealing deques and owner-fast scheduling structures | Roadmap index |
| [`park/`](park/) | Park/unpark contracts and lost-wakeup avoidance | Roadmap index |
| [`scope/`](scope/) | Structured concurrency, cancellation, deadlines | Roadmap index |
| [`parallel/`](parallel/) | Parallel map/reduce/scan, pipelines, work stealing, collectives | Roadmap index |
| [`ratelimit/`](ratelimit/) | Token buckets, sliding windows, breakers, bulkheads, backpressure | Roadmap index |
| [`actor/`](actor/) | Actors, mailboxes, supervision, request/reply | Roadmap index |
| [`arena/`](arena/) | Arena allocation, slabs, pools, freelists | Roadmap index |
| [`clock/`](clock/) | Lamport clocks, vector clocks, hybrid logical clocks | Roadmap index |
| [`crdt/`](crdt/) | Convergent replicated data types and merge laws | Roadmap index |
| [`_lab/`](_lab/) | Pattern notes, verification experiments, classic concurrency exercises | Roadmap indexes |

---

## Implemented highlights

### `syncx`: synchronization primitives

`syncx` is the main primitive playground. It currently contains:

| Family | Implemented | Scaffolded or planned |
|---|---|---|
| Locks | `SpinLock`, `TicketLock`, `MCSLock`, `RWMutexLock`, `SeqLock` | `MutexLock`, `RCULock`, CLH-style and other advanced lock variants |
| Semaphores | `ChannelSemaphore`, `MutexSemaphore`, `CondSemaphore`, `LockfreeSemaphore`, `RuntimeSemaphore` | weighted, timeout, and fair semaphores |
| Latches | `SpinLatch`, `ChanLatch`, custom `WaitGroup` | `SemaLatch`, `NotifyListLatch`, manual/auto-reset events, once variants |
| Barriers | `CountingBarrier` (one-shot), `SenseReversingBarrier` | combining tree, static tree, tournament, dissemination, butterfly |
| Channels | `OrderedChannel`, `UnorderedChannel`, `RingQueue` | pipeline backpressure, exchanger, one-shot channels |
| Other | runtime semaphore and notify-list experiments | Mesa condition variables, futures, promises, STM |

The package deliberately compares substrates: pure spinning, `sync.Mutex`,
`sync.Cond`, channels, lock-free queues, and `go:linkname` calls into Go's
runtime semaphore and notify-list machinery.

### `queue`: bounded concurrent queues

The queue package studies FIFO transfer under different producer and consumer
profiles:

- `MutexMPSC` and `MutexMPMC` as simple baselines.
- `LockFreeMPSC` for multiple producers and one consumer.
- `LockFreeMPMC` using per-slot sequence counters.
- `LockFreePaddedMPMC` to isolate hot fields and expose false-sharing costs.
- `Queue[T]` as the small shared interface used by tests and benchmarks.

The default bounded queue size is `1024`, with index notes for SPSC rings,
blocking queues, transfer queues, disruptor-style rings, and wait-free
directions.

### `stack`: lock-free and contention-mitigated stacks

The stack package contains:

- `MutexSliceMPMC` and `MutexLinkedMPMC` baselines.
- `LockFreeMPMC`, a Treiber-style CAS stack.
- `EliminationBackoffMPMC`, which lets opposing push/pop operations meet in an
  elimination array to reduce contention on the central stack head.

Tests focus on LIFO behavior, concurrent push/pop conservation, and bug
detectors that prove the tests can catch lost or duplicated values.

---

## Testing and benchmarking

Useful commands are collected in the [`Makefile`](Makefile):

```bash
make test          # go test -race -v ./...
make test-short    # race tests with slow tests skipped
make test-stress   # race tests with count=5
make test-chaos    # targeted chaos test

make bench         # all benchmarks
make bench-queue   # queue benchmark matrix
make bench-stack   # stack benchmark matrix
make bench-syncx   # syncx benchmark matrix

make bench-full    # benchmarks with cpu.out and mem.out
make cpu-prof      # open cpu.out with pprof
make mem-prof      # open mem.out with pprof
```

The benchmark suites report more than aggregate ops/sec. They also track
contention levels, latency distributions, memory deltas, and scaling behavior
across `GOMAXPROCS` values.

---

## How to read the repo

Start with the implemented packages, then use the indexes as the roadmap:

1. Read [`syncx/Index.md`](syncx/Index.md), [`queue/Index.md`](queue/Index.md),
   and [`stack/Index.md`](stack/Index.md) for the current primitive families.
2. Read the matching `.go` file next to each checklist item.
3. Read the tests before trusting an implementation; many files encode the
   intended safety contract more clearly than comments alone.
4. Use the benchmarks to compare variants only after checking the correctness
   tests for that package.

The indexes intentionally contain future work. They are the design map, not a
claim that every item is already implemented.

---

## Reference trail

The code and comments are shaped by classic synchronization and lock-free
building blocks:

- Mellor-Crummey and Scott style queue locks and scalable barriers.
- Treiber stacks and elimination backoff for highly contended LIFO workloads.
- Vyukov-style bounded MPMC rings with per-slot sequence counters.
- Go runtime parking primitives: semaphores, notify lists, channels, and
  scheduler-aware blocking.
- Memory-model topics such as acquire/release publication, happens-before,
  false sharing, ABA, and safe reclamation.

---

## Status

This repository is intentionally unstable. APIs, names, and implementations
change as primitives move from notes to scaffold to tested code.

Use it as a study and experimentation forge. Do not treat it as a production
dependency without a focused correctness review for the exact primitive and
workload you intend to use.

---

## License

No formal license has been selected yet. Treat the repository as
source-available for reading and experimentation until a license file is added.
