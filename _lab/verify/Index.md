# verify Index

> Learning goal: build tools that test concurrent algorithms by checking histories, schedules, locks, happens-before edges, and memory-ordering litmus cases.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Lab Notes

- This lab is for proving and falsifying concurrency claims. The output should be tools that test other packages, not application features.
- A useful checker needs a model, a history format, and at least one deliberately broken implementation it can catch.
- Recommended build order from the merged TODO: brute-force linearizability checker, property-based runner, runtime deadlock detector, Eraser lockset, litmus runner, happens-before detector, then DPOR.
- Dependencies: uses `clock/` vector clocks for happens-before, `memory/` for litmus cases, and exercises `queue/`, `stack/`, `mapx/`, and `syncx/`.
- Career signal: high for correctness-focused systems roles because it shows you do not trust intuition alone.
- Scope rule: every checker should ship with at least one deliberately broken implementation or schedule that it catches.

## Implementation Checklist

- [ ] LinearizabilityChecker
  - Core Concept: Given operation call/return intervals, find whether some legal sequential order explains the results.
  - Pros: Directly validates queue/stack/map correctness.
  - Cons: Search is expensive and needs small histories or pruning.
  - Scenarios: Lock-free data structure tests and Jepsen/Porcupine-style reasoning.

- [ ] HistoryRecorder
  - Core Concept: Wrap operations and record start time, end time, arguments, and results.
  - Pros: Reusable input format for checkers.
  - Cons: Instrumentation can perturb schedules.
  - Scenarios: Queue/stack stress tests and linearizability traces.

- [ ] SequentialConsistencyChecker
  - Core Concept: Check for a per-goroutine-order-preserving sequential explanation, weaker than linearizability.
  - Pros: Useful for contrasting consistency models.
  - Cons: Passing SC does not prove real-time order correctness.
  - Scenarios: Memory model education and register examples.

- [ ] RandomizedSchedulerHarness
  - Core Concept: Run operations with randomized sleeps, yields, and interleavings while checking invariants.
  - Pros: Cheap way to shake out races and lost updates.
  - Cons: Probabilistic and cannot prove absence of bugs.
  - Scenarios: Chaos tests for lock-free queue/stack.

- [ ] PropertyBasedConcurrentRunner
  - Core Concept: Generate random operation sequences and verify invariants over many runs.
  - Pros: Finds edge cases humans do not enumerate.
  - Cons: Requires shrinking and good models for useful failures.
  - Scenarios: Queue conservation, stack multiset checks, map invariants.

- [ ] DeadlockDetectorRuntime
  - Core Concept: Track goroutine-to-lock wait-for edges and search for cycles.
  - Pros: Exposes deadlocks with actionable graphs.
  - Cons: Go lacks public goroutine IDs, so instrumentation is awkward.
  - Scenarios: Lock ordering experiments and dining philosophers variants.

- [ ] LocksetChecker
  - Core Concept: Track the intersection of locks held for each shared variable and report when it becomes empty.
  - Pros: Teaches Eraser/TSan lineage.
  - Cons: False positives for lock-free and barrier-protected code.
  - Scenarios: Race detector internals and monitor object validation.

- [ ] HappensBeforeDetector
  - Core Concept: Use vector clocks to track synchronization edges and detect conflicting accesses without ordering.
  - Pros: Closer to modern race detectors than lockset alone.
  - Cons: Instrumentation overhead and vector-clock bookkeeping are high.
  - Scenarios: Mini `go -race`, memory model education.

- [ ] RaceDetectorHarness
  - Core Concept: Run package tests under the Go race detector and keep known race examples isolated.
  - Pros: Gives immediate feedback for ordinary data races.
  - Cons: Cannot prove race freedom and may miss unexecuted paths.
  - Scenarios: CI checks, teaching race reports, validating monitor-style code.

- [ ] SynctestHarness
  - Core Concept: Use Go's `testing/synctest` style of isolated concurrent bubbles to test cancellation, time, and wakeup behavior without sleeps.
  - Pros: Makes future, context, and waiter-queue tests more deterministic than wall-clock polling.
  - Cons: It validates Go-runtime-visible blocking behavior, not arbitrary hardware interleavings or lock-free proofs.
  - Scenarios: ContextCond, SingleFlightGroup, futures, DeadlineScheduler, actor shutdown.

- [ ] LitmusRunner
  - Core Concept: Repeatedly run tiny reorder-sensitive programs and summarize observed outcomes.
  - Pros: Makes memory ordering visible.
  - Cons: Go's atomic model and compiler behavior limit what can be demonstrated.
  - Scenarios: `memory/` exercises, x86 vs ARM experiments.

- [ ] DPORScheduleExplorer
  - Core Concept: Systematically explore representative schedules using dynamic partial-order reduction.
  - Pros: Stronger than random scheduling for small programs.
  - Cons: Implementation complexity is high.
  - Scenarios: Formal-ish testing of small primitives and educational schedulers.

- [ ] ModelCheckingHarness
  - Core Concept: Run a small instrumented algorithm against all bounded schedules or states.
  - Pros: Finds bugs random stress often misses.
  - Cons: State explosion requires tiny models and aggressive pruning.
  - Scenarios: Mutex/latch state machines, bounded queues, cancellation races.

- [ ] ReclamationStressHarness
  - Core Concept: Force delayed readers, rapid retire, and repeated reclamation scans.
  - Pros: Exposes memory-retention and unsafe-free bugs hidden by normal GC behavior.
  - Cons: Needs simulated allocation or unsafe hooks to be meaningful in Go.
  - Scenarios: Hazard pointers, EBR, QSBR, RCU tests.

- [ ] FairnessStarvationChecker
  - Core Concept: Track wait time, service order, and bounded progress for waiters under contention.
  - Pros: Catches starvation bugs that preserve safety but violate liveness.
  - Cons: Liveness claims need workload and timing assumptions.
  - Scenarios: Semaphore fairness, RWMutex variants, select fairness, actor schedulers.

- [ ] ScheduleTraceVisualizer
  - Core Concept: Render goroutine operations, waits, wakes, and happens-before edges as a timeline.
  - Pros: Makes difficult concurrency failures explainable after a test run.
  - Cons: Instrumentation can perturb timing and traces can be large.
  - Scenarios: Debugging lost wakeups, lock convoys, DPOR counterexamples.

- [ ] MailboxProtocolChecker
  - Core Concept: Generate message sequences and verify actor state machines handle calls, casts, timeouts, and unexpected messages.
  - Pros: Finds mailbox protocol bugs that ordinary data-race tools cannot see.
  - Cons: Requires a precise model for allowed messages and replies.
  - Scenarios: Actor ask/cast tests, selective receive, late-reply races.

- [ ] SupervisorFaultInjection
  - Core Concept: Inject worker crashes and verify restart strategy, shutdown order, and restart intensity behavior.
  - Pros: Turns fault tolerance into executable tests.
  - Cons: Needs deterministic control over failures and worker side effects.
  - Scenarios: Supervision trees, crash-only components, supervised worker pools.

- [ ] DeterministicRuntimeModel
  - Core Concept: Replace real scheduling and timers with a controlled runtime that explores wakeups, polls, and message delivery orders.
  - Pros: Makes async/task races reproducible instead of relying on sleeps.
  - Cons: Requires instrumented primitives and cannot model every runtime behavior.
  - Scenarios: Future/waker tests, actor mailboxes, cancellation races, timeout logic.

- [ ] PorcupineLinearizability
  - Core Concept: A practical linearizability checker (Go Porcupine) that searches for a valid sequential order using the Wing-Gong algorithm with partitioning.
  - Pros: Fast enough for real test histories and widely used to validate concurrent data structures.
  - Cons: Still worst-case exponential, so histories and models must be kept small.
  - Scenarios: Queue/stack/map linearizability tests, Jepsen-style validation.

- [ ] LincheckStyleChecker
  - Core Concept: Generate random concurrent scenarios and bounded-model-check them against a sequential specification, in the style of JetBrains Lincheck.
  - Pros: Finds data-structure bugs automatically without hand-written interleavings.
  - Cons: Needs a sequential oracle and bounded scenario sizes, and Go lacks Lincheck's JVM instrumentation.
  - Scenarios: Lock-free structure testing, regression suites, specification-based checking.

- [ ] CHESSPreemptionBounding
  - Core Concept: CHESS (Musuvathi-Qadeer 2007) systematically enumerates schedules while bounding the number of preemptions.
  - Pros: Finds most concurrency bugs with few preemptions, taming the schedule explosion.
  - Cons: Requires controlling the scheduler at synchronization points, which is hard in Go.
  - Scenarios: Small-primitive bug finding, scheduler-controlled testing, mutex/queue races.

- [ ] OptimalDPOR
  - Core Concept: Optimal DPOR (Abdulla et al 2014) uses source sets and wakeup trees to explore exactly one interleaving per Mazurkiewicz trace.
  - Pros: Avoids redundant equivalent schedules, far fewer runs than naive DPOR.
  - Cons: Bookkeeping for source sets and wakeup trees is intricate.
  - Scenarios: Stateless model checking, exhaustive small-program exploration, DPOR study.

- [ ] MaximalCausalityReduction
  - Core Concept: MCR (Huang 2015) explores only maximal causal models of a trace, a stronger reduction than partial-order methods.
  - Pros: Fewer explored schedules than DPOR for the same coverage.
  - Cons: Relies on an SMT solver to derive feasible schedules, raising implementation cost.
  - Scenarios: Advanced schedule exploration, research-grade checking, reduction comparison.

- [ ] TLAPlusSpec
  - Core Concept: Specify a primitive's state machine in TLA+/PlusCal and model-check invariants and liveness with TLC.
  - Pros: Catches design-level concurrency bugs before any code exists.
  - Cons: A separate spec language and tool, kept as companion artifacts rather than runnable Go.
  - Scenarios: Designing locks/queues/protocols, invariant specification, design-first verification.

- [ ] CoverageGuidedConcurrencyFuzz
  - Core Concept: Mutate scheduling choices such as Go select-case order, guided by coverage, to drive executions toward unexplored interleavings (GFuzz-style).
  - Pros: Finds order-dependent bugs that fixed tests and random stress miss.
  - Cons: Needs instrumentation of scheduling points and a coverage signal to guide mutation.
  - Scenarios: Channel/select order bugs, message-order races, fuzzing concurrent code.

- [ ] FastTrackRaceDetector
  - Core Concept: FastTrack (Flanagan-Freund 2009) is the efficient epoch-based vector-clock happens-before algorithm behind modern race detectors.
  - Pros: Near-constant-time per access in the common case, the lineage of TSan and go -race.
  - Cons: Still adds memory and time overhead and only flags races on executed paths.
  - Scenarios: Mini race-detector build, happens-before education, instrumentation study.
