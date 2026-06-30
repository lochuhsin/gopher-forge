# deque Index

> Learning goal: study double-ended queues, owner-fast work stealing, and how schedulers move work between local and global queues.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: owners and stealers have different rights. Chase-Lev gets speed by letting the owner mutate one end cheaply while stealers contend on the other.
- Optimizes: a single shared run queue (one mutex/cond — e.g. `syncx`'s worker pool) serializes every worker on one lock, so throughput stops scaling with cores. Work stealing removes that central contention — each worker owns a deque (owner push/pop is the lock-free fast path) and only touches a peer's deque when idle (the steal slow path). Wins: no central-lock contention on the common path, plus cache/affinity locality for subtasks a worker spawns for itself. This is the direct upgrade from the cond-based single-queue pool.
- The last item race is the key correctness point: owner pop and thief steal can target the same slot.
- Resizing introduces old-buffer lifetime concerns, which tie this package to reclamation or Go GC assumptions.
- Recommended build order from the merged TODO: mutex deque baseline, bounded ring deque, Chase-Lev, injector queue, work-stealing pool integration, then resize/ABA extensions.
- Dependencies: `parallel/` fork-join consumes this package; advanced versions depend on `memory/` and optionally `reclamation/`.
- Career signal: strong for work-stealing schedulers, fork/join pools, and parallel algorithm runtimes.
- Scope rule: prioritize portable work-stealing algorithms such as ABP and Chase-Lev; runtime-specific scheduler details should stay as comparison notes.

## Reference Trail and Go Boundary

- Classic deque line: ABP work stealing (`https://www.cs.cmu.edu/~guyb/paralg/papers/AroraBlumofePlaxton01.pdf`) and Chase-Lev dynamic circular work-stealing deque (`https://www.cs.wm.edu/~dcschmidt/PDF/work-stealing-dequeue.pdf`).
- Mental model: one owner has cheap bottom operations; thieves contend on top. The last element is the only place where owner and thief race for the same logical item.
- Go boundary: Go cannot portably pin goroutines to worker threads, so model workers explicitly instead of assuming runtime scheduler affinity.
- Resize boundary: growing the circular array creates old-buffer lifetime issues. Under Go GC this is easier mechanically, but the design notes should still name the reclamation proof.
- Progress boundary: work stealing is about scheduler throughput and load balance, not just a deque API; include victim policy and shutdown semantics when it becomes a pool.
- Interview artifact: every Chase-Lev explanation should include the empty case, multi-item case, and single-item CAS race.

## Implementation Checklist

- [ ] MutexDeque
  - Core Concept: A mutex guards push/pop operations at both front and back.
  - Pros: Simple correctness baseline for all deque variants.
  - Cons: No owner/stealer fast path and all operations serialize.
  - Scenarios: Baseline tests for Chase-Lev and bounded ring deques.

- [ ] BoundedRingDeque
  - Core Concept: A circular buffer supports push/pop from both ends with capacity limits.
  - Pros: Clear index arithmetic and no per-item allocation.
  - Cons: Concurrent front/back access needs careful synchronization.
  - Scenarios: Fixed worker queues and stepping stone to Chase-Lev.

- [ ] ChaseLevDeque
  - Core Concept: The owner pushes and pops from the bottom, while stealers CAS from the top.
  - Pros: Fast owner path and proven work-stealing scheduler primitive.
  - Cons: Races around the last item and resizing require precise memory ordering.
  - Scenarios: Fork/join pools, scheduler comparison, work-stealing deque study.

- [ ] ABPDeque
  - Core Concept: Arora-Blumofe-Plaxton work stealing uses a bounded array with owner bottom operations and thief top steals.
  - Pros: Historical foundation for Cilk-style schedulers and simpler than fully resizable Chase-Lev.
  - Cons: Fixed capacity and last-item races still require careful ordering.
  - Scenarios: Work-stealing literature, fork/join scheduler progression, comparing ABP vs Chase-Lev.

- [ ] WorkStealingInjector
  - Core Concept: A global injection queue receives external work and workers steal from it when local queues empty.
  - Pros: Balances load across workers and separates local from external scheduling.
  - Cons: Global injector can become a bottleneck.
  - Scenarios: Thread pools, task schedulers, actor runtime integration.

- [ ] WorkStealingPool
  - Core Concept: Workers own deques and steal from peers when idle.
  - Pros: Good locality for spawned subtasks and dynamic load balancing.
  - Cons: Shutdown, cancellation, and fairness are complex.
  - Scenarios: Parallel map/reduce, fork/join, async executor experiments.

- [ ] ResizableChaseLevDeque
  - Core Concept: The owner grows the circular array while stealers safely observe old arrays.
  - Pros: Avoids fixed capacity while preserving fast local operations.
  - Cons: Old array reclamation needs GC assumptions or EBR.
  - Scenarios: Production-like scheduler queues and reclamation integration.

- [ ] ABAProtectedDeque
  - Core Concept: Versioned indices or reclamation prevent stale steal observations from succeeding incorrectly.
  - Pros: Makes correctness arguments explicit under wraparound and resize.
  - Cons: Adds state width and proof complexity.
  - Scenarios: Advanced work-stealing correctness and memory model study.

- [ ] StealPolicyExperiments
  - Core Concept: Workers choose victims by random, round-robin, local-neighbor, or load-aware policies.
  - Pros: Makes scheduling fairness and locality measurable.
  - Cons: Results depend on workload shape and benchmark design.
  - Scenarios: Parallel runtime tuning, irregular workloads, actor scheduler integration.

- [ ] CilkTHEProtocol
  - Core Concept: The Cilk THE work-stealing protocol (Blumofe-Leiserson) lets the owner push/pop the tail with plain loads/stores and falls back to a lock only when a thief contends the last item.
  - Pros: Near-zero owner overhead on the common path, the historical foundation Chase-Lev later refined.
  - Cons: The last-item exception path needs a lock and careful ordering, and it predates fully lock-free deques.
  - Scenarios: Fork/join runtime study, work-stealing history, Chase-Lev motivation.

- [ ] IdempotentWorkStealing
  - Core Concept: Michael-Vechev-Saraswat (2009) relax work-stealing so a task may be extracted more than once, making owner operations cheap with only stores.
  - Pros: Cheapest owner path of the work-stealing family when tasks are idempotent.
  - Cons: Requires idempotent tasks because the same item can run twice; not a strict deque.
  - Scenarios: Idempotent task schedulers, relaxed work-stealing study, throughput-first runtimes.

- [ ] BWoSDeque
  - Core Concept: Block-based Work Stealing partitions the deque into blocks so owner and thieves touch different blocks, slashing steal contention.
  - Pros: Near-zero contention between owner and thieves and strong scaling for modern schedulers.
  - Cons: Block management and cross-block transitions add bookkeeping.
  - Scenarios: High-core work-stealing runtimes, scheduler contention tuning.

- [ ] StealHalf
  - Core Concept: A thief takes roughly half the victim's tasks per steal instead of one, amortizing the cost of contended steals.
  - Pros: Fewer steal attempts and better load spreading for bursty workloads.
  - Cons: Larger atomic transfer and trickier ownership split at the steal boundary.
  - Scenarios: Irregular parallel workloads, steal-frequency reduction experiments.
