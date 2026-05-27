# stack Index

> Learning goal: study LIFO transfer, single-hotspot CAS contention, ABA, and contention mitigation through elimination and combining.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: concurrent push/pop histories must be linearizable to a LIFO stack ordered by operation commit points.
- The central bottleneck is the single `head` pointer; most advanced variants either reduce traffic on that pointer or make the pointer safe to reclaim.
- Existing lock-free implementations are educational under Go GC, but the merged TODO treats Treiber plus hazard pointers as the production-grade upgrade.
- Recommended build order from the merged TODO: Treiber with hazard pointers, then flat combining, then bounded array stack, then optional wait-free/tagged-pointer variants.
- Dependencies: reclamation work depends on `hazard/` or `reclamation/`; all lock-free variants depend on `memory/` for CAS and publication reasoning.
- Career signal: lower than queues, but Treiber plus ABA plus reclamation is a classic senior systems interview topic.
- Scope rule: keep stack variants focused on CAS hotspots, ABA, reclamation, and contention mitigation; exotic language-specific allocator tricks belong in notes, not required implementations.

## Implementation Checklist

- [x] MutexSliceMPMC
  - Core Concept: A mutex protects a slice-backed stack where push/pop operate at the slice end.
  - Pros: Simple, cache-friendly, and a strong baseline.
  - Cons: All operations serialize and resizing can occur on push.
  - Scenarios: Baseline benchmarks and correctness comparison for lock-free stacks.

- [x] MutexLinkedMPMC
  - Core Concept: A mutex protects a linked-list stack rooted at head.
  - Pros: No slice resizing and mirrors Treiber's node shape.
  - Cons: Per-push allocation and serialized access.
  - Scenarios: Linked baseline for Treiber stack and memory-allocation comparisons.

- [x] LockFreeMPMC
  - Core Concept: Treiber stack pushes and pops by CAS-ing the head pointer.
  - Pros: Minimal lock-free data structure and clear linearization point.
  - Cons: Head is a contention hotspot and production safety requires ABA/reclamation handling.
  - Scenarios: CAS education, ABA demonstrations, lock-free interview prep.

- [ ] TreiberABAExperiment
  - Core Concept: Force a stale observed head to become valid again after an A -> B -> A sequence.
  - Pros: Makes the ABA problem concrete instead of only theoretical.
  - Cons: In Go, reproducing pointer reuse usually needs a simulated allocator or unsafe test harness.
  - Scenarios: ABA labs, tagged pointer comparison, hazard pointer motivation.

- [x] EliminationBackoffMPMC
  - Core Concept: Failed push/pop operations try to meet in an elimination array and exchange directly.
  - Pros: Reduces head CAS pressure under mixed push/pop contention.
  - Cons: More moving parts, probabilistic performance, and tuning-sensitive.
  - Scenarios: High-contention stack benchmarks and elimination-array study.

- [ ] EliminationArray
  - Core Concept: Push and pop operations pair off in bounded exchange slots before touching the central stack.
  - Pros: Separates the reusable exchange mechanism from the stack implementation.
  - Cons: Timeouts and slot selection determine throughput and fairness.
  - Scenarios: Reusable exchanger labs, contention scaling experiments, stack-vs-queue comparison.

- [ ] TreiberWithHazardPointers
  - Core Concept: Pop protects the observed head with a hazard pointer before reading and retiring it.
  - Pros: Makes Treiber production-grade in non-GC or manual-reclamation contexts.
  - Cons: Requires a hazard pointer domain, retire lists, and scan thresholds.
  - Scenarios: Integrating `hazard/`, ABA-safe stack, memory reclamation teaching.

- [ ] TreiberWithTaggedPointer
  - Core Concept: Pair the pointer with a version tag so A -> B -> A changes are detectable by CAS.
  - Pros: Direct ABA defense without retire scanning.
  - Cons: Needs wide CAS or pointer/tag packing assumptions.
  - Scenarios: Low-level systems interviews, ABA alternatives.

- [ ] FlatCombiningStack
  - Core Concept: Threads publish operations and one combiner executes a batch sequentially.
  - Pros: Converts high-contention CAS storms into one sequential critical path.
  - Cons: Reduces parallelism and fairness depends on combiner behavior.
  - Scenarios: Synchronization-parallelism tradeoff, Folly-style design study.

- [ ] CombiningPublicationList
  - Core Concept: Callers publish pending push/pop requests in slots that a combiner claims and resolves.
  - Pros: Makes flat combining reusable for maps, counters, and other objects.
  - Cons: Requires careful cleanup when callers cancel or time out.
  - Scenarios: Flat combining internals, high-contention data structure labs.

- [ ] BoundedArrayStack
  - Core Concept: A fixed array and atomic top index implement a capacity-limited stack.
  - Pros: No node allocation and no reclamation problem.
  - Cons: Bounded capacity and publication ordering must be exact.
  - Scenarios: Work buffers, embedded/arena-backed stacks, memory-ordering exercises.

- [ ] LockFreeFreelist
  - Core Concept: A stack of reusable nodes provides allocation-free pop/push for fixed-size objects.
  - Pros: Shows why Treiber stacks appear inside allocators and pools.
  - Cons: ABA risk is severe without tags, hazard pointers, or epochs.
  - Scenarios: Slab allocator freelists, queue node reuse, reclamation demonstrations.

- [ ] WaitFreeStack
  - Core Concept: Operations announce intent and help others finish so every caller completes in bounded steps.
  - Pros: Strong progress guarantee.
  - Cons: Considerably more complex and rarely worth the constant factor.
  - Scenarios: Advanced progress taxonomy and academic comparison.
