# deque Index

> Learning goal: study double-ended queues, owner-fast work stealing, and how schedulers move work between local and global queues.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: owners and stealers have different rights. Chase-Lev gets speed by letting the owner mutate one end cheaply while stealers contend on the other.
- The last item race is the key correctness point: owner pop and thief steal can target the same slot.
- Resizing introduces old-buffer lifetime concerns, which tie this package to reclamation or Go GC assumptions.
- Recommended build order from the merged TODO: mutex deque baseline, bounded ring deque, Chase-Lev, injector queue, work-stealing pool integration, then resize/ABA extensions.
- Dependencies: `parallel/` fork-join consumes this package; advanced versions depend on `memory/` and optionally `reclamation/`.
- Career signal: strong for work-stealing schedulers, fork/join pools, and parallel algorithm runtimes.
- Scope rule: prioritize portable work-stealing algorithms such as ABP and Chase-Lev; runtime-specific scheduler details should stay as comparison notes.

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
