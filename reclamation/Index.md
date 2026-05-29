# reclamation Index

> Learning goal: compare the major ways to prove that no concurrent reader can still hold a removed node.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: freeing is safe only after proving no active participant can still hold a removed node pointer.
- Main proof families from the merged TODO: reader announcement with hazard pointers, epoch pinning with EBR, explicit quiescent states with QSBR, and reference ownership with DRC.
- EBR is the preferred next implementation here because it pairs naturally with linked lock-free queues and exposes stalled-reader memory retention.
- Recommended build order from the merged TODO: EBR, then QSBR with `rcu/`, then DRC, then interval-based reclamation as a paper-level extension.
- Dependencies: depends on `memory/`; consumed by `queue/`, `stack/`, `map/`, and `rcu/`.
- Career signal: advanced but important; the best interview answer to "what breaks in your lock-free stack?" is ABA plus unsafe reclamation.
- Scope rule: include reclamation strategies that can be simulated or implemented behind explicit Go APIs; allocator-specific free-list tricks stay in `arena/` or notes.

## Reference Trail and Go Boundary

- Classic reclamation line: hazard pointers, Fraser-style epoch reclamation (`https://www.cl.cam.ac.uk/techreports/UCAM-CL-TR-579.pdf`), and Linux RCU grace periods (`https://docs.kernel.org/RCU/`).
- Mental model: logical removal and physical reuse/free are separate events. Reclamation proves no reader can still hold the old pointer between those events.
- Go boundary: GC removes ordinary free/reuse pressure, so this package should use explicit retire callbacks, simulated allocators, or unsafe labs to teach the proof without pretending Go requires manual free.
- EBR boundary: epochs are cheap for readers but one stuck pinned participant can retain unbounded retired memory.
- QSBR boundary: quiescent-state reporting is nearly free in hot read sections but only works if every participant cooperates.
- Interview artifact: every reclamation strategy should include a stalled-reader scenario and explain safety, memory growth, and progress separately.

## Implementation Checklist

- [ ] EpochBasedReclamation
  - Core Concept: Readers pin the current global epoch; retired nodes are freed only after enough epoch advancement proves old readers are gone.
  - Pros: Cheaper reader path than hazard pointers for many operations.
  - Cons: A stuck pinned reader can cause unbounded memory retention.
  - Scenarios: Lock-free maps, Michael-Scott queue, epoch-protected linked structures.

- [ ] ReclamationDomain
  - Core Concept: A domain owns participants, retired records, epoch state, and reclamation callbacks for related structures.
  - Pros: Keeps lifecycle and scanning policy explicit instead of global.
  - Cons: APIs must prevent mixing guards and retired nodes from different domains.
  - Scenarios: Queue/stack/map integration and benchmark isolation.

- [ ] EBRGuard
  - Core Concept: A guard represents a pinned critical section and owns deferred frees until unpin.
  - Pros: Makes correct usage explicit in APIs.
  - Cons: Guards must not be leaked or held across blocking work.
  - Scenarios: Queue/stack operations and map traversal.

- [ ] LimboBags
  - Core Concept: Retired objects are grouped by epoch modulo a small number of bags.
  - Pros: Batch reclaim reduces per-node overhead.
  - Cons: Bag rotation and epoch advancement require exact invariants.
  - Scenarios: EBR implementation internals.

- [ ] TryAdvanceEpoch
  - Core Concept: The global epoch advances only when all pinned readers are inactive or pinned in a compatible epoch.
  - Pros: Central proof step for EBR safety.
  - Cons: Scanning all participants can be expensive.
  - Scenarios: Stress tests for stalled readers and memory growth.

- [ ] StalledParticipantDetection
  - Core Concept: Track participants that remain pinned too long and report memory-retention risk.
  - Pros: Makes EBR/QSBR failure modes visible during tests.
  - Cons: Detection thresholds are heuristic and should not force unsafe reclamation.
  - Scenarios: Reclamation stress harness, production observability, leak-style debugging.

- [ ] QSBR
  - Core Concept: Readers explicitly report quiescent states outside critical sections; writers reclaim after every participant passed a safe point.
  - Pros: Zero atomic cost inside read-side critical sections.
  - Cons: Requires disciplined safe-point calls; missed quiescence delays freeing.
  - Scenarios: Read-mostly long-running systems, userspace RCU variants.

- [ ] QSBRThreadRegistration
  - Core Concept: Participants register and publish their last observed quiescent counter.
  - Pros: Keeps writer-side synchronization explicit.
  - Cons: Registration lifecycle and goroutine identity are nontrivial in Go.
  - Scenarios: Service loops, schedulers, actor runtimes.

- [ ] HazardVsEpochComparison
  - Core Concept: Run the same data structure with hazard pointers and EBR to compare reader cost, reclaim latency, and stalled-reader behavior.
  - Pros: Builds intuition for choosing a reclamation family.
  - Cons: Requires equivalent APIs and benchmarks to avoid misleading results.
  - Scenarios: Treiber stack, Michael-Scott queue, interview tradeoff explanations.

- [ ] DeferredReferenceCounting
  - Core Concept: Each node tracks references and is freed when the count reaches zero after deferral rules are satisfied.
  - Pros: Bounded memory and intuitive ownership model.
  - Cons: Atomic refcount updates add reader-side cache traffic.
  - Scenarios: Shared pointer internals, `Arc`/`shared_ptr` comparisons.

- [ ] IntervalBasedReclamation
  - Core Concept: Readers publish active intervals and retired nodes are reclaimed when outside all intervals.
  - Pros: Hybrid of HP precision and EBR batching.
  - Cons: Paper-level complexity and more metadata.
  - Scenarios: Advanced reclamation research and bounded-memory lock-free structures.

- [ ] ReclamationStressHarness
  - Core Concept: Force delayed readers, rapid retire, and repeated scans to expose memory-safety or retention bugs.
  - Pros: Makes subtle reclamation failures reproducible.
  - Cons: Requires unsafe hooks or controlled alloc/free simulation in Go.
  - Scenarios: Validating hazard, EBR, QSBR, and RCU implementations.
