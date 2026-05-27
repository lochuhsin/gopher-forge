# queue Index

> Learning goal: study FIFO transfer under different producer/consumer profiles, from mutex baselines to bounded lock-free rings and advanced broadcast queues.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: `Enqueue(v)` must happen-before a successful `Dequeue()` that returns `v`, and FIFO order is defined by the linearization order of committed operations.
- Concurrency profile is part of the contract: SPSC, MPSC, SPMC, and MPMC are not interchangeable implementation details.
- Bounded queues must define overflow behavior: return false, block, drop, or grow. This package currently favors non-blocking bounded rings plus mutex baselines.
- Recommended build order from the merged TODO: finish `LockFreeSPSC`, then implement LMAX Disruptor, then Michael-Scott queue once reclamation exists, then intrusive MPSC, then academic wait-free/priority queues.
- Dependencies: lock-free variants depend on `memory/`; Michael-Scott depends on `hazard/` or `reclamation/`; Disruptor can stay ring-based and avoid reclamation.
- Career signal: SPSC and Disruptor are the strongest HFT/crypto signals; MPMC bounded queues are the general systems baseline.
- Transfer rule: queues here include Go channels as a mental model, but implementation topics should expose FIFO contracts, blocking policy, capacity, fairness, and linearization points.

## Implementation Checklist

- [x] MutexMPMC
  - Core Concept: A mutex protects an unbounded FIFO linked structure shared by multiple producers and consumers.
  - Pros: Simple correctness model, useful baseline, no fixed capacity.
  - Cons: All operations serialize on one mutex and contention hides queue-specific costs.
  - Scenarios: Baseline benchmarks, blocking queue extension, correctness oracle for lock-free queues.

- [x] MutexMPSC
  - Core Concept: Multiple producers enqueue under a mutex while one consumer dequeues in FIFO order.
  - Pros: Simple MPSC baseline and unbounded capacity.
  - Cons: Producer concurrency still serializes on the mutex.
  - Scenarios: Comparing lock-free MPSC against a boring version, log/event ingestion.

- [x] LockFreeMPMC
  - Core Concept: A bounded ring uses per-slot sequence numbers plus CAS on head and tail to coordinate multiple producers and consumers.
  - Pros: No mutex, fixed allocation, strong teaching example of Vyukov-style slot ownership.
  - Cons: Bounded capacity, retry loops under contention, head/tail cache lines can become hot.
  - Scenarios: Work queues, bounded handoff, lock-free ring buffer interviews.

- [x] LockFreePaddedMPMC
  - Core Concept: Same bounded MPMC algorithm, but hot atomic counters are padded to reduce false sharing.
  - Pros: Makes cache-line placement measurable and usually improves throughput under contention.
  - Cons: Larger memory footprint and still bounded.
  - Scenarios: False-sharing benchmarks, HFT/crypto ring-buffer tuning.

- [x] LockFreeMPSC
  - Core Concept: Producers CAS tail in a bounded ring while the single consumer owns head non-atomically.
  - Pros: Cheaper dequeue path than MPMC and matches many ingestion pipelines.
  - Cons: Only safe with exactly one consumer; capacity is fixed.
  - Scenarios: Network thread to strategy thread handoff, logging, telemetry ingestion.

- [~] LockFreeSPSC
  - Core Concept: A single producer and single consumer can use cached head/tail snapshots to reduce cross-core loads.
  - Pros: Highest ROI queue for low-latency systems; fewer atomics than MPSC/MPMC.
  - Cons: `lockfree_spsc.go` currently only declares the package.
  - Scenarios: HFT pipelines, market-data ingestion, thread-per-core systems.

- [ ] LamportSPSCRing
  - Core Concept: A bounded circular buffer uses producer-owned tail and consumer-owned head with acquire/release publication.
  - Pros: Minimal lock-free queue and best first memory-ordering exercise.
  - Cons: Only supports one producer and one consumer; full/empty detection must be exact.
  - Scenarios: Audio buffers, feed-handler handoff, core-to-core pipelines.

- [ ] BlockingBoundedQueue
  - Core Concept: A bounded queue parks producers when full and consumers when empty.
  - Pros: Teaches condvars, backpressure, and cancellation-safe waiting.
  - Cons: Waiter removal and shutdown semantics add complexity.
  - Scenarios: FAANG-style interview queue, producer-consumer systems.

- [ ] SynchronousQueue
  - Core Concept: A zero-capacity queue transfers an item only when producer and consumer rendezvous.
  - Pros: Strong backpressure and no buffering ambiguity.
  - Cons: Matching, fairness, timeout, and cancellation rules are harder than buffered queues.
  - Scenarios: CSP rendezvous, Java SynchronousQueue comparison, direct handoff worker pools.

- [ ] TransferQueue
  - Core Concept: Producers can enqueue normally or wait until a consumer receives a specific item.
  - Pros: Combines buffering with explicit handoff semantics.
  - Cons: API and waiter matching are complex.
  - Scenarios: Executor handoff queues, actor ask/reply, service backpressure experiments.

- [ ] LMAXDisruptor
  - Core Concept: A sequenced ring broadcasts slots to consumers that track their own progress rather than consuming items.
  - Pros: Avoids per-consumer allocation and supports dependency graphs among consumers.
  - Cons: More complex than a transfer queue; slow consumers constrain wraparound.
  - Scenarios: Matching engines, market-data fan-out, journaling/replication pipelines.

- [ ] MulticastRingBuffer
  - Core Concept: One producer publishes each slot to many consumers, and wraparound waits for the slowest gated consumer.
  - Pros: Teaches fan-out without copying messages per consumer.
  - Cons: A single stalled consumer can block reuse of the ring.
  - Scenarios: Market-data fan-out, event sourcing, Disruptor stepping stone.

- [ ] MichaelScottQueue
  - Core Concept: An unbounded linked MPMC queue uses CAS on `tail.next` and helping to advance lagging pointers.
  - Pros: Classic lock-free queue with no fixed capacity.
  - Cons: Safe node reclamation requires hazard pointers or EBR outside Go GC assumptions.
  - Scenarios: Java `ConcurrentLinkedQueue` study, lock-free helping, reclamation integration.

- [ ] VyukovIntrusiveMPSC
  - Core Concept: Producers append intrusive nodes with one atomic exchange while the single consumer drains a linked list.
  - Pros: Very fast producer path and useful systems pattern.
  - Cons: Intrusive ownership complicates API and empty/stub state has edge cases.
  - Scenarios: Runtime schedulers, logging queues, low-allocation event streams.

- [ ] WaitFreeQueue
  - Core Concept: Each operation completes in a bounded number of steps using announcements and helping.
  - Pros: Strongest progress guarantee.
  - Cons: High constant factor and much more complex than lock-free queues.
  - Scenarios: Academic study and wait-free progress comparison.

- [ ] SPMCRing
  - Core Concept: A single producer publishes work into a ring while multiple consumers claim slots.
  - Pros: Useful complement to MPSC and exposes consumer-side contention.
  - Cons: Correct slot claiming and ordering are more complex than SPSC.
  - Scenarios: Fan-out work distribution, schedulers, queue taxonomy completeness.

- [ ] LockFreePriorityQueue
  - Core Concept: A concurrent skip-list or heap-like structure supports priority-ordered dequeue.
  - Pros: Combines ordering with non-blocking progress.
  - Cons: Delete-min and reclamation are difficult.
  - Scenarios: Schedulers, timers, advanced lock-free data-structure study.
