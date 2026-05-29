# crdt Index

> Learning goal: design replicated data types whose merge operations are commutative, associative, and idempotent so replicas converge without coordination.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: merge must be commutative, associative, and idempotent so replicas converge despite reordering and duplicate delivery.
- State-based CRDTs send state and merge by join; op-based CRDTs send operations and require reliable causal delivery.
- Many CRDTs need metadata from `clock/`, usually replica IDs, counters, vector clocks, dots, or HLC timestamps.
- Recommended build order from the merged TODO: G-Counter, PN-Counter, G-Set, OR-Set, LWW Register, OR-Map, RGA/LSEQ, then delta-state replication.
- Dependencies: uses `clock/` for timestamps/causality and `_lab/verify` for algebraic property checks.
- Career signal: strongest for collaborative editing, geo-replicated state, crypto distributed state, and FAANG system-design depth.
- Scope rule: CRDTs are not low-level synchronization primitives; keep them as optional coordination-free concurrency and distributed-state training.

## Reference Trail and Go Boundary

- Primary CRDT references: Shapiro et al. CRDT overview (`https://arxiv.org/abs/1805.06358`) and the CRDT paper catalog (`https://crdt.tech/papers.html`).
- Mental model: CRDT correctness is algebra first, code second. Merge must be commutative, associative, and idempotent, or convergence is accidental.
- Go boundary: CRDTs do not replace local synchronization. A replica implementation still needs ordinary Go safety for its local map/set/list state.
- Causality boundary: state-based CRDTs can tolerate duplicate/reordered full state; op-based CRDTs need causal delivery or metadata that makes missing dependencies explicit.
- Metadata boundary: tombstones, dots, vector clocks, and HLC timestamps are part of the data type, not incidental implementation details.
- Interview artifact: every CRDT should ship with property tests for merge laws and at least one concurrent-update example that explains the chosen conflict semantics.

## Implementation Checklist

- [ ] GCounter
  - Core Concept: Each replica owns a grow-only counter slot; merge takes element-wise max and value is the sum.
  - Pros: Simplest state-based CRDT and easy to prove.
  - Cons: Only supports increments and metadata grows with replicas.
  - Scenarios: Distributed counters, CRDT introduction, semilattice proof.

- [ ] PNCounter
  - Core Concept: Combine two G-Counters, one for increments and one for decrements.
  - Pros: Supports positive and negative updates while preserving convergence.
  - Cons: Doubles metadata and still requires replica identity.
  - Scenarios: Inventory deltas, score counters, quota examples.

- [ ] GSet
  - Core Concept: A grow-only set merges by union.
  - Pros: Trivial convergence and no tombstones.
  - Cons: Cannot remove elements.
  - Scenarios: Seen IDs, feature flags that only enable, CRDT baseline.

- [ ] TwoPhaseSet
  - Core Concept: Maintain add-set and remove-set; membership is add minus remove.
  - Pros: Adds removal while keeping merge as union.
  - Cons: Once removed, an element cannot be re-added.
  - Scenarios: Tombstone teaching and remove semantics comparison.

- [ ] ORSet
  - Core Concept: Each add creates a unique tag; remove deletes only observed tags.
  - Pros: Supports add/remove under concurrency without permanent remove bans.
  - Cons: Tag tombstones and compaction are required.
  - Scenarios: Shopping carts, membership sets, collaborative state.

- [ ] LWWRegister
  - Core Concept: Store the value with the greatest timestamp, usually from HLC or wall clock plus tie-breaker.
  - Pros: Simple conflict resolution and small metadata.
  - Cons: Concurrent writes can lose data and bad clocks corrupt semantics.
  - Scenarios: Last-write-wins configs and replicated key-value stores.

- [ ] MVRegister
  - Core Concept: Preserve all concurrently written values and collapse only causally overwritten values.
  - Pros: Does not silently discard conflicts.
  - Cons: Application must resolve multiple values.
  - Scenarios: Dynamo-style conflict handling and user-visible merges.

- [ ] ORMap
  - Core Concept: Keys are tracked with OR-Set semantics and values are themselves CRDTs.
  - Pros: Composes CRDTs into nested replicated objects.
  - Cons: Remove/update races and nested tombstones are subtle.
  - Scenarios: Replicated documents, object stores, Sui-like object state.

- [ ] CausalDeliveryBuffer
  - Core Concept: Hold op-based CRDT operations until their causal predecessors are present.
  - Pros: Shows where vector clocks become protocol machinery.
  - Cons: Buffer growth and membership changes require explicit policy.
  - Scenarios: Op-based OR-Set, collaborative editing, replicated actor logs.

- [ ] LWWMap
  - Core Concept: Map entries use LWW registers for conflict resolution.
  - Pros: Simple practical map for many systems.
  - Cons: Loses concurrent updates and depends on clock quality.
  - Scenarios: Feature flags, service discovery, replicated metadata.

- [ ] RGAList
  - Core Concept: Insert elements after stable identifiers and merge by causal/order metadata.
  - Pros: Supports collaborative sequence editing.
  - Cons: Identifier growth, tombstones, and ordering rules are complex.
  - Scenarios: Google Docs/Notion-style collaborative text.

- [ ] DeltaStateReplication
  - Core Concept: Send join-preserving deltas instead of the full state.
  - Pros: Reduces bandwidth while keeping state-based convergence.
  - Cons: Delta buffering and anti-entropy protocol are harder.
  - Scenarios: Large replicated sets/maps and P2P gossip.

- [ ] CRDTAlgebraPropertyTests
  - Core Concept: Verify merge is commutative, associative, and idempotent under generated states.
  - Pros: Catches convergence bugs independently of network schedules.
  - Cons: Requires good generators and equality normalization.
  - Scenarios: `_lab/verify` integration, semilattice education, regression tests.

- [ ] AntiEntropyGossip
  - Core Concept: Replicas periodically exchange state, deltas, or digests until all converge.
  - Pros: Connects CRDT merge laws to real synchronization protocols.
  - Cons: Membership, bandwidth, and duplicate suppression are nontrivial.
  - Scenarios: P2P replication, distributed caches, eventually consistent services.
