# memory Index

> Learning goal: build the mental model for happens-before, ordering, fences, publication safety, and the bugs that lock-free algorithms rely on you understanding.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: synchronization is created by correct pairing, not by "using atomics" in isolation.
- Release on the publishing side and acquire on the observing side are the recurring pattern behind queues, futures, OnceCell, seqlock validation, RCU, and reclamation.
- Go exposes a stronger public atomic model than many lower-level systems APIs, so this package should teach the portable ordering model explicitly.
- Recommended build order from the merged TODO: ordering cheatsheet, Once/OnceCell, publication safety examples, double-checked locking case study, ABA notes, then litmus tests in `_lab/verify`.
- Dependencies: no upstream dependencies; this package is upstream of `queue/`, `stack/`, `deque/`, `hazard/`, `reclamation/`, `rcu/`, and lock-free `mapx/`.
- Career signal: highest for low-level systems and HFT-style work because it gives the vocabulary for lock-free correctness.
- Scope rule: keep this package about portable concepts: atomic operations, happens-before, publication, progress guarantees, ABA, and litmus tests; architecture-specific assembly fences belong only as explanatory notes.
- Implementation scope rule: only items that can be expressed as real Go code (with tests or benchmarks) live in the checklist below. Pure concepts that Go has no public API for — fences, consume ordering, LL/SC, double-width CAS, weak references, MESI, OOTA, etc. — are intentionally excluded so the Index never claims a deliverable that cannot be built.

## Reference Trail and Go Boundary

- Primary Go references: Go memory model (`https://go.dev/ref/mem`) and `sync/atomic` (`https://pkg.go.dev/sync/atomic`). Go atomics are the public substrate; they are not optional decoration around ordinary racy fields.
- Correctness reference: Herlihy and Wing linearizability (`https://www.cs.cmu.edu/~wing/publications/HerlihyWing90.pdf`) gives the "one instant between call and return" model used by queues, stacks, maps, and checkers.
- Substrate boundary: implement CAS, fetch-add, swap, and atomic pointer publication by calling `sync/atomic`; do not pretend to implement those hardware read-modify-write operations from ordinary Go loads and stores.
- Mental model: every item should name the protected location, the write that publishes, the read that observes, the happens-before edge, and the progress class.
- ABA focus: treat ABA as "same representation, different logical object." Go GC hides many reuse bugs, so ABA labs should use tagged indexes, simulated allocators, unsafe experiments, or reclamation integration instead of pretending GC makes the concept disappear.
- Interview artifact: each memory item should be explainable as a small state machine plus a two-goroutine trace, because most lock-free interview failures come from mixing visibility, atomicity, and ownership.

## Implementation Checklist

- [~] OrderingScaffold
  - Core Concept: The package exists as the home for memory-ordering examples; `ordering.go` is still empty, while `atomic.go` now contains the first executable atomic cell example.
  - Pros: Gives the repo a dedicated foundation layer for every lock-free structure.
  - Cons: Ordering-specific examples and documentation are still missing beyond basic atomic load/store.
  - Scenarios: Next step before queue, stack, hazard pointer, RCU, and atomics-heavy work.

- [x] AtomicLoadStore
  - Core Concept: `AtomicCell[T]` publishes value copies through an atomic pointer, and `Load` observes either the latest published value or T's zero value.
  - Pros: Foundation for ready flags, published pointers, counters, and lock-free metadata; covered by basic, zero-value, overwrite, pointer, copy, and concurrent no-tear tests.
  - Cons: Does not make a multi-field invariant atomic by itself, and it intentionally covers only load/store rather than CAS or arithmetic.
  - Scenarios: Publication flags, seqlock counters, RCU pointers, queue slot states.
  - Substrate: `sync/atomic.Pointer[T]`; linearization point is the atomic call; progress is wait-free; publishes only the cell pointer, not unrelated reachable fields.

- [ ] CompareAndSwap
  - Core Concept: CAS updates a location only if it still contains the expected value.
  - Pros: Core operation behind lock-free stacks, queues, maps, and state machines.
  - Cons: Retry loops can livelock under contention and CAS alone does not solve ABA.
  - Scenarios: Treiber stack, Michael-Scott queue, mutex fast paths, OnceCell state transitions.

- [ ] FetchAddCounter
  - Core Concept: Atomic arithmetic reserves tickets, increments counters, or advances sequence numbers without a lock.
  - Pros: Excellent for monotonic IDs, ticket locks, refcounts, and ring-buffer cursors.
  - Cons: Hot counters become cache-line bottlenecks and overflow must be defined.
  - Scenarios: TicketLock, WaitGroup packed state, queue head/tail counters, rate limiter accounting.

- [ ] StripedCounter
  - Core Concept: Split updates across padded atomic cells and aggregate on read, LongAdder-style, to reduce a single hot counter's cache-line traffic.
  - Pros: Teaches contention dilution, padding, approximate read cost, and why one atomic counter can bottleneck metrics and admission paths.
  - Cons: Reads must sum all cells and may not be a single linearizable point unless the API explicitly documents weak snapshot semantics.
  - Scenarios: Metrics counters, rate limiter accounting, queue statistics, false-sharing benchmarks.

- [ ] AtomicExchange
  - Core Concept: Atomically replace a value and return the previous value.
  - Pros: Useful for intrusive queues, once-only handoff, and state swaps.
  - Cons: Less conditional than CAS, so callers must reason about all old states.
  - Scenarios: Vyukov intrusive MPSC queue, signal exchange, pointer publication experiments.

- [ ] AcquireReleasePairing
  - Core Concept: A release write publishes prior writes; an acquire read that observes it establishes happens-before.
  - Pros: The minimum ordering needed for most queues, stacks, and publication patterns.
  - Cons: Pairing must be on the right synchronization variable; otherwise the code can look correct but publish stale data.
  - Scenarios: SPSC queue slots, future completion, OnceCell state, seqlock validation.

- [ ] FalseSharing
  - Core Concept: Independent hot variables on the same cache line invalidate each other under concurrent writes.
  - Pros: Makes performance bugs visible without changing logical correctness.
  - Cons: Padding fixes increase memory footprint and can be overused.
  - Scenarios: Queue head/tail padding, counters, lock benchmarking, HFT tuning.

- [ ] PublicationSafety
  - Core Concept: Initialize data fully before publishing the pointer or ready flag that readers observe.
  - Pros: Central invariant behind safe lock-free reads.
  - Cons: Violations are timing-dependent and often invisible on x86.
  - Scenarios: Immutable config publish, ring buffer slots, future result delivery.

- [ ] DoubleCheckedLocking
  - Core Concept: Fast-path check, slow-path lock, initialize once, and publish with correct ordering.
  - Pros: Shows why unsafely published lazy initialization is broken and how acquire/release fixes it.
  - Cons: Easy to implement incorrectly if the state flag and value publication are separated poorly.
  - Scenarios: Singleton, lazy cache, OnceCell, config loading.

- [ ] OnceCell
  - Core Concept: A state machine `empty -> initializing -> done` protects one-time value initialization.
  - Pros: Practical teaching bridge between atomics, mutex fallback, and future-like write-once values.
  - Cons: Panic, retry, and concurrent initializer semantics must be explicitly chosen.
  - Scenarios: Lazy initialization, memoization, service-level shared resources.

- [ ] AtomicRefcount
  - Core Concept: Increment references with relaxed ordering and use release decrement plus acquire fence before destruction.
  - Pros: Explains `Arc`/`shared_ptr` internals and safe object lifetime.
  - Cons: Subtle because increment and destruction use different ordering strengths.
  - Scenarios: Shared ownership, resource lifetime management, reclamation internals.

- [ ] ABAProblem
  - Core Concept: A pointer can change A -> B -> A, making a CAS succeed even though the logical object changed.
  - Pros: Essential bridge from Treiber stack to hazard pointers, tagged pointers, and EBR.
  - Cons: Hard to reproduce under Go GC unless examples are carefully designed.
  - Scenarios: Lock-free stacks, Michael-Scott queue, freelists.

- [ ] TaggedVersionedPointers
  - Core Concept: Pair a pointer or index with a version so repeated addresses still produce different CAS values.
  - Pros: Classic ABA mitigation and useful for bounded rings or index-based structures.
  - Cons: Wide CAS or careful packing may be unavailable or non-portable in Go.
  - Scenarios: ABA labs, freelists, bounded deques, tagged-index comparison.
