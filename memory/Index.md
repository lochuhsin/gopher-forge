# memory Index

> Learning goal: build the mental model for happens-before, ordering, fences, publication safety, and the bugs that lock-free algorithms rely on you understanding.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: synchronization is created by correct pairing, not by "using atomics" in isolation.
- Release on the publishing side and acquire on the observing side are the recurring pattern behind queues, futures, OnceCell, seqlock validation, RCU, and reclamation.
- Go exposes a stronger public atomic model than many lower-level systems APIs, so this package should teach the portable ordering model explicitly.
- Recommended build order from the merged TODO: ordering cheatsheet, Once/OnceCell, publication safety examples, double-checked locking case study, ABA notes, then litmus tests in `_lab/verify`.
- Dependencies: no upstream dependencies; this package is upstream of `queue/`, `stack/`, `deque/`, `hazard/`, `reclamation/`, `rcu/`, and lock-free `map/`.
- Career signal: highest for low-level systems and HFT-style work because it gives the vocabulary for lock-free correctness.
- Scope rule: keep this package about portable concepts: atomic operations, happens-before, publication, progress guarantees, ABA, and litmus tests; architecture-specific assembly fences belong only as explanatory notes.

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

- [ ] HappensBeforeGraph
  - Core Concept: Model synchronization as a partial order over reads, writes, channel operations, locks, and atomics.
  - Pros: Gives a precise language for visibility and race reasoning.
  - Cons: Graphs get large quickly without tool support.
  - Scenarios: Explaining condvars, channels, `sync.Map`, futures, and `_lab/verify` race detection.

- [ ] AcquireReleasePairing
  - Core Concept: A release write publishes prior writes; an acquire read that observes it establishes happens-before.
  - Pros: The minimum ordering needed for most queues, stacks, and publication patterns.
  - Cons: Pairing must be on the right synchronization variable; otherwise the code can look correct but publish stale data.
  - Scenarios: SPSC queue slots, future completion, OnceCell state, seqlock validation.

- [ ] RelaxedAtomics
  - Core Concept: Atomic operations can provide tear-free counters without synchronization ordering.
  - Pros: Fast and enough for statistics, IDs, and reference increments that do not publish data.
  - Cons: Does not create happens-before; dangerous when mixed with data visibility assumptions.
  - Scenarios: Metrics counters, randomized tickets, refcount increments before release-on-drop.

- [ ] SeqCstOrdering
  - Core Concept: Sequential consistency places all SeqCst atomics into one global order.
  - Pros: Easiest model to reason about and Go's public atomic operations effectively live here.
  - Cons: Can over-synchronize and hide which ordering is actually required in weaker memory-order APIs.
  - Scenarios: Teaching baseline, first implementation before weakening to acquire/release.

- [ ] DataRaceAndDRFSC
  - Core Concept: A data-race-free program can be explained by a sequentially consistent interleaving of goroutines.
  - Pros: Defines the safe default contract before studying weaker or lock-free patterns.
  - Cons: Racy programs may still appear to work, which makes discipline and tooling necessary.
  - Scenarios: Race detector labs, happens-before education, reviewing unsynchronized shared memory.

- [ ] OwnershipAndAliasingDiscipline
  - Core Concept: Mutable state has exactly one active owner, or every shared access goes through an explicit synchronization protocol.
  - Pros: Prevents accidental shared mutation and gives a clean design rule before reaching for atomics.
  - Cons: Ownership boundaries can be inconvenient for graph-shaped data and long-lived shared caches.
  - Scenarios: Message passing, actor state isolation, handoff queues, safe API design.

- [ ] OwnershipTransfer
  - Core Concept: Sending a value transfers responsibility for mutation and lifetime to the receiver instead of sharing mutable aliases.
  - Pros: Eliminates many locks by construction and makes handoff points explicit.
  - Cons: Requires copy, move, or clear ownership conventions for values referenced elsewhere.
  - Scenarios: Channels, actor messages, pipeline stages, work queues.

- [ ] TransferabilityContract
  - Core Concept: A type or resource declares whether it can safely move between concurrent execution contexts.
  - Pros: Catches thread-affinity and non-thread-safe resource mistakes at API boundaries.
  - Cons: Go cannot enforce this statically for arbitrary values, so examples need runtime checks or documentation discipline.
  - Scenarios: File/socket ownership, event-loop resources, actor-local state, non-shareable handles.

- [ ] ShareabilityContract
  - Core Concept: A type or resource declares whether shared references may be used concurrently and under which synchronization rules.
  - Pros: Separates "can move ownership" from "can be shared by many readers/writers."
  - Cons: Requires clear invariants around interior mutation and protected fields.
  - Scenarios: Read-only immutable state, atomic counters, mutex-protected caches, concurrent maps.

- [ ] GuardedMutation
  - Core Concept: Shared data may be mutated only while holding a guard that represents the active synchronization permission.
  - Pros: Couples access rights to lock lifetime and reduces forgotten-unlock style bugs.
  - Cons: Long-lived guards can accidentally expand critical sections or deadlock.
  - Scenarios: Mutex-protected maps, monitor objects, scoped lock helpers, protected caches.

- [ ] InteriorMutabilityBoundary
  - Core Concept: Mutation through a shared handle is allowed only when the object internally enforces atomic, lock, or single-owner rules.
  - Pros: Makes hidden synchronization explicit in type and API design.
  - Cons: Easy to abuse if callers cannot see whether operations are atomic, locked, or merely unsafe.
  - Scenarios: Atomic cells, protected values, actor references, lazily initialized state.

- [ ] ThreadAffinity
  - Core Concept: Some state is valid only on its owning worker, event loop, or actor and must not be used elsewhere.
  - Pros: Preserves locality and avoids accidental unsynchronized access.
  - Cons: Requires handoff or proxy APIs when other workers need access.
  - Scenarios: UI/event-loop state, thread-per-core shards, actor-local resources, scheduler-owned queues.

- [ ] FenceCheatsheet
  - Core Concept: Map compiler barriers, CPU fences, acquire loads, release stores, and full barriers across x86 and ARM.
  - Pros: Makes architecture differences concrete.
  - Cons: Go does not expose fine-grained public fence APIs, so examples need careful framing.
  - Scenarios: HFT interview prep, low-level systems review, and ARM64 correctness discussion.

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

- [ ] WeakReference
  - Core Concept: A non-owning handle observes an object only if it can be upgraded while a strong reference still exists.
  - Pros: Breaks ownership cycles and separates observation from lifetime extension.
  - Cons: Upgrade races and object finalization semantics must be precise.
  - Scenarios: Actor registries, cache entries, shared ownership graphs, resource managers.

- [ ] ProgressGuarantees
  - Core Concept: Classify algorithms as blocking, obstruction-free, lock-free, or wait-free.
  - Pros: Prevents vague claims like "lock-free" from hiding starvation or helping requirements.
  - Cons: Stronger progress often costs significant complexity and constant factors.
  - Scenarios: Queue/stack reviews, interview explanations, wait-free vs lock-free comparisons.

- [ ] LinearizationPoint
  - Core Concept: Each operation appears to take effect at one instant between call and return.
  - Pros: Connects memory ordering to correctness proofs and `_lab/verify` histories.
  - Cons: Some algorithms have helping or conditional linearization that is hard to identify.
  - Scenarios: Treiber push/pop CAS, Michael-Scott enqueue/dequeue, concurrent map updates.

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

- [ ] LitmusTests
  - Core Concept: Small programs expose allowed reorderings such as store buffering and load buffering.
  - Pros: Turns abstract memory model rules into observable experiments.
  - Cons: Results depend on hardware, compiler, and Go's atomic model.
  - Scenarios: `_lab/verify` integration, x86 vs ARM demonstrations.
