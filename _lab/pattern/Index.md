# pattern Index

> Learning goal: compose primitives into recognizable concurrency architectures such as Reactor, Active Object, Disruptor, and pipeline backpressure.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Lab Notes

- This lab is for composition, not new primitive invention. Implement patterns by importing `syncx/`, `queue/`, `deque/`, `scope/`, and `actor/`.
- Each pattern should include a small runnable example, the primitive list it uses, and one benchmark or stress case when meaningful.
- Recommended build order from the merged TODO: Monitor Object, Active Object, Reactor, Half-Sync/Half-Async, Leader-Followers, Disruptor, Pipeline with Backpressure, Acceptor-Connector.
- Dependencies: uses most core packages; `parallel/pipeline` can reuse these patterns.
- Career signal: strongest when you can name the architecture, not just the primitives: Reactor, Active Object, Disruptor, and Half-Sync/Half-Async are standard senior vocabulary.
- Scope rule: this lab should compose portable concepts; OS-specific IO APIs can be described, but examples should run with Go channels, queues, contexts, and fake clocks.

## Implementation Checklist

- [ ] MonitorObject
  - Core Concept: Encapsulate state, mutex, and condition variables inside an object whose methods enforce synchronization.
  - Pros: Clear invariant boundary and keeps lock discipline inside the object.
  - Cons: Can hide lock ordering and block callers inside methods.
  - Scenarios: Bounded buffer, guarded state, condvar teaching.

- [ ] ActiveObject
  - Core Concept: Convert method calls into queued messages executed by a private worker.
  - Pros: Decouples caller from executor and isolates state.
  - Cons: Return values need futures and queue overload must be handled.
  - Scenarios: Async services, actor bridge, request serialization.

- [ ] ShareByCommunicating
  - Core Concept: Move ownership of data through messages instead of sharing mutable data behind locks.
  - Pros: Makes race freedom a design property instead of a testing afterthought.
  - Cons: Large payloads may require copying, immutable sharing, or reference ownership rules.
  - Scenarios: Pipelines, actor messages, work handoff, staged services.

- [ ] CSPPipelinePatterns
  - Core Concept: Compose channels into stages with `or-done`, `tee`, `bridge`, fan-in, and fan-out helpers.
  - Pros: Teaches idiomatic Go communication and cancellation as a reusable vocabulary.
  - Cons: Naive versions leak goroutines when downstream exits early.
  - Scenarios: Streaming transforms, Go pipeline practice, cancellation drills.

- [ ] FanInFanOut
  - Core Concept: Fan-out distributes work across workers; fan-in merges worker outputs into one stream.
  - Pros: Simple and directly useful for parallel pipelines.
  - Cons: Ordering, cancellation, and slow-worker behavior must be specified.
  - Scenarios: ETL, inference batches, market-data enrichment.

- [ ] Reactor
  - Core Concept: One event loop demultiplexes readiness events and dispatches handlers.
  - Pros: Efficient non-blocking IO architecture and easy handler ownership.
  - Cons: Long handlers block the loop.
  - Scenarios: Netty/Node-style servers, exchange gateways, event demux.

- [ ] Proactor
  - Core Concept: The OS/runtime performs async operations and notifies completion.
  - Pros: Handler runs after work completes rather than on readiness.
  - Cons: Harder to emulate portably in Go without OS-specific APIs.
  - Scenarios: IOCP/io_uring study and async file/network completion.

- [ ] HalfSyncHalfAsync
  - Core Concept: Async IO threads receive events and hand blocking or CPU work to synchronous worker pools through queues.
  - Pros: Common production architecture and isolates IO from business work.
  - Cons: Queue sizing and backpressure are critical.
  - Scenarios: Web servers, inference serving, market-data processors.

- [ ] LeaderFollowers
  - Core Concept: One leader waits for events; when an event arrives, a follower becomes the new leader while the old leader handles work.
  - Pros: Reduces context switches and handoff latency.
  - Cons: Role transitions and fairness are complex.
  - Scenarios: Low-latency servers and POSA2 pattern study.

- [ ] Disruptor
  - Core Concept: A single-writer sequenced ring plus consumer barriers builds a lock-free pipeline.
  - Pros: High throughput, low allocation, and explicit dependency graph.
  - Cons: More specialized than a normal queue and slow consumers constrain wraparound.
  - Scenarios: LMAX/HFT matching pipeline, market-data fan-out.

- [ ] PipelineBackpressure
  - Core Concept: Bounded stages propagate pressure upstream when downstream slows.
  - Pros: Prevents unbounded memory growth and makes overload visible.
  - Cons: Shutdown, cancellation, and draining semantics are subtle.
  - Scenarios: Reactive streams, data processing, exchange pipelines.

- [ ] BackpressurePolicyMatrix
  - Core Concept: Implement and compare block, drop-oldest, drop-newest, shed, buffer, and degrade policies.
  - Pros: Makes overload behavior an explicit design choice.
  - Cons: No single policy is correct for all workloads.
  - Scenarios: Actor mailboxes, queues, API admission, streaming systems.

- [ ] SelectFairnessLab
  - Core Concept: Stress `select` across multiple ready operations and measure starvation or priority behavior.
  - Pros: Builds intuition for Go channel multiplexing under load.
  - Cons: Scheduler randomness makes results probabilistic unless instrumented.
  - Scenarios: Control/data channel priority, cancellation selects, starvation tests.

- [ ] ThreadPerCore
  - Core Concept: Partition state by core and communicate through queues instead of shared locks.
  - Pros: Strong locality and predictable latency.
  - Cons: Requires careful sharding and cross-core messaging.
  - Scenarios: HFT, Seastar/ScyllaDB, symbol-sharded matching.

- [ ] CrashOnlyComponent
  - Core Concept: Treat component failure as a normal signal: stop, discard local state, and restart from a supervisor-owned boundary.
  - Pros: Keeps recovery logic explicit and avoids half-repaired state.
  - Cons: Requires idempotent inputs or durable checkpoints to avoid duplicated side effects.
  - Scenarios: Actor services, worker pools, pipeline stages, fault-injection labs.

- [ ] SupervisedWorkerPool
  - Core Concept: A supervisor owns a worker pool, restarts failed workers, and applies restart intensity limits.
  - Pros: Combines practical pools with explicit fault containment.
  - Cons: Restarting workers can duplicate in-flight work unless tasks are idempotent or acknowledged.
  - Scenarios: Background jobs, ingestion workers, retryable task execution.

- [ ] WatchBroadcastStatePattern
  - Core Concept: Use a watch-style latest-value stream for state and a broadcast-style stream for every event.
  - Pros: Teaches the difference between state observation and event delivery.
  - Cons: Mixing the two incorrectly causes missed events or unnecessary backlog.
  - Scenarios: Config propagation, leader changes, actor event subscriptions.

- [ ] StagedEventDrivenArchitecture
  - Core Concept: Divide a service into stages connected by queues, each with its own resources and admission policy.
  - Pros: Makes resource bottlenecks explicit.
  - Cons: Adds queueing latency and operational complexity.
  - Scenarios: Web services, GPU inference schedulers, pipeline experiments.

- [ ] WorkStealingPool
  - Core Concept: Workers process local deques and steal from others when idle.
  - Pros: Balances irregular recursive work while preserving locality.
  - Cons: Depends on a correct Chase-Lev deque and scheduler policy.
  - Scenarios: Fork/join, work-stealing execution, parallel algorithms.

- [ ] BulkheadAndBreakerPattern
  - Core Concept: Combine semaphores, timeouts, circuit breakers, and load shedding to isolate failure domains.
  - Pros: Connects low-level primitives to production resilience.
  - Cons: Policy tuning and observability are as important as code correctness.
  - Scenarios: Backend RPC clients, inference dependency isolation, exchange risk controls.
