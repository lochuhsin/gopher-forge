# scope Index

> Learning goal: bind goroutine lifetime, cancellation, deadlines, and error propagation into structured concurrency instead of launching unowned work.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: the code that starts work owns its lifetime. A scope must not exit while its children are still running.
- Cancellation is cooperative in Go; this package should make checkpoints and propagation explicit rather than promising preemption.
- Structured concurrency differs from `context.Context`: context carries a cancellation signal, while scope also owns spawned goroutines.
- Recommended build order from the merged TODO: CancellationToken, TokenTree, Nursery, ErrGroup, OnCancel callbacks, DeadlineScheduler, then cancellation benchmarks.
- Dependencies: uses `syncx.WaitGroup`, future/promise work, and cancellation-aware wait queues; consumed by `parallel/`, `actor/`, and `_lab/pattern`.
- Career signal: high for senior Go because context, errgroup, deadlines, and graceful shutdown are practical interview and production topics.
- Scope rule: structured concurrency items are included when they enforce ownership and bounded lifetime, not just because a language has an async keyword.

## Implementation Checklist

- [ ] CancellationToken
  - Core Concept: A sticky cancellation signal can be cloned, observed, and triggered by an owner.
  - Pros: Cleaner fan-out/fan-in cancellation than passing raw channels everywhere.
  - Cons: Does not stop work preemptively; goroutines must cooperate.
  - Scenarios: Request cancellation, worker shutdown, async task trees.

- [ ] TokenTree
  - Core Concept: Parent cancellation propagates to children through an explicit tree.
  - Pros: Models ownership and cascading shutdown.
  - Cons: Child registration/removal races require synchronization.
  - Scenarios: Service subsystems, actor supervision, nested request work.

- [ ] Nursery
  - Core Concept: A scope owns child goroutines; `Wait` cannot return until all children exit.
  - Pros: Prevents goroutine leaks and makes lifetime explicit.
  - Cons: Requires every spawned task to go through the scope.
  - Scenarios: Trio/Kotlin-style structured concurrency in Go.

- [ ] ErrGroupClone
  - Core Concept: Run tasks in parallel, return the first error, and cancel siblings.
  - Pros: High-value practical pattern for server code.
  - Cons: First-error policy may hide later errors unless collected separately.
  - Scenarios: Parallel RPC fan-out, batch jobs, service startup.

- [ ] ResultGroup
  - Core Concept: Run tasks in a scope and collect typed results, errors, and cancellation state.
  - Pros: Extends errgroup from "wait for error" to useful parallel composition.
  - Cons: Result ordering, partial failure, and memory retention must be specified.
  - Scenarios: Parallel queries, fan-out aggregation, async task composition.

- [ ] DeadlineScheduler
  - Core Concept: Manage many deadlines efficiently with a heap or timing wheel.
  - Pros: Avoids one goroutine/timer per task at high scale.
  - Cons: Clock jumps, cancellation cleanup, and timer granularity are tricky.
  - Scenarios: LLM request timeouts, rate limiter waits, task cancellation.

- [ ] CancellationCallback
  - Core Concept: Register callbacks that fire when a token is cancelled.
  - Pros: Avoids polling and integrates with wait queues.
  - Cons: Callback ordering, panic behavior, and deregistration must be defined.
  - Scenarios: Unparking blocked waiters and cancelling futures.

- [ ] TaskGroup
  - Core Concept: A group exposes spawn, cancel, wait, and result aggregation as one unit.
  - Pros: Practical structured-concurrency API.
  - Cons: API can become too broad if it mixes scheduling and cancellation policy.
  - Scenarios: Service orchestration and parallel workloads.

- [ ] BoundedTaskGroup
  - Core Concept: A task group limits the number of concurrently running child tasks.
  - Pros: Combines structured concurrency with admission control.
  - Cons: Submitters can deadlock if tasks wait while holding scarce permits.
  - Scenarios: Parallel RPC caps, worker-pool replacement, bounded fan-out.

- [ ] JoinHandle
  - Core Concept: A spawned child returns a handle that can join, observe result, or request cancellation while the scope still owns lifetime.
  - Pros: Names per-task lifecycle without allowing orphaned goroutines.
  - Cons: Handles must not let callers outlive or detach work accidentally.
  - Scenarios: Async result composition, actor asks, fork/join tasks.

- [ ] CooperativeCancellationBenchmark
  - Core Concept: Compare polling frequency, callback wakeups, and blocked-operation cancellation.
  - Pros: Turns cancellation policy into measurable latency/overhead tradeoffs.
  - Cons: Benchmark results depend heavily on workload.
  - Scenarios: Runtime tuning and cancellation checkpoint design.
