# park Index

> Learning goal: understand how sleeping synchronization primitives atomically check a condition, park, and wake without losing notifications.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: checking a predicate and going to sleep must be atomic with respect to wakeup, otherwise wake-before-park is lost.
- All higher sleeping primitives eventually need this shape: atomic state word on the fast path, real parking on the slow path, loop on predicate after wake.
- Go parks goroutines, not OS threads, so runtime semaphores and channels are the practical educational substrate.
- Recommended build order from the merged TODO: runtime-sema Parker wrapper, use it to finish `syncx.MutexLock`, then Linux futex demo, then permit-style Parker.
- Dependencies: consumed by `syncx` locks, condvars, semaphores, latches, and futures.
- Career signal: advanced; useful for explaining futexes, Java LockSupport/AQS, Go runtime parking, and C++20 atomic wait/notify.
- Scope rule: portable Go implementations should expose check-then-park and waiter-queue semantics; raw OS futex code is an optional Linux-specific lab.

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
  - Scenarios: Futex mutex, C++20 atomic wait/notify comparison, low-latency systems.

- [ ] AtomicWaitNotify
  - Core Concept: Wait on an atomic word while it remains equal to an expected value, then notify one or all waiters after changing it.
  - Pros: Portable concept shared by futexes and C++20 atomic wait/notify.
  - Cons: Go lacks a public direct primitive, so implementation must use channels, condvars, or runtime internals.
  - Scenarios: Mutex slow paths, semaphores, parking queues, cross-language primitive comparison.

- [ ] FutexMutex
  - Core Concept: A 3-state mutex uses `unlocked`, `locked no waiters`, and `locked with waiters` to avoid unnecessary wake syscalls.
  - Pros: Shows why production mutexes have more than a boolean state.
  - Cons: State transitions and waiter races are subtle.
  - Scenarios: Linux pthread mutex internals and `syncx.MutexLock` completion.

- [ ] PermitParker
  - Core Concept: `unpark` can pre-issue a single permit so a later `park` returns immediately.
  - Pros: Avoids lost wakeups in Java LockSupport/AQS style designs.
  - Cons: Permit semantics differ from counting semaphores and must not accidentally accumulate.
  - Scenarios: Java AQS comparison, actor mailbox wakeups, task schedulers.

- [ ] WaiterQueue
  - Core Concept: Blocked waiters are stored in FIFO/LIFO/priority queues and woken according to policy.
  - Pros: Makes fairness and starvation policy explicit.
  - Cons: Cancellation and timeout removal require careful list manipulation.
  - Scenarios: Condvar, semaphore, fair mutex, scheduler experiments.

- [ ] SelectWaitSet
  - Core Concept: A waiter registers interest in multiple wait sources and exactly one source wins.
  - Pros: Explains `select`-style multiplexing and cancellation races at the primitive level.
  - Cons: Atomic registration and deregistration across multiple queues is difficult.
  - Scenarios: Channel select internals, future-or-timeout waits, multi-resource admission.

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
