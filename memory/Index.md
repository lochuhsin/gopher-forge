# memory Index

> Learning goal: build the mental model for happens-before, ordering, fences, publication safety, and the bugs that lock-free algorithms rely on you understanding.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: synchronization is created by correct pairing, not by "using atomics" in isolation.
- Release on the publishing side and acquire on the observing side are the recurring pattern behind queues, futures, OnceCell, seqlock validation, RCU, and reclamation.
- Go exposes a stronger public atomic model than C++/Rust, so this package should teach the cross-language model explicitly rather than pretending Go has the same fine-grained API.
- Recommended build order from the merged TODO: ordering cheatsheet, Once/OnceCell, publication safety examples, double-checked locking case study, ABA notes, then litmus tests in `_lab/verify`.
- Dependencies: no upstream dependencies; this package is upstream of `queue/`, `stack/`, `deque/`, `hazard/`, `reclamation/`, `rcu/`, and lock-free `map/`.
- Career signal: highest for Rust/C++/HFT-style systems work because it gives the vocabulary for lock-free correctness.
- Scope rule: keep this package about portable concepts: atomic operations, happens-before, publication, progress guarantees, ABA, and litmus tests; architecture-specific assembly fences belong only as explanatory notes.

## Implementation Checklist

- [~] OrderingScaffold
  - Core Concept: The package exists as the home for memory-ordering examples, but `ordering.go` currently only declares `package memory`.
  - Pros: Gives the repo a dedicated foundation layer for every lock-free structure.
  - Cons: No executable examples or docs live in the package yet.
  - Scenarios: Next step before queue, stack, hazard pointer, RCU, and atomics-heavy work.

- [ ] AtomicLoadStore
  - Core Concept: Single-location atomic reads and writes provide tear-free access and, in Go, sequentially consistent ordering.
  - Pros: Foundation for ready flags, published pointers, counters, and lock-free metadata.
  - Cons: Does not make a multi-field invariant atomic by itself.
  - Scenarios: Publication flags, seqlock counters, RCU pointers, queue slot states.

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
  - Cons: Can over-synchronize and hide which ordering is actually required in C++/Rust.
  - Scenarios: Teaching baseline, first implementation before weakening to acquire/release.

- [ ] DataRaceAndDRFSC
  - Core Concept: A data-race-free program can be explained by a sequentially consistent interleaving of goroutines.
  - Pros: Defines the safe default contract before studying weaker or lock-free patterns.
  - Cons: Racy programs may still appear to work, which makes discipline and tooling necessary.
  - Scenarios: Race detector labs, happens-before education, reviewing unsynchronized shared memory.

- [ ] FenceCheatsheet
  - Core Concept: Map compiler barriers, CPU fences, acquire loads, release stores, and full barriers across x86 and ARM.
  - Pros: Makes architecture differences concrete.
  - Cons: Go does not expose fine-grained public fence APIs, so examples need careful framing.
  - Scenarios: HFT/Rust/C++ interview prep and ARM64 correctness discussion.

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
  - Pros: Shows why old Java DCL was broken and how acquire/release fixes it.
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
  - Scenarios: Rust `Arc`, C++ shared ownership, reclamation internals.

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
  - Scenarios: ABA labs, freelists, bounded deques, C++/Rust comparison.

- [ ] LitmusTests
  - Core Concept: Small programs expose allowed reorderings such as store buffering and load buffering.
  - Pros: Turns abstract memory model rules into observable experiments.
  - Cons: Results depend on hardware, compiler, and Go's atomic model.
  - Scenarios: `_lab/verify` integration, x86 vs ARM demonstrations.
