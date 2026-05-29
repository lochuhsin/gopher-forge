# park Index

> Learning goal: understand how sleeping synchronization primitives atomically check a condition, park, and wake without losing notifications.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: checking a predicate and going to sleep must be atomic with respect to wakeup, otherwise wake-before-park is lost.
- All higher sleeping primitives eventually need this shape: atomic state word on the fast path, real parking on the slow path, loop on predicate after wake.
- Go parks goroutines, not OS threads, so runtime semaphores and channels are the practical educational substrate.
- Recommended build order from the merged TODO: runtime-sema Parker wrapper, use it to finish `syncx.MutexLock`, then Linux futex demo, then permit-style Parker.
- Cross-language gap: add AQS-style queued waiters before building many more
  sleeping primitives. Most timeout, fairness, and cancellation bugs live in
  waiter registration/removal, not in the fast-path atomic word.
- Dependencies: consumed by `syncx` locks, condvars, semaphores, latches, and futures.
- Career signal: advanced; useful for explaining futexes, permit-based parking, runtime parking, and atomic wait/notify.
- Scope rule: portable Go implementations should expose check-then-park and waiter-queue semantics; raw OS futex code is an optional Linux-specific lab.

## Reference Trail and Go Boundary

- Primary Go reference: runtime semaphore and notify-list source (`https://cs.opensource.google/go/go/+/refs/tags/go1.26.1:src/runtime/sema.go`).
- Cross-language reference: C++ `atomic_wait`/`notify` and semaphores (`https://eel.is/c++draft/thread`) name the portable shape, while Linux futex names the OS check-and-sleep primitive.
- Mental model: parking is never "sleep now." It is "publish waiter, re-check predicate, then sleep only if the wake cannot be lost."
- Go boundary: public Go has no futex or arbitrary atomic-wait API. Use channels/condvars for portable baselines and `go:linkname` runtime semaphores only for explicit runtime-internals learning.
- Cancellation boundary: timed waits need an ownership decision for wake-vs-timeout. Either the waker owns the permit or the canceller does; an ambiguous race creates leaks or double wakeups.
- Interview artifact: every parked primitive should include a lost-wakeup trace and the exact step that prevents it.

## Implementation Checklist

- [ ] Parker
  - Core Concept: A small wrapper around park/unpark operations gives higher-level primitives a clean blocking substrate.
  - Pros: Centralizes wait/wake semantics instead of scattering runtime calls.
  - Cons: Must define whether wakeups count, collapse to one permit, or can be spurious.
  - Scenarios: MutexLock, condvars, semaphores, futures, and latches.

- [ ] RuntimeSemaphoreParker
  - Core Concept: Wrap Go runtime semacquire/semrelease in a local API.
  - Pros: Reuses goroutine-aware parking that frees the P while blocked.
  - Cons: Requires `go:linkname` into unstable runtime internals.
  - Scenarios: Educational Go runtime study and replacing spin waits with parking.

- [ ] FutexWaitWake
  - Core Concept: Linux futex waits only if the memory word still equals an expected value, combining check and sleep atomically.
  - Pros: Prevents wake-before-sleep races and is the foundation of sleeping mutexes.
  - Cons: Linux-specific and requires syscall/build-tag handling.
  - Scenarios: Futex mutex, atomic wait/notify comparison, low-latency systems.

- [ ] AtomicWaitNotify
  - Core Concept: Wait on an atomic word while it remains equal to an expected value, then notify one or all waiters after changing it.
  - Pros: Portable concept shared by futexes and modern atomic wait/notify APIs.
  - Cons: Go lacks a public direct primitive, so implementation must use channels, condvars, or runtime internals.
  - Scenarios: Mutex slow paths, semaphores, parking queues, cross-language primitive comparison.

- [ ] WaitNode
  - Core Concept: Represent one blocked goroutine with a wake token, cancellation state, queue links, and optional deadline metadata.
  - Pros: Makes waiter lifecycle explicit and reusable across mutexes, semaphores, condvars, futures, and actor mailboxes.
  - Cons: Reuse and removal races are subtle; nodes must not be copied or woken twice.
  - Scenarios: FairSemaphore, ContextCond, TimeoutPark, cancellation-safe blocking queues.

- [ ] QueuedSynchronizer
  - Core Concept: Maintain an atomic state word plus a FIFO wait queue, with exclusive/shared acquire and release hooks like a Go-shaped AQS study.
  - Pros: Centralizes fairness, barging, wake-one, wake-all, and cancellation behavior instead of reimplementing them in every primitive.
  - Cons: Generic hooks can obscure each primitive's state machine if the abstraction is too broad.
  - Scenarios: Fair mutex, fair semaphore, reader-writer lock variants, phasers, countdown events.

- [ ] FutexMutex
  - Core Concept: A 3-state mutex uses `unlocked`, `locked no waiters`, and `locked with waiters` to avoid unnecessary wake syscalls.
  - Pros: Shows why production mutexes have more than a boolean state.
  - Cons: State transitions and waiter races are subtle.
  - Scenarios: Linux pthread mutex internals and `syncx.MutexLock` completion.

- [ ] PermitParker
  - Core Concept: `unpark` can pre-issue a single permit so a later `park` returns immediately.
  - Pros: Avoids lost wakeups in permit-based wait/wake designs.
  - Cons: Permit semantics differ from counting semaphores and must not accidentally accumulate.
  - Scenarios: Queued synchronizer comparison, actor mailbox wakeups, task schedulers.

- [ ] WaiterQueue
  - Core Concept: Blocked waiters are stored in FIFO/LIFO/priority queues and woken according to policy.
  - Pros: Makes fairness and starvation policy explicit.
  - Cons: Cancellation and timeout removal require careful list manipulation.
  - Scenarios: Condvar, semaphore, fair mutex, scheduler experiments.

- [ ] CancellationSafeWaiterRemoval
  - Core Concept: A timed-out or cancelled waiter removes itself or marks itself skipped without racing a concurrent wake into a lost permit.
  - Pros: Required for service-safe `Wait(ctx)`, `Acquire(ctx)`, and `TryLockUntil` APIs.
  - Cons: The wake-vs-cancel race must define exactly which side owns the permit or notification.
  - Scenarios: Context-aware condvars, weighted semaphores, blocking queues, futures.

- [ ] SelectWaitSet
  - Core Concept: A waiter registers interest in multiple wait sources and exactly one source wins.
  - Pros: Explains `select`-style multiplexing and cancellation races at the primitive level.
  - Cons: Atomic registration and deregistration across multiple queues is difficult.
  - Scenarios: Channel select internals, future-or-timeout waits, multi-resource admission.

- [ ] WakerRegistration
  - Core Concept: A task registers a wake handle, re-checks readiness, and sleeps only if the condition is still false.
  - Pros: Captures the core no-lost-wakeup rule behind async tasks, futures, and notify primitives.
  - Cons: Double registration, stale wakers, and cancellation cleanup are subtle.
  - Scenarios: Future polling, one-shot completion, notify-based queues, custom schedulers.

- [ ] TimeoutPark
  - Core Concept: Parking can return due to wakeup, timeout, cancellation, or spurious return.
  - Pros: Required for service-safe blocking APIs.
  - Cons: Removing a timed-out waiter races with a concurrent wake.
  - Scenarios: TryLock with timeout, rate limiter waits, bounded queues with context.

- [ ] SpuriousWakeupContract
  - Core Concept: A primitive defines whether wakeups can be spurious and requires callers to loop on the predicate.
  - Pros: Prevents condition-variable misuse and clarifies cross-language differences.
  - Cons: Adds API burden if every wait must report its wake reason.
  - Scenarios: Condvars, futexes, timeout waits, lost-wakeup labs.

- [ ] SchedulerHandoff
  - Core Concept: A wake can either make a waiter runnable or hand ownership directly to reduce barging.
  - Pros: Explains fairness modes such as Go mutex starvation handoff.
  - Cons: Direct handoff can reduce throughput under light contention.
  - Scenarios: Mutex fairness, semaphore handoff, latency-vs-throughput tuning.
