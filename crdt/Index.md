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

- [ ] BoundedCounterEscrow
  - Core Concept: An escrow/bounded counter pre-divides decrement rights among replicas so a non-negative invariant holds without coordination on each decrement.
  - Pros: Enforces limits like stock or quota locally while staying convergent.
  - Cons: Needs rebalancing of reservations when a replica runs out, and bounds reduce local availability.
  - Scenarios: Inventory, quotas, rate budgets, exchange position limits.

- [ ] LogootLSEQ
  - Core Concept: Logoot (Weiss 2009) and LSEQ assign dense, ordered position identifiers between elements so inserts need no central sequence.
  - Pros: Supports concurrent ordered insertion with convergence and no tombstone for ordering.
  - Cons: Identifier length can grow with edits, and the allocation strategy strongly affects size.
  - Scenarios: Collaborative text, ordered lists, sequence-CRDT comparison.

- [ ] YATA
  - Core Concept: YATA (Yjs) links each element to its origin neighbors and resolves concurrent inserts deterministically, powering the Yjs library.
  - Pros: Fast, production-proven collaborative editing with low metadata per element.
  - Cons: Conflict-resolution rules and garbage collection of deleted items are intricate.
  - Scenarios: Real-time collaborative editors, Yjs-style documents, CRDT sequence study.

- [ ] Fugue
  - Core Concept: Fugue (Weidner-Kleppmann 2023) is a sequence CRDT that provably avoids the interleaving anomalies of RGA and Logoot.
  - Pros: Guarantees non-interleaving of concurrent insertions for cleaner merged text.
  - Cons: Newer and less battle-tested than RGA/YATA in production tooling.
  - Scenarios: Collaborative editing correctness, interleaving-anomaly study, modern sequence CRDTs.

- [ ] PureOpBasedCRDT
  - Core Concept: Pure op-based CRDTs (Baquero-Almeida-Shoker 2014) ship only operations over a tagged reliable causal broadcast and keep a partially ordered log.
  - Pros: Minimal metadata on the wire and a clean separation of delivery from datatype logic.
  - Cons: Requires exactly-once causal broadcast and PO-log compaction to bound growth.
  - Scenarios: Op-based set/map design, causal-broadcast integration, bandwidth-sensitive replication.

- [ ] CausalStabilityGC
  - Core Concept: Collect tombstones and PO-log entries only once an update is causally stable, meaning every replica has observed it.
  - Pros: Bounds otherwise unbounded metadata growth in op-based and OR-style CRDTs safely.
  - Cons: Needs membership knowledge and a stability detector, which is hard under churn.
  - Scenarios: Tombstone reclamation, op-log compaction, long-lived CRDT replicas.
