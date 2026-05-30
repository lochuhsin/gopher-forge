# mapx Index

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

## Reference Trail and Go Boundary

- Primary Go reference: `sync.Map` documents workload assumptions and synchronizes-before guarantees (`https://pkg.go.dev/sync#Map`).
- Classic map line: Harris linked-list deletion (`https://doi.org/10.1007/3-540-45414-4_21`), split-ordered lists (`https://ldhulipala.github.io/readings/split_ordered_lists.pdf`), and Ctrie-style concurrent hash tries.
- Mental model: maps are not one primitive. Lookup, insert, delete, iteration, resize, and eviction can each have different consistency and blocking rules.
- Go boundary: start with sharded mutex/RWMutex maps and LRU before lock-free maps. Lock-free resize and delete require `memory/`, `hazard/`, or `reclamation/` vocabulary.
- Iteration boundary: "concurrent map" does not imply linearizable iteration. Snapshot, weakly consistent, and locked iteration are different APIs.
- Interview artifact: every map should document hot-key behavior, resize protocol, deletion tombstones, and whether reads mutate hidden state.

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

- [ ] JavaCHMStyle
  - Core Concept: Java 8 ConcurrentHashMap CAS-installs the first node in an empty bin, locks per-bin for collisions, and treeifies long chains, with striped resize help.
  - Pros: Strong read/write scaling without a global lock and graceful handling of hash collisions.
  - Cons: Per-bin locking, treeification, and cooperative resize make it a large state machine to get right.
  - Scenarios: FAANG concurrent-map design, bin-locking vs sharding comparison, production hash-map study.

- [ ] CliffClickNonBlockingHashMap
  - Core Concept: Cliff Click's NBHM is an open-addressed lock-free map whose slots move through a finite-state machine, with resize handled by a copy state machine.
  - Pros: CAS-only with no locks and scales across many cores on read- and write-heavy mixes.
  - Cons: The key/value state machine and incremental copy are notoriously subtle and reclamation leans on GC.
  - Scenarios: Lock-free hash-map study, state-machine concurrency, JVM map internals.

- [ ] ConcurrentCuckoo
  - Core Concept: libcuckoo (Li-Andersen-Kaminsky 2014) uses cuckoo hashing with fine-grained striped locks and optimistic, lock-free reads.
  - Pros: High load factor, predictable lookup cost, and strong read scaling.
  - Cons: Inserts may trigger cuckoo eviction paths and need careful lock ordering to avoid deadlock.
  - Scenarios: High-density caches, read-heavy lookup tables, open-addressing concurrency study.

- [ ] HopscotchHashing
  - Core Concept: Hopscotch hashing (Herlihy-Shavit-Tzafrir 2008) keeps each key within a bounded neighborhood of its home bucket, displacing entries to stay close.
  - Pros: Cache-friendly open addressing with bounded probe distance, good for concurrency.
  - Cons: Insert displacement can cascade and neighborhood bitmaps add per-bucket metadata.
  - Scenarios: Cache-conscious hash tables, open-addressing alternatives to chaining.

- [ ] CLHT
  - Core Concept: The Cache-Line Hash Table (David-Guerraoui-Trigonakis, ASPLOS 2015) sizes each bucket to one cache line so a lookup completes with at most one cache-line transfer, and updates are designed not to block concurrent lookups.
  - Pros: Minimal cache traffic for lookups, with both lock-based (CLHT-LB) and lock-free (CLHT-LF) resizable variants.
  - Cons: Cache-line bucket layout limits entries per bucket and complicates resize.
  - Scenarios: Cache-line-aware design, NUMA hash tables, scalable search-structure study.

- [ ] BwTree
  - Core Concept: The Bw-Tree (Levandoski-Lomet-Sengupta 2013) is a latch-free ordered index using delta record chains and a CAS-updated indirection mapping table.
  - Pros: Latch-free ordered map with good cache behavior, used in production storage engines.
  - Cons: Delta chains need consolidation and epoch reclamation plus the mapping table add complexity.
  - Scenarios: Database/storage index study, ordered read-mostly maps, latch-free B-tree comparison.

## Cache Eviction Policies

- [ ] CacheEvictionCLOCK
  - Core Concept: CLOCK (second-chance) approximates LRU with a circular buffer and a reference bit set on access, evicting the first entry whose bit is clear.
  - Pros: Near-LRU quality with O(1) updates and no per-hit list reordering, so reads stay cheap.
  - Cons: Only approximates recency and can retain recently touched but cold entries for an extra sweep.
  - Scenarios: Page caches, low-contention eviction, baseline for CLOCK-Pro and SIEVE.

- [ ] CacheEvictionARC
  - Core Concept: ARC (Megiddo-Modha 2003) balances a recency list and a frequency list using ghost entries to self-tune the split.
  - Pros: Scan-resistant and self-adapting without manual tuning, beating plain LRU on many traces.
  - Cons: Four lists plus ghost bookkeeping, and patent baggage pushed many systems to alternatives.
  - Scenarios: Storage/database buffer pools, adaptive cache study, eviction benchmark baseline.

- [ ] CacheEvictionLIRS
  - Core Concept: LIRS ranks entries by inter-reference recency (reuse distance) rather than last-access time to separate hot from cold blocks.
  - Pros: Strongly scan-resistant and often higher hit ratio than ARC on looping workloads.
  - Cons: Stack pruning and state maintenance are intricate.
  - Scenarios: Database/file-system caches, scan-heavy workloads, reuse-distance study.

- [ ] CacheEvictionWTinyLFU
  - Core Concept: W-TinyLFU (Caffeine, Einziger-Friedman-Manes 2017) gates a small LRU admission window with a frequency sketch so only high-frequency keys enter the main SLRU.
  - Pros: State-of-the-art hit ratio with tiny metadata via a count-min sketch plus aging.
  - Cons: Sketch sizing, aging cadence, and window ratio need tuning, more moving parts than LRU.
  - Scenarios: Application caches (Caffeine), CDN/edge caches, FAANG cache design.

- [ ] CacheEvictionS3FIFO
  - Core Concept: S3-FIFO (2023) uses a small probationary FIFO, a main FIFO, and a ghost FIFO instead of LRU lists, promoting only reused items.
  - Pros: Scales without LRU list locking and beats many policies on large production trace corpora.
  - Cons: Three-queue sizing matters and it is newer with less long-tail production history.
  - Scenarios: High-throughput key-value caches, lock-light eviction, modern cache comparison.

- [ ] CacheEvictionSIEVE
  - Core Concept: SIEVE (NSDI 2024) keeps a single FIFO with a visited bit and a moving hand that retains visited entries in place and evicts the first unvisited one.
  - Pros: Simpler than LRU, near lock-free on hits (just set a bit), with strong hit ratios and scalability.
  - Cons: Not LRU-equivalent and behavior under adversarial access is still being characterized.
  - Scenarios: Web/object caches, near-lock-free hot paths, AI-infra block pools.
