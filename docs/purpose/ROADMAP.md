# gopher-forge ROADMAP — Index-Driven Rust/Go High-Performance Plan

> Target: high-salary, high-moat Rust/Go infrastructure roles: HFT, market
> making, exchange core, crypto infrastructure, FAANG/top-tier infra, AI/agent
> runtime, fintech hot paths, streaming, database/storage engines, high-throughput
> and ultra-low-latency systems. Non-goal: CRUD backend.

## 0. Source of Truth

This roadmap is driven by the package `Index.md` files. The old `TODO.md` files
are no longer treated as authoritative.

Use these documents as the current system map:

- Foundation: `memory/Index.md`, `syncx/Index.md`, `park/Index.md`
- Data structures: `queue/Index.md`, `stack/Index.md`, `map/Index.md`, `deque/Index.md`
- Reclamation/read-mostly: `hazard/Index.md`, `reclamation/Index.md`, `rcu/Index.md`
- Runtime/patterns: `scope/Index.md`, `actor/Index.md`, `parallel/Index.md`, `_lab/pattern/Index.md`
- Verification: `_lab/verify/Index.md`
- Niche/deep tracks: `arena/Index.md`, `clock/Index.md`, `crdt/Index.md`, `_lab/excercise/Index.md`

External market calibration:

- `docs/research/high_performance_findings_2026-05-27.md`
- `docs/research/high_performance_source_catalog_2026-05-27.tsv`

## 1. Strategy

The highest-return path is not to implement every primitive. The highest-return
path is to build a chain of artifacts that a performance interviewer can inspect:

```text
memory model -> SPSC ring -> p99 benchmark report -> Disruptor / ThreadPerCore
-> exchange or streaming lab -> Rust atomic proof -> reclamation/deep systems
```

This repo should tell one story:

> I can reason about memory ordering, cache behavior, progress guarantees,
> backpressure, p99 latency, and correctness, then compose those primitives into
> exchange, database, streaming, and AI-runtime systems.

Do not frame the project as a production replacement for `sync`, `crossbeam`,
`parking_lot`, `tokio`, or `folly`. Frame it as a high-performance systems forge.

## 2. Scoring Model

Rank Index items by these axes:

| Axis | Meaning |
|---|---|
| `M` | Moat: separates high-performance engineers from ordinary backend engineers |
| `H` | Hot-path relevance: appears in latency, throughput, p99, or cache-sensitive paths |
| `P` | Portfolio proof: can produce code, tests, benchmarks, reports, and demos |
| `R` | Rust/Go transfer: useful in both Go and Rust systems interviews |
| `I` | Interview frequency in target verticals |
| `D` | Dependency leverage: unlocks many later packages |

Sort rule:

1. Maximize `M + H + P`.
2. Prefer high `D` foundations early.
3. Use `R` to keep Rust/Go roles both viable.
4. Use `I` to avoid beautiful but rarely asked topics too early.
5. Only then consider implementation effort.

## 3. Current State From Index

Already useful:

- `queue/`: `MutexMPMC`, `MutexMPSC`, `LockFreeMPMC`, `LockFreePaddedMPMC`, `LockFreeMPSC`
- `stack/`: mutex baselines, Treiber stack, elimination-backoff stack
- `syncx/`: spin/ticket/MCS locks, seqlock, semaphore variants, WaitGroup, counting/sense barriers, channel helpers

Highest-value gaps:

- `memory/Index.md`: most items are planned; this is the upstream correctness layer.
- `queue/Index.md`: `LockFreeSPSC` is scaffold; `LamportSPSCRing`, `LMAXDisruptor`, `MulticastRingBuffer` are planned.
- `_lab/verify/Index.md`: no checker/harness layer yet; benchmarks and correctness claims need this.
- `_lab/pattern/Index.md`: `Disruptor`, `ThreadPerCore`, `PipelineBackpressure`, `BackpressurePolicyMatrix` are planned.
- Rust proof is missing from the repo structure; add it as a small companion after Go SPSC.
- `hazard/`, `reclamation/`, `rcu/`: planned; needed for deep Rust/HFT/DB credibility.

## 4. Dependency Graph

```text
memory/
  -> queue/ SPSC, MPSC, MPMC, Michael-Scott
  -> stack/ Treiber ABA/reclamation work
  -> syncx/ MutexLock, condvars, futures, barriers
  -> hazard/, reclamation/, rcu/
  -> map/ lock-free and RCU variants
  -> deque/ Chase-Lev

queue/ + syncx/ + scope/
  -> _lab/pattern/ Disruptor, PipelineBackpressure, ThreadPerCore
  -> actor/ bounded mailbox and scheduler
  -> parallel/ pipeline and work stealing

hazard/ or reclamation/
  -> stack/ TreiberWithHazardPointers
  -> queue/ MichaelScottQueue
  -> map/ lock-free maps

_lab/verify/
  -> every package that claims lock-free, wait-free, linearizable, fair, or race-free behavior
```

## 5. Phase 1 — Performance Correctness Foundation

Goal: make the repo credible before adding more primitives.

### 5.1 `memory/Index.md`

Implement first:

- `AtomicLoadStore`
- `CompareAndSwap`
- `FetchAddCounter`
- `AtomicExchange`
- `HappensBeforeGraph`
- `AcquireReleasePairing`
- `SeqCstOrdering`
- `DataRaceAndDRFSC`
- `FalseSharing`
- `PublicationSafety`
- `ProgressGuarantees`
- `LinearizationPoint`
- `ABAProblem`
- `LitmusTests`

Why:

- This is the vocabulary for every Rust/Go high-performance interview.
- It unlocks `queue/`, `stack/`, `hazard/`, `reclamation/`, `rcu/`, `deque/`, and lock-free `map/`.

Deliverables:

- executable examples, not only prose;
- broken examples where useful;
- `docs/notes/memory-model.md`;
- links to `_lab/verify/LitmusRunner` once that exists.

### 5.2 `_lab/verify/Index.md`

Implement early:

- `HistoryRecorder`
- `RandomizedSchedulerHarness`
- `PropertyBasedConcurrentRunner`
- `RaceDetectorHarness`
- `LitmusRunner`
- `FairnessStarvationChecker`

Defer:

- `DPORScheduleExplorer`
- `ModelCheckingHarness`
- full `HappensBeforeDetector`

Why:

- The repo already has several lock-free claims. The moat is not "I wrote CAS";
  the moat is "I can falsify broken CAS algorithms."

Deliverables:

- one deliberately broken queue/stack variant caught by the harness;
- one reproducible litmus-style memory example;
- one fairness/starvation benchmark for locks or semaphores.

## 6. Phase 2 — Low-Latency Queue Core

Goal: build the highest-signal queue path for HFT, exchanges, streaming engines,
database shard communication, and Rust/C++ interviews.

### 6.1 `queue/Index.md`

Implement in this order:

1. `LamportSPSCRing`
2. complete scaffolded `LockFreeSPSC`
3. `BlockingBoundedQueue`
4. `LMAXDisruptor`
5. `MulticastRingBuffer`

Defer:

- `MichaelScottQueue` until `hazard/` or `reclamation/` exists.
- `WaitFreeQueue`, `LockFreePriorityQueue`, and `TransferQueue` until later.

Why:

- SPSC and Disruptor are the strongest low-latency signals in the Index.
- `BlockingBoundedQueue` is lower moat, but it is the practical bridge to condvars,
  backpressure, and FAANG/top-tier interview floors.

Deliverables:

- `go test -race ./queue`;
- benchmarks: channel vs MPSC vs MPMC vs padded MPMC vs SPSC;
- p50/p95/p99 latency report, not just throughput;
- false-sharing comparison report.

### 6.2 `_lab/pattern/Index.md`

Compose queue primitives into:

- `Disruptor`
- `PipelineBackpressure`
- `BackpressurePolicyMatrix`
- `ThreadPerCore`

Why:

- This turns primitives into recognizable architecture: market-data fan-out,
  exchange sequencer, streaming stage pipeline, shard-per-core DB toy.

Deliverables:

- small runnable example per pattern;
- one benchmark or stress case per high-value pattern;
- explicit slow-consumer policy: block, drop-oldest, drop-newest, shed, degrade.

## 7. Phase 3 — Rust Proof Track

Goal: convert Go concept mastery into Rust-role credibility.

Add a small `rust/` companion tree, starting with:

1. SPSC ring using `UnsafeCell`, explicit `Send`/`Sync`, `Acquire`/`Release`, cache-line alignment.
2. `Arc<T>` from scratch using relaxed increments, release decrement, acquire fence before drop.
3. Treiber stack using `crossbeam-epoch`.

Why:

- `memory/Index.md`, `queue/Index.md`, `hazard/Index.md`, and `reclamation/Index.md`
  transfer strongly to Rust, but Rust hiring still requires visible Rust code.

Deliverables:

- Rust tests;
- short safety comments explaining each unsafe block;
- side-by-side note: Go atomic model vs Rust `Ordering`.

## 8. Phase 4 — Exchange / Streaming / Database Portfolio Labs

Goal: produce non-CRUD artifacts aligned with HFT, crypto exchange, streaming, and
database/storage targets.

### 8.1 Exchange / Market Data Lab

Use Index items:

- `queue/LMAXDisruptor`
- `queue/MulticastRingBuffer`
- `_lab/pattern/Disruptor`
- `_lab/pattern/ThreadPerCore`
- `_lab/pattern/BackpressurePolicyMatrix`
- `ratelimit/TokenBucket`
- `ratelimit/LoadShedding`

Build:

- single-writer per-symbol matching toy;
- SPSC/MPSC ingress;
- sequenced event output;
- Disruptor fan-out;
- deterministic replay;
- risk/admission gate.

### 8.2 Database / Streaming Engine Lab

Use Index items:

- `_lab/pattern/ThreadPerCore`
- `queue/LamportSPSCRing`
- `queue/BlockingBoundedQueue`
- `parallel/PipelineWithBackpressure`
- `map/ShardedMutexMap`
- `map/CopyOnWriteMap`
- `rcu/RCUPointer` later
- `arena/ThreadLocalArena` later

Build:

- shard-per-core key-value toy;
- append-log toy;
- group commit;
- deterministic replay;
- bounded pipeline with backpressure;
- p99 report under slow-consumer or slow-disk simulation.

## 9. Phase 5 — Interview Floor Without Diluting Moat

Goal: cover frequent senior interview primitives after the hot-path artifacts are
underway.

### 9.1 `syncx/Index.md`

Implement:

- `MutexLock`
- `MesaQueueCond`
- `MesaNotifyListCond`
- `Once`
- `OnceValue`
- `FuturePromise`
- `CancellableFuture`

Defer:

- extra lock variants beyond `TTASLock`/`BackoffSpinLock`;
- advanced barriers unless targeting AI training infra;
- STM unless targeting crypto L1 / Block-STM.

Why:

- These build sleeping, wakeup, publication, and async-result competence.
- Condvars unlock bounded blocking queues.
- Futures unlock actor ask-pattern and structured async composition.

### 9.2 `map/Index.md`

Implement:

- `ShardedMutexMap`
- `StripedRWMutexMap`
- `ThreadSafeLRU`
- `CopyOnWriteMap`

Defer:

- `LockFreeOpenAddressingMap`
- `SplitOrderedListMap`
- `LockFreeSkipListMap`
- `ConcurrentResizeProtocol`

Why:

- LRU and sharded map are common FAANG/top-tier infra floors.
- Lock-free maps are deep, but unsafe to prioritize before reclamation.

### 9.3 `ratelimit/Index.md`

Implement:

- `TokenBucket`
- `SlidingWindowCounter`
- `GCRA`
- `CircuitBreaker`
- `LoadShedding`
- `Bulkhead`
- `QueueDepthBackpressure`

Why:

- This is the fintech/exchange/API/inference admission-control layer.
- It is not the deepest low-level moat, so do it after SPSC/Disruptor/benchmark work.

## 10. Phase 6 — Runtime, AI / Agent Infra, and Parallel Systems

Goal: target AI/agent infrastructure and high-throughput runtime roles without
drifting into app-level AI.

### 10.1 `scope/Index.md`

Implement:

- `CancellationToken`
- `TokenTree`
- `Nursery`
- `ErrGroupClone`
- `TaskGroup`
- `BoundedTaskGroup`
- `DeadlineScheduler`
- `CooperativeCancellationBenchmark`

Why:

- Ownership of goroutine lifetime is a senior Go requirement.
- This is the substrate for actor runtime, pipelines, and inference/request orchestration.

### 10.2 `actor/Index.md`

Implement:

- `Mailbox`
- `BoundedMailbox`
- `ActorInterface`
- `ActorRef`
- `AskPattern`
- `Scheduler`
- `Supervisor`
- `GracefulShutdown`

Why:

- Actor runtime matters for AI/agent infra and Ray-style systems.
- Keep it minimal; do not build a large framework before queue/backpressure proof exists.

### 10.3 `parallel/Index.md`

Implement:

- `ParallelMap`
- `ParallelFor`
- `WorkerPool`
- `ParallelReduce`
- `PipelineWithBackpressure`
- `AllReduceRing`
- `AllReduceTree`

Defer:

- `WorkStealingScheduler` until `deque/ChaseLevDeque` exists.
- heavy graph/frontier work until the basics are measured.

Why:

- AI training and database/query engines need work/span, scan/reduce, pipeline,
  and collective communication vocabulary.

## 11. Phase 7 — Deep Lock-Free, Reclamation, and Read-Mostly Systems

Goal: become credible for Rust/HFT/database internals beyond toy lock-free code.

### 11.1 `hazard/Index.md`

Implement:

- `Domain`
- `HazardRecord`
- `Holder`
- `ProtectReloadLoop`
- `ResetProtection`
- `RetireList`
- `ScanAndReclaim`
- `HazardPointerSpecTests`
- `TreiberIntegration`

Then:

- `MichaelScottIntegration`

### 11.2 `reclamation/Index.md`

Implement:

- `EpochBasedReclamation`
- `ReclamationDomain`
- `EBRGuard`
- `LimboBags`
- `TryAdvanceEpoch`
- `StalledParticipantDetection`
- `HazardVsEpochComparison`
- `ReclamationStressHarness`

Why:

- `crossbeam-epoch` mental model is one of the strongest Rust systems signals.
- It unlocks production-grade `queue/MichaelScottQueue`, Treiber upgrades, and lock-free maps.

### 11.3 `stack/Index.md` and `queue/Index.md`

Upgrade:

- `stack/TreiberABAExperiment`
- `stack/TreiberWithHazardPointers`
- `stack/TreiberWithTaggedPointer`
- `queue/MichaelScottQueue`

### 11.4 `rcu/Index.md`

Implement after EBR/QSBR basics:

- `RCUReadSection`
- `AssignPointer`
- `Dereference`
- `RCUPointer`
- `URCU`
- `SynchronizeRCU`
- `QSBRRCU`
- `RCUCorrectnessTests`

Why:

- RCU is high signal for read-mostly databases, routing tables, market-data
  snapshots, AI routing, and kernel-style thinking.

## 12. Phase 8 — Work Stealing, Allocators, and Optional Specialization

### 12.1 `deque/Index.md`

Implement:

- `MutexDeque`
- `BoundedRingDeque`
- `ChaseLevDeque`
- `WorkStealingInjector`
- `WorkStealingPool`

Why:

- This unlocks `parallel/ForkJoin`, `parallel/WorkStealingScheduler`, and Rust
  `crossbeam-deque` interview transfer.

### 12.2 `arena/Index.md`

Implement if targeting HFT/database/AI memory-management roles:

- `BumpAllocator`
- `TypedArena`
- `ResettableRegion`
- `ThreadLocalArena`
- `SlabAllocator`
- `AllocationBenchmarkSuite`

Why:

- Allocation latency and lifecycle control matter for HFT, storage engines,
  parsers, queue nodes, actor messages, and vLLM-style block pools.

### 12.3 Later / Niche Tracks

Only pull these forward for a specific target:

- `syncx/STM`: crypto L1 / Block-STM / OCC / MVCC roles.
- `clock/HybridLogicalClock`, `clock/VectorClock`: distributed DB and replication roles.
- `crdt/ORSet`, `crdt/LWWMap`, `crdt/DeltaStateReplication`: collaborative/distributed data roles.
- advanced `syncx` barriers: AI training/NCCL collective path.
- `_lab/excercise`: interview drills, not roadmap driver.

## 13. 12-Week Execution Plan

```text
Week 01  memory: AtomicLoadStore, CAS, FetchAdd, HappensBeforeGraph, AcquireReleasePairing
Week 02  memory: FalseSharing, PublicationSafety, ProgressGuarantees, LinearizationPoint, ABAProblem
Week 03  queue: LamportSPSCRing / LockFreeSPSC + FIFO/full/empty/wraparound/race tests
Week 04  benchmark/report: SPSC vs MPSC vs MPMC vs channel; padded vs unpadded; p50/p95/p99
Week 05  _lab/verify: HistoryRecorder + RandomizedSchedulerHarness + RaceDetectorHarness
Week 06  queue + pattern: LMAXDisruptor minimum + slow-consumer policy
Week 07  pattern: ThreadPerCore + PipelineBackpressure + BackpressurePolicyMatrix
Week 08  exchange lab: single-writer matching toy + sequenced replay
Week 09  database/streaming lab: shard-per-core KV + append-log/group-commit toy
Week 10  syncx/cond + queue/BlockingBoundedQueue
Week 11  Rust companion: SPSC ring with UnsafeCell/Send/Sync/Acquire/Release proof
Week 12  map/ThreadSafeLRU + ratelimit/TokenBucket + final benchmark/resume report
```

If time is limited to four weeks:

```text
memory core -> SPSC -> benchmark report -> Disruptor minimum
```

If targeting Rust roles immediately:

```text
memory core -> Go SPSC -> Rust SPSC -> Arc<T> -> Treiber + crossbeam-epoch
```

If targeting HFT / exchange:

```text
SPSC -> Disruptor -> ThreadPerCore -> matching lab -> p99 report -> Rust SPSC
```

If targeting database / streaming:

```text
memory -> SPSC -> ThreadPerCore -> append-log/group-commit -> LRU/cache -> RCU/EBR
```

If targeting AI / agent infra:

```text
queue scheduler -> scope -> FuturePromise -> actor mailbox -> bounded task group
-> inference/agent gateway demo -> AllReduce only if training infra
```

## 14. Definition of Done

Every roadmap item that moves from `[ ]` or `[~]` to `[x]` in an `Index.md` must have:

- code;
- unit tests;
- at least one concurrency/race/stress test when relevant;
- benchmark when performance is part of the claim;
- a short note explaining invariants and linearization/progress where relevant;
- Index status updated in the same change.

Do not mark an item `[x]` just because a type exists.

For lock-free items, also require:

- identified linearization point;
- progress guarantee statement;
- memory-ordering explanation;
- broken-variant or stress test where practical.

For pattern labs, also require:

- runnable example;
- explicit backpressure/overflow/shutdown policy;
- one benchmark or stress case if the pattern is performance-related.

## 15. What Not To Do Yet

- Do not implement more lock variants before SPSC, Disruptor, and p99 reports.
- Do not start full CRDT/clock suites unless targeting distributed DB/collab roles.
- Do not implement lock-free maps before reclamation exists.
- Do not build a large actor framework before `scope/`, `FuturePromise`, and bounded mailboxes exist.
- Do not build advanced barriers before deciding to target AI training/NCCL-style roles.
- Do not call a primitive production-grade if `_lab/verify` cannot falsify at least one broken version.

## 16. One-Line Summary

```text
Index-driven path:
memory -> verify -> SPSC -> benchmarks -> Disruptor/ThreadPerCore
-> exchange + database/streaming labs -> Rust proof -> cond/LRU/ratelimit floors
-> scope/actor/parallel -> hazard/EBR/RCU/deque/arena specialization
```

This keeps the project aligned with your new Markdown structure and with the
Rust/Go high-performance market target.
