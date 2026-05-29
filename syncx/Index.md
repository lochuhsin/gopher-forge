# syncx Index

> Learning goal: build synchronization primitives from atomics, spinning, runtime parking, and monitor-style coordination before using higher-level Go abstractions.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Merged sources: the former root and family-specific roadmap files for locks, semaphores, latches, barriers, condition variables, channel helpers, futures, and STM.
- Package boundary: `syncx/` is for primitive families where variants share the same conceptual interface, such as lock, semaphore, latch, barrier, condition variable, future, and STM.
- Core invariant: every primitive must define its state machine, blocking policy, wakeup rule, fairness policy, and happens-before edge.
- Current truth from source: spin/ticket/MCS/RW wrapper/seqlock, semaphore variants, spin/channel latches, WaitGroup, Once/OnceCell, counting/sense barriers, and channel helpers have usable code; CLH has code but needs checklist-grade tests, and several runtime/cond/barrier/future/STM items are scaffolds.
- Recommended build order from the merged TODOs: finish `MutexLock`, complete condition variables, then event/future work, then advanced barriers, then STM as a later high-complexity topic.
- Dependencies: `syncx/` depends on `memory/` conceptually and on `queue/` for `LockfreeSemaphore`; higher packages depend heavily on `syncx/`.
- Career signal: this package is the foundation layer for the repo. Locks, semaphores, condvars, WaitGroup, and barriers are the vocabulary needed before data structures and patterns.
- Transfer rule: include named primitives only when their core state machine ports cleanly to Go, such as `CountDownLatch`, `CyclicBarrier`, `Phaser`, futures, and semaphores.

## Reference Trail and Go Boundary

- Classic sync line: Mellor-Crummey and Scott queue locks/barriers (`https://www.cs.rochester.edu/research/synchronization/pseudocode/ss.html`), Go `sync` docs (`https://pkg.go.dev/sync`), and runtime semaphore/notify-list internals (`https://cs.opensource.google/go/go/+/refs/tags/go1.26.1:src/runtime/sema.go`).
- Mental model: a synchronization primitive is a state machine plus an ownership rule plus a wake policy. If one of those is unnamed, the implementation is not ready.
- Substrate boundary: `sync.Mutex`, `sync.Cond`, `sync.Once`, channels, and `sync.Map` are baselines or semantic references unless the checklist item explicitly studies that abstraction.
- Sleeping boundary: anything that blocks should route through `park/` concepts: register waiter, re-check predicate, park, wake, loop, and handle cancellation.
- Fairness boundary: spin locks, ticket locks, MCS/CLH locks, semaphores, and condvars must say whether barging is allowed; safety without fairness is not the full primitive contract.
- Interview artifact: each primitive should have a one-page explanation of fast path, slow path, wake path, and the exact happens-before edge it exports to callers.

## Lock Family

- [x] SpinLock
  - Core Concept: A single atomic flag protects mutual exclusion; contenders repeatedly CAS until the flag flips from unlocked to locked.
  - Pros: Tiny implementation, zero allocation, very low overhead for extremely short critical sections.
  - Cons: No fairness, burns CPU while waiting, and all waiters fight over one cache line.
  - Scenarios: Teaching CAS loops, short non-blocking critical sections, baseline for spin-vs-park comparisons.

- [ ] BackoffSpinLock
  - Core Concept: Failed CAS attempts wait with exponential or randomized backoff before retrying.
  - Pros: Reduces cache-line thrashing under contention while preserving a small lock shape.
  - Cons: Tuning is workload-dependent and still wastes CPU during long waits.
  - Scenarios: Spinlock contention labs, HFT-style short critical sections, comparing busy-wait policies.

- [ ] TTASLock
  - Core Concept: Test-and-test-and-set spins on reads until the lock looks free, then attempts CAS.
  - Pros: Reduces write invalidations compared with repeatedly CAS-ing a locked word.
  - Cons: Still has a thundering herd when the owner releases the lock.
  - Scenarios: Cache-coherence education and stepping stone to queue locks.

- [x] TicketLock
  - Core Concept: Each caller takes a monotonically increasing ticket and spins until the serving counter reaches its ticket.
  - Pros: FIFO fairness, simple invariant, no starvation under normal progress.
  - Cons: Every waiter spins on the same serving counter, causing O(N) cache invalidation per unlock.
  - Scenarios: Fairness demonstrations, comparing centralized spinning with queue-based locks.

- [x] MCSLock
  - Core Concept: Each waiter enqueues a node and spins on its own node-local flag; unlock hands ownership to the next node.
  - Pros: FIFO fairness with O(1) coherence traffic per handoff; scales better than ticket locks under contention.
  - Cons: More complex enqueue/dequeue races and higher constant cost at low contention.
  - Scenarios: Many-core lock design, Linux qspinlock/AQS mental model, cache-line locality lessons.

- [~] MutexLock
  - Core Concept: A sleeping mutex should combine a fast atomic state word with runtime parking for waiters.
  - Pros: Avoids burning CPU under long waits while keeping the uncontended path cheap.
  - Cons: Current file only has the state/sema scaffold; Lock and Unlock are empty.
  - Scenarios: Futex-style 3-state mutex, Go runtime sema exploration, bridge from spinlocks to parking locks.

- [ ] TryTimedMutex
  - Core Concept: A sleeping mutex adds non-blocking and deadline-bounded acquisition paths.
  - Pros: Makes locks usable in services where indefinite blocking is unacceptable.
  - Cons: Timeout cleanup races with unlock and can complicate fairness.
  - Scenarios: Request-scoped critical sections, overload avoidance, cancellation-aware primitives.

- [ ] ScopedLockGuard
  - Core Concept: Lock acquisition returns a guard object or closure scope that owns the unlock responsibility.
  - Pros: Ties critical-section lifetime to a visible scope and prevents many forgotten-unlock bugs.
  - Cons: Guard escape or nested scopes can accidentally make critical sections too large.
  - Scenarios: Safe mutex wrappers, monitor methods, panic-safe cleanup, teaching access discipline.

- [ ] PoisonableMutex
  - Core Concept: If a goroutine fails while holding a lock, the protected state is marked suspect for later callers.
  - Pros: Surfaces invariant corruption instead of silently continuing with possibly broken state.
  - Cons: Poisoning policy can be noisy and recovery semantics must be explicit.
  - Scenarios: Invariant-heavy shared state, transaction-like monitors, failure-aware lock design.

- [x] RWMutexLock
  - Core Concept: A reader-writer lock lets multiple readers share the lock while writers require exclusivity.
  - Pros: Useful baseline via `sync.RWMutex`; demonstrates read-sharing API shape.
  - Cons: This is a wrapper, not a from-scratch implementation, so it does not teach the internal queueing policy yet.
  - Scenarios: Read-heavy workloads, comparison target for custom reader-preferring/writer-preferring/fair variants.

- [x] SeqLock
  - Core Concept: Writers increment a sequence counter around a write; readers optimistically copy data and validate that the sequence stayed even and unchanged.
  - Pros: Readers do not acquire a lock; excellent for read-mostly pure-value snapshots.
  - Cons: Readers may retry forever under heavy writes; unsafe for pointer-rich mutable structures without extra care.
  - Scenarios: Market data snapshots, `gettimeofday`-style state, optimistic concurrency teaching.

- [~] RCULock
  - Core Concept: Read-side critical sections should be nearly free while updates publish a new version and wait for old readers to drain.
  - Pros: Teaches the API surface of RCU-style reads.
  - Cons: Current methods are no-ops; there is no grace-period tracking or reclamation.
  - Scenarios: Placeholder for comparing lock-style reads with full `rcu/` package semantics.

- [~] CLHLock
  - Core Concept: A queue lock where each waiter spins on its predecessor's state rather than a shared lock word.
  - Pros: FIFO fairness and less shared cache-line bouncing than ticket locks; `CLHLock` code and explanatory notes exist in `lock.go`.
  - Cons: The API returns a per-acquisition node handle and does not yet have dedicated correctness tests or benchmarks, so it is not checklist-complete.
  - Scenarios: Compare CLH with MCS and queued synchronizer internals.

- [ ] OptimisticStampedRWLock
  - Core Concept: Readers can use optimistic stamps, validate later, or fall back to read/write locking.
  - Pros: Combines optimistic reads with explicit lock modes.
  - Cons: API is more complex and misuse can silently invalidate assumptions.
  - Scenarios: Read-mostly structures, stamped optimistic reads, seqlock-to-RWMutex bridge.

- [ ] BigReaderLock
  - Core Concept: Readers use sharded/per-CPU state while writers coordinate across all shards.
  - Pros: Reduces reader-side contention dramatically.
  - Cons: Writer path is expensive and implementation depends on stable sharding.
  - Scenarios: Read-dominant global state, configuration snapshots, kernel-style big-reader designs.

- [ ] RWMutexVariants
  - Core Concept: Implement reader-preferring, writer-preferring, and fair reader-writer locks to expose starvation tradeoffs.
  - Pros: Makes fairness policy explicit and measurable.
  - Cons: More state-machine complexity than a simple mutex.
  - Scenarios: Interview prep, database latch policy, reader/writer starvation demonstrations.

## Semaphore Family

- [x] ChannelSemaphore
  - Core Concept: A buffered channel bounds the number of concurrent acquisitions.
  - Pros: Idiomatic Go, easy cancellation with `select`, minimal code.
  - Cons: Pays channel runtime overhead and hides the lower-level wait queue.
  - Scenarios: Practical Go throttling and baseline for custom semaphores.

- [x] MutexSemaphore
  - Core Concept: A mutex protects the permit count and blocked waiters park on pre-locked gate mutexes.
  - Pros: Clear FIFO handoff model and no busy-waiting.
  - Cons: Allocates per blocked waiter and retained waiter slice capacity can grow.
  - Scenarios: Teaching direct handoff and wait queue ownership.

- [x] CondSemaphore
  - Core Concept: A monitor-style mutex plus condition variable guards the permit predicate.
  - Pros: Canonical condition-variable pattern with a predicate loop.
  - Cons: No strict FIFO handoff; a signaled waiter can be barged by a new acquirer.
  - Scenarios: Condvar education, bounded-resource monitors.

- [x] LockfreeSemaphore
  - Core Concept: A signed atomic permit counter encodes both available permits and waiter deficit; waiters spin on gate flags.
  - Pros: Lock-free fast path and useful composition of atomics plus a queue.
  - Cons: Slow path burns CPU and the enqueue/wake race is subtle.
  - Scenarios: Learning fast-path/slow-path state machines before replacing spin with park.

- [x] RuntimeSemaphore
  - Core Concept: A fast atomic counter falls back to Go runtime semaphores for true goroutine parking.
  - Pros: Cheap uncontended path and blocked goroutines consume no CPU.
  - Cons: Uses `go:linkname` internal runtime APIs that are not stable.
  - Scenarios: Go runtime internals, sleeping primitives, mutex/semaphore implementation study.

- [ ] WeightedSemaphore
  - Core Concept: Acquire and release variable permit weights instead of single units.
  - Pros: Models heterogeneous resource costs.
  - Cons: Fairness and head-of-line blocking become harder.
  - Scenarios: Connection pools, memory budgets, batch admission control.

- [ ] TimeoutSemaphore
  - Core Concept: Permit acquisition can fail after a deadline or context cancellation.
  - Pros: Makes blocking primitives usable in services.
  - Cons: Requires cancellation-safe waiter removal.
  - Scenarios: RPC concurrency limits, overload protection, resilience patterns.

- [ ] FairSemaphore
  - Core Concept: Waiters acquire permits according to an explicit FIFO queue rather than barging.
  - Pros: Prevents starvation and makes tests deterministic.
  - Cons: Can reduce throughput and creates head-of-line blocking for weighted acquires.
  - Scenarios: Service admission control, fairness labs, weighted semaphore comparison.

- [ ] CloseableSemaphore
  - Core Concept: Closing the semaphore wakes blocked acquirers and makes future acquires fail deterministically.
  - Pros: Gives shutdown a first-class state instead of relying on external cancellation only.
  - Cons: Release-after-close and permit leak semantics must be specified precisely.
  - Scenarios: Worker pool shutdown, bounded service admission, async runtime primitive study.

## Latch, Event, and Once Family

- [x] SpinLatch
  - Core Concept: A one-shot countdown latch where waiters spin until an atomic count reaches zero.
  - Pros: Minimal and exposes countdown semantics clearly.
  - Cons: Busy-waits and is only appropriate for very short waits.
  - Scenarios: Teaching one-shot release and atomic counter visibility.

- [x] ChanLatch
  - Core Concept: Closing a channel broadcasts completion to all waiters.
  - Pros: Simple, idiomatic, and handles many waiters with one close.
  - Cons: One-shot only and does not expose lower-level wake mechanics.
  - Scenarios: Go-style countdown latch, completion signals.

- [x] CountDownLatch
  - Core Concept: A one-shot count reaches zero through `Done` or `CountDown` calls, releasing all waiters and letting future waits pass immediately.
  - Pros: Names the classic countdown latch concept and maps cleanly to existing spin/channel latch work.
  - Cons: Cannot be reset; reusable phases belong to barriers or phasers.
  - Scenarios: Start gates, completion gates, waiting for N workers or N events.

- [~] SemaLatch
  - Core Concept: Build broadcast on top of one-at-a-time runtime semaphore wakeups.
  - Pros: Exposes the lost-wakeup race directly.
  - Cons: Current type and methods are TODO scaffolds.
  - Scenarios: Understanding why broadcast is harder than wake-one.

- [~] NotifyListLatch
  - Core Concept: Use runtime notify-list tickets to register waiters before parking.
  - Pros: Models the mechanism behind `sync.Cond.Broadcast`.
  - Cons: Current implementation is scaffold only and depends on runtime internals.
  - Scenarios: Lost-wakeup avoidance and Go runtime wait-list study.

- [x] WaitGroup
  - Core Concept: Pack counter and waiter count into one atomic word, then release waiters when the counter reaches zero.
  - Pros: Zero-value usable, reusable with rules, and demonstrates atomic state packing.
  - Cons: Misuse detection is runtime-only and broadcast is O(waiters).
  - Scenarios: Rebuilding `sync.WaitGroup`, packed state machines.

- [ ] ManualResetEvent
  - Core Concept: Once signaled, all current and future waiters pass until reset.
  - Pros: Broadcast state is explicit and persistent.
  - Cons: Reset races are subtle when waiters are entering concurrently.
  - Scenarios: Persistent event gates, broadcast signals, phase gates.

- [ ] AutoResetEvent
  - Core Concept: A signal releases one waiter and then automatically returns to non-signaled.
  - Pros: Good for handoff between producer and one consumer.
  - Cons: Fairness and signal-before-wait semantics must be defined precisely.
  - Scenarios: Thread handoff, wake-one event semantics.

- [ ] Notify
  - Core Concept: A small notification primitive for one-shot or repeated wakeups.
  - Pros: Useful building block for futures and task wakeups.
  - Cons: Easy to lose signals if registration and wake are not atomic.
  - Scenarios: Async runtime wakers, lightweight event notification.

- [x] Once
  - Core Concept: Run a function exactly once and publish its side effects to all later callers.
  - Pros: Teaches double-checking and publication safety; tests cover exactly-once execution, blocking followers, happens-before publication, panic semantics, and stress.
  - Cons: Follows `sync.Once`-style panic behavior by marking the operation done; it does not provide retry semantics.
  - Scenarios: Lazy initialization and `sync.Once` internals.

- [x] OnceValue
  - Core Concept: Run a supplier once, store its value or error, and return the same result to all callers.
  - Pros: Practical bridge from `Once` to `OnceCell` and future-like value publication; implemented as `OnceCell[T]` and `OnceCells[T, K]` with value, pair, concurrent, and error round-trip tests.
  - Cons: The public names are `OnceCell` and `OnceCells`, not `OnceValue`; panic behavior inherits `Once`.
  - Scenarios: Lazy config loading, singleton resources, memoized expensive computation.

## Barrier Family

- [x] CountingBarrier
  - Core Concept: A one-shot centralized counter releases all waiters when the last participant arrives.
  - Pros: Easy invariant and tiny state.
  - Cons: Not reusable and all waiters spin on one counter.
  - Scenarios: First barrier implementation and centralized contention demo.

- [x] SenseReversingBarrier
  - Core Concept: A reusable barrier uses an epoch/sense variable to distinguish rounds.
  - Pros: Reusable and still compact.
  - Cons: Centralized spinning still causes cache-line traffic.
  - Scenarios: Iterative parallel algorithms and phase synchronization.

- [~] CyclicBarrier
  - Core Concept: A reusable barrier releases a fixed number of parties at the end of each generation.
  - Pros: Classic phase primitive and a clear user-facing name for sense/counting barrier behavior.
  - Cons: Sense-reversing behavior exists, but a full CyclicBarrier-style API with broken-barrier, timeout, and barrier-action semantics is not complete.
  - Scenarios: Iterative solvers, worker phase loops, reusable barrier APIs.

- [ ] BarrierAction
  - Core Concept: The last arriving participant runs a completion callback before the next phase is released.
  - Pros: Encodes per-phase aggregation or state rollover at the synchronization point.
  - Cons: Callback panic/latency can break or delay every participant.
  - Scenarios: Parallel reductions, simulation ticks, cyclic barrier API study.

- [ ] Phaser
  - Core Concept: A reusable phase barrier lets parties register and deregister dynamically while tracking phase numbers.
  - Pros: More flexible than fixed-party barriers and latches.
  - Cons: State machine is larger and termination rules are easy to get wrong.
  - Scenarios: Dynamic task sets, recursive parallel phases, variable-party phase coordination.

- [~] CombiningTreeBarrier
  - Core Concept: Arrival combines up a tree and release propagates back down.
  - Pros: Reduces per-node contention from N to a small fan-in.
  - Cons: Current methods are empty scaffolds.
  - Scenarios: Many-core barrier scaling and collective communication intuition.

- [~] StaticTreeBarrier
  - Core Concept: A fixed tree topology maps participants to leaves and internal release nodes.
  - Pros: Predictable memory layout and bounded contention.
  - Cons: Current implementation is scaffold only.
  - Scenarios: Static worker pools and tree-based phase coordination.

- [~] TournamentBarrier
  - Core Concept: Participants advance through pairwise rounds, with winners carrying arrival upward and release downward.
  - Pros: Logarithmic rounds and clear pairwise roles.
  - Cons: Current implementation is scaffold only and role assignment is tricky.
  - Scenarios: Barrier algorithms from MCS-style literature.

- [~] DisseminationBarrier
  - Core Concept: In round k, each participant signals a partner at distance 2^k, completing in O(log N) rounds.
  - Pros: Symmetric, no root bottleneck, useful for distributed collectives.
  - Cons: Current implementation is scaffold only and requires per-round flags.
  - Scenarios: MPI/NCCL-style collective learning.

- [~] ButterflyBarrier
  - Core Concept: Participants synchronize along butterfly communication edges across log rounds.
  - Pros: Maps well to network and collective communication patterns.
  - Cons: Current implementation is scaffold only.
  - Scenarios: AI infra collectives and topology-aware synchronization.

## Condition Variable Family

- [~] MesaQueueCond
  - Core Concept: Waiters release a lock, park in a queue, and re-check the predicate after wakeup under Mesa semantics.
  - Pros: Teaches why condition variables require a predicate loop.
  - Cons: Current methods are empty.
  - Scenarios: Bounded blocking queues and monitor patterns.

- [~] MesaNotifyListCond
  - Core Concept: Runtime notify-list tickets avoid lost wakeups between registration and parking.
  - Pros: Mirrors how Go's `sync.Cond` works internally.
  - Cons: Current methods are empty and depend on runtime-private APIs.
  - Scenarios: Condvar internals, Signal vs Broadcast, lost-wakeup analysis.

- [ ] ContextCond
  - Core Concept: A condition variable wait can return because the predicate was signaled, the context was cancelled, or a deadline expired.
  - Pros: Makes condition variables usable in service code and exercises cancellation-safe waiter removal.
  - Cons: Removing a waiter races with Signal/Broadcast, so the wake ownership rule must be exact.
  - Scenarios: Blocking queues, admission control waits, shutdown-aware monitors.

## Channel Helper Family

- [x] UnorderedChannel
  - Core Concept: Drain an input channel while buffering overflow and later emit buffered values without preserving strict streaming order.
  - Pros: Simple non-blocking producer-facing helper.
  - Cons: Buffer growth can hide backpressure and ordering semantics are weaker.
  - Scenarios: Channel buffering experiments and backpressure tradeoffs.

- [x] RingQueue
  - Core Concept: A growable ring buffer stores pending channel values using head/tail indexes and a power-of-two mask.
  - Pros: Clear FIFO mechanics and amortized growth.
  - Cons: Not concurrent by itself.
  - Scenarios: Internal buffer for ordered channel helpers.

- [x] OrderedChannel
  - Core Concept: Preserve FIFO output while accepting input and output concurrently through an internal queue.
  - Pros: Demonstrates select-driven buffering without phantom values.
  - Cons: Single goroutine mediator can bottleneck.
  - Scenarios: Go channel scheduling and ordered fan-in/fan-out teaching.

- [ ] PipelineWithBackpressure
  - Core Concept: Bound each stage and propagate cancellation or pressure upstream.
  - Pros: Turns channels into service-safe pipelines.
  - Cons: Cancellation and draining rules are easy to get wrong.
  - Scenarios: Stream processing, market-data fan-out, inference batching.

- [ ] Exchanger
  - Core Concept: Two participants rendezvous and atomically swap values.
  - Pros: Teaches pairwise rendezvous without a persistent queue.
  - Cons: Timeout/cancellation and odd participant counts require careful rules.
  - Scenarios: Double-buffer swaps, genetic algorithms, pairwise handoff exercises.

- [ ] OneShotChannel
  - Core Concept: A single sender publishes exactly one value to one or more waiters, then the channel is permanently completed.
  - Pros: Minimal async result primitive and a direct bridge between latch and future.
  - Cons: Cancellation, late receivers, and double-send behavior must be specified.
  - Scenarios: Task completion, request/reply, initialization handoff, promise internals.

- [ ] WatchChannel
  - Core Concept: Keep only the latest value plus a version counter; receivers wait until they observe a newer version.
  - Pros: Efficient for state observation because slow receivers skip stale intermediate updates.
  - Cons: Not a queue; consumers that need every event must use broadcast or FIFO.
  - Scenarios: Config updates, health state, leader epoch, latest-price snapshots.

- [ ] BroadcastChannel
  - Core Concept: Each sent value is delivered to every active receiver, with per-receiver cursors over a shared buffer.
  - Pros: Fan-out without manually copying a message into N queues.
  - Cons: Slow receivers need lag/drop policy and buffer retention can grow.
  - Scenarios: Pub/sub, actor event streams, market-data fan-out, invalidation messages.

- [ ] ChannelCloseSemantics
  - Core Concept: Closing a channel or queue wakes waiters and makes send/receive outcomes explicit.
  - Pros: Prevents goroutine leaks and gives shutdown/error propagation a standard contract.
  - Cons: Double close, send-after-close, and buffered-drain behavior must be defined.
  - Scenarios: Pipeline shutdown, actor mailbox stop, producer disappearance, test cleanup.

## Keyed and Deduplication Family

- [ ] KeyedMutex
  - Core Concept: Hash keys to independent mutexes or dynamically create per-key locks so unrelated keys proceed concurrently.
  - Pros: Practical bridge from primitive locks to per-account, per-symbol, or per-resource isolation.
  - Cons: Dynamic lock cleanup needs refcounts or epochs; striped versions can serialize unrelated hot keys.
  - Scenarios: Per-user cache fills, account risk gates, symbol shards, idempotency keys.

- [ ] KeyedSemaphore
  - Core Concept: Limit concurrent work independently per key while preserving a global API for acquire and release.
  - Pros: Models resource partitioning without forcing callers to manage many semaphore instances.
  - Cons: Permit leaks and key cleanup are harder than for a single semaphore.
  - Scenarios: Per-tenant throttling, per-market request caps, partitioned worker pools.

- [ ] SingleFlight
  - Core Concept: Concurrent callers for the same key share one in-flight computation and all receive the same result.
  - Pros: Prevents cache stampedes and duplicate RPCs while teaching result publication and waiter broadcast.
  - Cons: Cancellation policy is subtle: one caller timing out must not necessarily cancel the shared producer.
  - Scenarios: Config reload, cache fill, price snapshot refresh, RPC fan-in.

## Future and STM Family

- [~] FuturePromise
  - Core Concept: A write-once value wakes all waiters and lets later readers observe the published result.
  - Pros: Connects latch, channel close, and async result delivery.
  - Cons: `future.go` currently only declares the package.
  - Scenarios: Async result handoff, RPC replies, future/promise state machines.

- [ ] CompletableFuture
  - Core Concept: Compose futures with callbacks, map, bind, when-all, and when-any.
  - Pros: Teaches async dependency graphs.
  - Cons: Thread-safe callback registration is subtle.
  - Scenarios: Task orchestration and functional concurrency.

- [ ] CancellableFuture
  - Core Concept: Pending futures can be cancelled and propagate cancellation downstream.
  - Pros: Aligns async result delivery with structured cancellation.
  - Cons: Requires cooperative producer behavior and state races are tricky.
  - Scenarios: Request cancellation, timeouts, async runtime study.

- [~] STM
  - Core Concept: Software transactions read/write versioned memory and commit atomically if conflicts are absent.
  - Pros: Teaches optimistic concurrency at a higher abstraction than locks.
  - Cons: `stm.go` currently only declares the package.
  - Scenarios: Block-STM, OCC, MVCC, and transaction conflict detection.
