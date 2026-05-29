# queue Index

> Learning goal: study FIFO transfer under different producer/consumer profiles, from mutex baselines to bounded lock-free rings and advanced broadcast queues.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: `Enqueue(v)` must happen-before a successful `Dequeue()` that returns `v`, and FIFO order is defined by the linearization order of committed operations.
- Concurrency profile is part of the contract: SPSC, MPSC, SPMC, and MPMC are not interchangeable implementation details.
- Bounded queues must define overflow behavior: return false, block, drop, or grow. This package currently favors non-blocking bounded rings plus mutex baselines.
- Recommended build order from the merged TODO: build the LMAX Disruptor next, then Michael-Scott queue once reclamation exists, then intrusive MPSC, then academic wait-free/priority queues.
- Dependencies: lock-free variants depend on `memory/`; Michael-Scott depends on `hazard/` or `reclamation/`; Disruptor can stay ring-based and avoid reclamation.
- Career signal: SPSC and Disruptor are the strongest HFT/crypto signals; MPMC bounded queues are the general systems baseline.
- Transfer rule: queues here include Go channels as a mental model, but implementation topics should expose FIFO contracts, blocking policy, capacity, fairness, and linearization points.

## Reference Trail and Go Boundary

- Classic queue line: Lamport-style SPSC rings, LMAX Disruptor (`https://lmax-exchange.github.io/disruptor/user-guide/`), Michael-Scott queues (`https://www.cs.rochester.edu/research/synchronization/pubs.shtml`), and Vyukov bounded MPMC rings (`https://docs.ros.org/en/kinetic/api/ros_opcua_impl_freeopcua/html/mpmc__bounded__q_8h_source.html`).
- SPSC mental model: fixed-size ring first. Dynamic growth is a different problem because resize changes ownership, reclamation, and full/empty invariants.
- Multiplicity rule: SPSC uses producer-owned tail and consumer-owned head; MPSC adds contention only on producer claim; MPMC adds contention on both claim sides and usually needs per-slot sequence numbers.
- Go boundary: use `sync/atomic` for sequence publication and CAS-based slot claims. Channels are a semantic comparison point, not the implementation of lock-free queue items.
- Reclamation boundary: linked unbounded queues should wait for `hazard/` or `reclamation/` even though Go GC keeps nodes alive; the study goal is to understand the non-GC proof.
- Interview artifact: for every queue, document capacity policy, linearization point, progress guarantee, and slow-consumer behavior before benchmarking throughput.

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

- [x] LockFreeSPSC
  - Core Concept: A single producer and single consumer can use cached head/tail snapshots to reduce cross-core loads.
  - Pros: Highest ROI queue for low-latency systems; fewer atomics than MPSC/MPMC; implemented by `CachedSPSCQueue` with SPSC correctness, chaos, benchmark, padding, and cache-refresh coverage.
  - Cons: Only safe with exactly one producer and one consumer; capacity is fixed.
  - Scenarios: HFT pipelines, market-data ingestion, thread-per-core systems.

- [x] LamportSPSCRing
  - Core Concept: A bounded circular buffer uses producer-owned tail and consumer-owned head with acquire/release publication.
  - Pros: Minimal lock-free queue and best first memory-ordering exercise; implemented by `SPSCQueue` and covered by empty/full/FIFO/wraparound/concurrent SPSC tests.
  - Cons: Only supports one producer and one consumer; full/empty detection must be exact.
  - Scenarios: Audio buffers, feed-handler handoff, core-to-core pipelines.

- [ ] BlockingBoundedQueue
  - Core Concept: A bounded queue parks producers when full and consumers when empty.
  - Pros: Teaches condvars, backpressure, and cancellation-safe waiting.
  - Cons: Waiter removal and shutdown semantics add complexity.
  - Scenarios: FAANG-style interview queue, producer-consumer systems.

- [ ] BoundedChannelPolicy
  - Core Concept: A bounded channel-like queue declares what writers do when capacity is full: wait, fail, drop-oldest, drop-newest, drop-write, or replace-latest.
  - Pros: Turns overload behavior into an explicit API contract and aligns queue tests with backpressure design.
  - Cons: Each policy changes delivery guarantees, fairness, and memory visibility expectations.
  - Scenarios: Actor mailboxes, telemetry buffers, market-data fan-out, inference admission queues.

- [ ] LossyBoundedQueue
  - Core Concept: A bounded queue intentionally drops or replaces items under pressure according to `BoundedChannelPolicy`.
  - Pros: Useful when stale work is worse than missing work, and it gives concrete benchmarks for slow-consumer policy.
  - Cons: Not linearizable as a normal FIFO unless dropped items are part of the specified history.
  - Scenarios: Latest quote streams, metrics, logs, UI/event state, load shedding.

- [ ] SynchronousQueue
  - Core Concept: A zero-capacity queue transfers an item only when producer and consumer rendezvous.
  - Pros: Strong backpressure and no buffering ambiguity.
  - Cons: Matching, fairness, timeout, and cancellation rules are harder than buffered queues.
  - Scenarios: CSP rendezvous, zero-capacity channel comparison, direct handoff worker pools.

- [ ] OwnershipTransferQueue
  - Core Concept: Enqueue hands off logical ownership of an item to the queue and eventually to the consumer.
  - Pros: Makes "share by communication" concrete and avoids shared mutable payloads.
  - Cons: Payload aliasing outside the queue can violate the ownership story unless APIs restrict it.
  - Scenarios: Actor messages, pipeline stages, work queues, resource handoff.

- [ ] OneShotReplyQueue
  - Core Concept: A request carries a one-use reply endpoint so the receiver can complete exactly one response.
  - Pros: Gives request/reply a queue-native shape without exposing shared state.
  - Cons: Late replies, timeout cancellation, and double replies must be handled.
  - Scenarios: Actor ask pattern, RPC fan-out, completion services.

- [ ] CloseableQueue
  - Core Concept: A queue has an explicit closed state that wakes blocked producers/consumers and rejects future sends.
  - Pros: Makes lifecycle and shutdown testable instead of relying on sentinel values.
  - Cons: Buffered item drain, blocked sender failure, and close idempotency need exact rules.
  - Scenarios: Pipeline shutdown, worker-pool stop, actor mailbox termination.

- [ ] DisconnectedChannel
  - Core Concept: Send and receive operations can detect that all opposite endpoints have disappeared.
  - Pros: Prevents indefinite blocking when producers or consumers are gone.
  - Cons: Endpoint reference counting and close races add complexity.
  - Scenarios: MPSC channels, request/reply cancellation, service teardown.

- [ ] TransferQueue
  - Core Concept: Producers can enqueue normally or wait until a consumer receives a specific item.
  - Pros: Combines buffering with explicit handoff semantics.
  - Cons: API and waiter matching are complex.
  - Scenarios: Executor handoff queues, actor ask/reply, service backpressure experiments.

- [~] LMAXDisruptor
  - Core Concept: A sequenced ring broadcasts slots to consumers that track their own progress rather than consuming items.
  - Pros: Avoids per-consumer allocation and supports dependency graphs among consumers; `Sequence` and `WaitStrategy` scaffolds are present.
  - Cons: Ring buffer, sequencer, sequence barrier, gating consumers, and working wait strategies are not implemented yet.
  - Scenarios: Matching engines, market-data fan-out, journaling/replication pipelines.

- [ ] MulticastRingBuffer
  - Core Concept: One producer publishes each slot to many consumers, and wraparound waits for the slowest gated consumer.
  - Pros: Teaches fan-out without copying messages per consumer.
  - Cons: A single stalled consumer can block reuse of the ring.
  - Scenarios: Market-data fan-out, event sourcing, Disruptor stepping stone.

- [ ] LaggingReceiverPolicy
  - Core Concept: A broadcast queue defines what happens when a receiver falls behind the retained buffer.
  - Pros: Forces explicit choice between blocking producers, dropping old messages, or failing slow receivers.
  - Cons: Each policy changes delivery guarantees and backpressure behavior.
  - Scenarios: Broadcast channels, pub/sub streams, telemetry fan-out.

- [ ] MichaelScottQueue
  - Core Concept: An unbounded linked MPMC queue uses CAS on `tail.next` and helping to advance lagging pointers.
  - Pros: Classic lock-free queue with no fixed capacity.
  - Cons: Safe node reclamation requires hazard pointers or EBR outside Go GC assumptions.
  - Scenarios: Unbounded linked queue study, lock-free helping, reclamation integration.

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

- [ ] DelayQueue
  - Core Concept: Items become available only after their deadline, usually backed by a min-heap plus a wakeup mechanism.
  - Pros: Provides a concrete scheduling queue for deadlines, retries, actor timers, and rate limiter waits.
  - Cons: Cancellation cleanup, clock jumps, and wakeup coalescing can produce subtle bugs.
  - Scenarios: DeadlineScheduler, retry queues, timeout tests with `clock.ManualClock`, delayed actor messages.

- [ ] LockFreePriorityQueue
  - Core Concept: A concurrent skip-list or heap-like structure supports priority-ordered dequeue.
  - Pros: Combines ordering with non-blocking progress.
  - Cons: Delete-min and reclamation are difficult.
  - Scenarios: Schedulers, timers, advanced lock-free data-structure study.
