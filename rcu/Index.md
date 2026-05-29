# rcu Index

> Learning goal: separate publication from reclamation and make reads extremely cheap by shifting synchronization cost to writers and grace-period tracking.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: RCU is a publication and grace-period protocol, not a lock. Readers see a stable version while writers publish a replacement and defer old-version reclamation.
- Writer flow: copy old state, mutate new state, release-publish pointer, wait for a grace period, then reclaim the old version.
- Go cannot directly model classic Linux preempt-disable RCU; the practical implementation should be URCU-style explicit read sections.
- Recommended build order from the merged TODO: URCU, SRCU, QSBR-backed RCU, then classic Linux RCU as explanatory documentation.
- Dependencies: depends on `memory/` and often `reclamation/`; replaces the no-op `syncx.RCULock` concept with a full package.
- Career signal: strongest for read-mostly database, kernel, AI infra, and crypto validator systems.
- Scope rule: implement userspace-style explicit read sections in Go; Linux preemption/interrupt semantics are reference material, not required code.

## Implementation Checklist

- [ ] RCUReadSection
  - Core Concept: Readers enter and exit explicit critical sections while dereferencing published pointers.
  - Pros: Makes the read-side lifetime visible to grace-period logic.
  - Cons: In Go this cannot be pure Linux-style preempt-disable; explicit bookkeeping is required.
  - Scenarios: Config snapshots, routing tables, read-mostly maps.

- [ ] AssignPointer
  - Core Concept: Writers publish a new pointer with release semantics after fully initializing it.
  - Pros: Clean publication protocol.
  - Cons: Does not by itself solve old-version reclamation.
  - Scenarios: Atomic config reload, service discovery snapshots.

- [ ] Dereference
  - Core Concept: Readers acquire or consume the published pointer before reading the pointed data.
  - Pros: Minimal read API and direct connection to memory ordering.
  - Cons: Must be used inside a read-side critical section if reclamation matters.
  - Scenarios: RCU maps, copy-update structures.

- [ ] RCUPointer
  - Core Concept: A typed pointer wrapper pairs publish, dereference, and retire operations under one API.
  - Pros: Prevents ad hoc atomic pointer use from bypassing grace-period rules.
  - Cons: Generic pointer wrappers still cannot enforce read-section lifetime statically in Go.
  - Scenarios: Config snapshots, RCU map roots, routing-table swaps.

- [ ] URCU
  - Core Concept: Userspace RCU tracks per-participant counters to detect readers that were active during publication.
  - Pros: Practical Go-compatible model compared with kernel RCU.
  - Cons: Reader registration and goroutine identity are difficult.
  - Scenarios: Go RCU implementation, userspace routing/config tables.

- [ ] SynchronizeRCU
  - Core Concept: A writer waits until all readers active at the start of the grace period have exited.
  - Pros: Provides the proof needed to free old versions.
  - Cons: Writer latency can be high and blocked readers delay updates.
  - Scenarios: Hot reload, routing table replacement, retired snapshot freeing.

- [ ] CallRCU
  - Core Concept: Queue callbacks to run after a grace period instead of blocking the writer.
  - Pros: Asynchronous reclamation keeps writer latency lower.
  - Cons: Requires callback queue management and worker scheduling.
  - Scenarios: Kernel-style deferred frees and background cleanup.

- [ ] RCUBarrier
  - Core Concept: Wait until all queued `CallRCU` callbacks that were pending before the barrier have completed.
  - Pros: Makes shutdown and tests deterministic.
  - Cons: Requires callback generation tracking and worker lifecycle coordination.
  - Scenarios: Graceful shutdown, test cleanup, deferred-free flushes.

- [ ] SRCU
  - Core Concept: Sleepable RCU allows readers to block inside critical sections using per-domain counters.
  - Pros: Works for long or blocking read sections.
  - Cons: Reader path is heavier than classic RCU.
  - Scenarios: AI inference routing, long decode-step readers, service-side snapshots.

- [ ] QSBRRCU
  - Core Concept: Readers report quiescent states and writers wait for all participants to pass a safe point.
  - Pros: Zero read-side cost inside critical paths.
  - Cons: Requires every participant to cooperate.
  - Scenarios: Event loops, actors, thread-per-core systems.

- [ ] LeftRight
  - Core Concept: Maintain two copies and flip active side while waiting for readers on the old side to drain.
  - Pros: Very cheap reads and simpler than full RCU for some maps.
  - Cons: Doubles memory and write path updates both sides.
  - Scenarios: Read-mostly maps and routing tables.

- [ ] RCUList
  - Core Concept: Linked-list updates publish new links while removed nodes are reclaimed after a grace period.
  - Pros: Classic RCU teaching structure and bridge to maps and routing tables.
  - Cons: Deletion and traversal invariants are subtle without typed helpers.
  - Scenarios: Subscriber lists, routing entries, lock-free list comparison.

- [ ] RCUMap
  - Core Concept: Writers copy and publish a new map while readers use the current immutable snapshot.
  - Pros: Simple API with extremely cheap reads.
  - Cons: Copy cost grows with map size and write frequency.
  - Scenarios: Config maps, symbol routing, feature flag snapshots.

- [ ] RCUCorrectnessTests
  - Core Concept: Verify that old versions are not reclaimed until all pre-existing read-side sections have exited.
  - Pros: Tests the actual safety property rather than only API shape.
  - Cons: Requires deterministic hooks to hold readers across writer publication.
  - Scenarios: `_lab/verify` integration, grace-period debugging, QSBR/URCU comparison.

- [ ] CallRCUBatching
  - Core Concept: Batch many deferred `CallRCU` callbacks into one grace period so reclamation cost is amortized across retirements.
  - Pros: Keeps writer latency low and reduces grace-period overhead under heavy update rates.
  - Cons: Requires generation tracking and a flush/barrier path so shutdown and tests stay deterministic.
  - Scenarios: High-churn RCU maps/lists, deferred-free throughput tuning.

- [ ] ClassicLinuxRCUModel
  - Core Concept: Document the kernel preempt-disable read side where a grace period is the moment every CPU has passed through a context switch.
  - Pros: Anchors userspace RCU against the canonical design and explains quiescent-state intuition.
  - Cons: Go has no preempt-disable or per-CPU context-switch signal, so this is reference material, not runnable code.
  - Scenarios: Kernel-style RCU explanation, URCU/QSBR motivation, interview depth on read-mostly systems.
