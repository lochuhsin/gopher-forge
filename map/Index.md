# map Index

> Learning goal: compare concurrent lookup/update strategies, from sharded locks to read-optimized maps and non-blocking hash tables.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: each map variant must define its consistency model for lookup, insert, delete, iteration, and resize.
- The low-risk path is sharded locking first; lock-free maps should wait until `memory/` and reclamation are in better shape.
- `sync.Map`-style designs are read-optimized, not universally faster maps. The workload shape matters.
- Recommended build order from the merged TODO: sharded mutex map, thread-safe LRU, `sync.Map` clone, striped RWMutex map, then lock-free open addressing/split-ordered/skip-list variants.
- Dependencies: uses `syncx/` lock variants for baselines; lock-free variants depend on `memory/` and `reclamation/`; RCU map depends on `rcu/`.
- Career signal: very high for FAANG-style interviews because thread-safe map and LRU are common senior coding tasks.
- Scope rule: separate consistency models explicitly: linearizable operations, weak iteration, snapshot reads, eventual cleanup, and resize behavior.

## Implementation Checklist

- [ ] ShardedMutexMap
  - Core Concept: Hash keys into shards, each guarded by its own mutex.
  - Pros: Simple, scalable enough for many workloads, and easy to reason about.
  - Cons: Hot keys still serialize and resize across shards needs coordination.
  - Scenarios: Service caches, order-book shards, thread-safe map interview baseline.

- [ ] StripedRWMutexMap
  - Core Concept: Use RWMutex per stripe so reads share access while writes lock one stripe.
  - Pros: Good read-heavy baseline without exotic atomics.
  - Cons: Writer starvation and lock ordering issues appear during multi-key operations.
  - Scenarios: Read-mostly configuration maps and comparison with `sync.Map`.

- [ ] SyncMapClone
  - Core Concept: Maintain a read-only snapshot plus dirty map, promoting dirty state after misses.
  - Pros: Excellent for read-mostly or write-once/read-many keys.
  - Cons: Poor fit for hot overwrite-heavy keys; promotion logic is subtle.
  - Scenarios: Rebuilding Go `sync.Map`, cache internals, read-copy tradeoffs.

- [ ] CopyOnWriteMap
  - Core Concept: Writers copy the current immutable map, mutate the copy, and atomically publish the new root.
  - Pros: Lock-free reads and very simple snapshot semantics.
  - Cons: Expensive writes and full-map copies make it unsuitable for hot update paths.
  - Scenarios: Routing tables, feature flags, configuration snapshots.

- [ ] ThreadSafeLRU
  - Core Concept: Combine a hash map with a recency list under synchronization.
  - Pros: High interview value and practical service primitive.
  - Cons: Every hit mutates recency state, so read operations are not purely read-only.
  - Scenarios: API caches, idempotency stores, FAANG-style design/coding task.

- [ ] LockFreeOpenAddressingMap
  - Core Concept: Probe an array of buckets and use CAS to install/update entries.
  - Pros: Allocation-light and cache-friendly when load factor is controlled.
  - Cons: Deletion tombstones, resizing, and memory reclamation are hard.
  - Scenarios: Advanced lock-free hash table study and Robin Hood hashing.

- [ ] SplitOrderedListMap
  - Core Concept: Use ordered linked buckets that can be initialized lazily as the table grows.
  - Pros: Incremental resizing and classic non-blocking hash map design.
  - Cons: Requires linked-list CAS and reclamation support.
  - Scenarios: Lock-free map literature and scalable hash table internals.

- [ ] LockFreeSkipListMap
  - Core Concept: A probabilistic ordered list supports concurrent search, insert, and delete.
  - Pros: Naturally ordered and can back priority/range queries.
  - Cons: Delete marking and node reclamation are subtle.
  - Scenarios: Concurrent ordered maps, priority queues, database index intuition.

- [ ] LeftRightMap
  - Core Concept: Readers use one active copy while writers update the inactive copy and flip sides after readers drain.
  - Pros: Very cheap reads with a simpler proof than general-purpose lock-free maps.
  - Cons: Doubles storage and writes must update both sides eventually.
  - Scenarios: Read-mostly state, RCU comparison, low-latency lookup tables.

- [ ] RCUMap
  - Core Concept: Readers access immutable snapshots while writers copy, mutate, and publish new roots.
  - Pros: Very cheap reads and simple reader code.
  - Cons: Writes copy more data and old snapshots require grace-period reclamation.
  - Scenarios: Config maps, routing tables, read-mostly state.

- [ ] ConcurrentResizeProtocol
  - Core Concept: Map growth or rehashing is coordinated without stopping all readers.
  - Pros: Turns resize from a hidden footgun into an explicit design dimension.
  - Cons: Migration, tombstones, and helping paths are complex.
  - Scenarios: Lock-free hash maps, sharded map growth, split-ordered lists.
