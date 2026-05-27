# clock Index

> Learning goal: turn happens-before and causality into concrete timestamps that can be compared, merged, and printed.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core question: are two events causally ordered or concurrent? Wall-clock time alone cannot answer this reliably.
- Lamport clocks give a causal-compatible scalar order but can falsely order concurrent events; vector clocks detect concurrency exactly at O(n) metadata.
- HLC adds wall-clock proximity while preserving monotonic causal updates.
- Recommended build order from the merged TODO: Lamport clock, vector clock, causal broadcast, HLC, matrix clock, then interval/bloom-style advanced clocks if needed.
- Dependencies: consumed by `crdt/` and `_lab/verify`; conceptually mirrors happens-before reasoning in `memory/`.
- Career signal: strongest for distributed databases, crypto/L1 systems, causal broadcast, and collaborative systems.
- Scope rule: this package covers both logical causality clocks and deterministic local clocks needed to test time-based concurrency.

## Implementation Checklist

- [ ] LamportClock
  - Core Concept: Increment a scalar counter on local/send events and merge by `max(local, remote)+1` on receive.
  - Pros: Tiny, deterministic, and establishes a total order when paired with replica ID.
  - Cons: Cannot distinguish true causality from concurrent events.
  - Scenarios: Event ordering, Block-STM-style versions, distributed systems intro.

- [ ] VectorClock
  - Core Concept: Track one counter per replica and compare vectors element-wise.
  - Pros: Correctly detects happens-before vs concurrent events.
  - Cons: O(number of replicas) space and merge cost.
  - Scenarios: Race detectors, causal delivery, CRDT metadata.

- [ ] SparseVectorClock
  - Core Concept: Store only non-zero or active replica counters.
  - Pros: More practical when replica sets are large but sparse.
  - Cons: Comparison logic and tombstone cleanup are more complex.
  - Scenarios: Dynamic distributed systems and CRDT replicas.

- [ ] HybridLogicalClock
  - Core Concept: Combine physical time with a logical counter to remain monotonic while staying close to wall time.
  - Pros: Useful for databases and logs that need causal-ish timestamps.
  - Cons: Clock skew and persistence across restarts require care.
  - Scenarios: CockroachDB-style timestamps, audit logs, cross-service ordering.

- [ ] MatrixClock
  - Core Concept: Each process tracks what it believes every other process knows.
  - Pros: Can infer global knowledge and garbage-collect delivered events.
  - Cons: O(n^2) metadata.
  - Scenarios: Gossip protocols and distributed garbage collection.

- [ ] VersionVector
  - Core Concept: A vector clock variant tracks versions per replica for an object or key.
  - Pros: Natural fit for replicated key-value stores.
  - Cons: Metadata grows with replica count and requires compaction.
  - Scenarios: Dynamo-style conflict detection and CRDT versions.

- [ ] DottedVersionVector
  - Core Concept: Represent a causal context plus a single event dot for precise update identity.
  - Pros: More compact and precise for OR-Set/OR-Map style CRDTs.
  - Cons: Harder to explain and implement than plain vectors.
  - Scenarios: Advanced CRDT metadata and causal compaction.

- [ ] CausalBroadcastMetadata
  - Core Concept: Use vector clocks to deliver messages only when causal predecessors are present.
  - Pros: Demonstrates clocks as protocol machinery, not just data.
  - Cons: Buffers out-of-order messages and needs membership rules.
  - Scenarios: Chat, collaborative editing, replicated logs.

- [ ] ClockSkewSimulator
  - Core Concept: Simulate physical clock jumps, drift, and NTP corrections.
  - Pros: Makes HLC and wall-clock failure modes concrete.
  - Cons: Simulation must be controlled to produce useful examples.
  - Scenarios: Testing HLC monotonicity and timestamp assumptions.

- [ ] ManualClock
  - Core Concept: Tests advance time explicitly instead of sleeping on wall-clock timers.
  - Pros: Makes deadlines, rate limiters, and cancellation deterministic and fast.
  - Cons: Code must consistently depend on the injected clock interface.
  - Scenarios: `ratelimit/`, `scope/DeadlineScheduler`, timeout tests.

- [ ] TimerWheel
  - Core Concept: Timers are bucketed by expiration tick to reduce scheduling overhead for many deadlines.
  - Pros: Efficient for large numbers of approximate timers.
  - Cons: Granularity and long-duration handling require careful design.
  - Scenarios: Deadline schedulers, rate limiter waits, actor timeouts.

- [ ] DeadlineHeap
  - Core Concept: A min-heap orders timers by earliest deadline.
  - Pros: Precise and straightforward for moderate timer counts.
  - Cons: Insert/cancel are O(log n) and cancellation cleanup can leave tombstones.
  - Scenarios: Task timeouts, future deadlines, retry scheduling.

- [ ] TickerAndTimerAbstraction
  - Core Concept: Wrap one-shot timers and periodic tickers behind interfaces that can be faked in tests.
  - Pros: Prevents flaky sleeps and centralizes timer cleanup rules.
  - Cons: API design must avoid leaking timers or goroutines.
  - Scenarios: Periodic maintenance, rate limiter refill, actor heartbeats.
