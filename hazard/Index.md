# hazard Index

> Learning goal: make lock-free node reclamation safe by letting readers announce which pointers they may dereference.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: a node may be reclaimed only after no hazard slot in the domain contains its address.
- The protect protocol is `load -> publish hazard -> fence -> reload -> retry if changed`; skipping the reload is the classic bug.
- Hazard pointers shift cost to readers: each protected dereference pays store/fence/reload, while reclamation cost is amortized by retire-list scans.
- Recommended build order from the merged TODO: global domain with fixed slots, dynamic holder registration, Treiber stack integration, then amortized reclaim thresholds.
- Dependencies: depends on `memory/`; consumed by `stack/`, Michael-Scott queue in `queue/`, and optionally `rcu/`.
- Career signal: strong for systems interviews because it turns lock-free structures from toy examples into memory-safe designs.
- Scope rule: focus on the portable hazard-pointer protocol; avoid depending on goroutine-local storage and require explicit holder ownership in Go.

## Implementation Checklist

- [~] HazardPointerScaffold
  - Core Concept: `pointer.go` establishes the package but currently contains no domain, holder, or record implementation.
  - Pros: Reserves the correct package boundary for hazard-pointer work.
  - Cons: No usable API exists yet.
  - Scenarios: Starting point for upgrading Treiber stack and Michael-Scott queue.

- [ ] Domain
  - Core Concept: A domain owns all hazard records and retired nodes for a set of data structures.
  - Pros: Keeps scanning local to related structures and avoids global interference.
  - Cons: Requires registration and lifecycle management.
  - Scenarios: Per-stack or per-queue hazard pointer management.

- [ ] HazardRecord
  - Core Concept: A record contains one or more atomic slots where a reader publishes pointers it may dereference.
  - Pros: Gives reclaimers a precise set of currently protected nodes.
  - Cons: Slot count must be chosen for each algorithm, commonly one or two.
  - Scenarios: Protecting Treiber head, Michael-Scott head/tail/next pointers.

- [ ] MultiSlotProtection
  - Core Concept: An operation protects several related pointers at once, such as `head`, `tail`, and `next`.
  - Pros: Required for realistic linked queues and complex lock-free structures.
  - Cons: Slot ordering and reset discipline become harder to audit.
  - Scenarios: Michael-Scott queue, linked maps, delete-heavy skip lists.

- [ ] Holder
  - Core Concept: A goroutine-local or operation-local handle owns hazard slots and retire state.
  - Pros: Encapsulates protect/reset/retire calls cleanly.
  - Cons: Go lacks stable goroutine-local storage, so pooling or explicit handles are needed.
  - Scenarios: API used by queue/stack operations.

- [ ] ProtectReloadLoop
  - Core Concept: Load pointer, publish it to a hazard slot, fence, reload, and retry if the pointer changed.
  - Pros: This is the core correctness rule that makes dereference safe.
  - Cons: Adds reader-side store/fence cost and retry complexity.
  - Scenarios: `Pop` in Treiber stack, `Dequeue` in linked lock-free queues.

- [ ] ResetProtection
  - Core Concept: Clear a hazard slot after the protected pointer is no longer needed.
  - Pros: Allows retired nodes to be reclaimed sooner.
  - Cons: Forgetting reset causes memory retention.
  - Scenarios: End of pop/dequeue operation, long-running readers.

- [ ] RetireList
  - Core Concept: Unlinked nodes are placed in a deferred retire list instead of being freed immediately.
  - Pros: Separates logical removal from physical reclamation.
  - Cons: Memory can grow until scans occur.
  - Scenarios: Lock-free node deletion, stack pop, queue dequeue.

- [ ] ScanAndReclaim
  - Core Concept: Collect all hazard slots and free retired nodes not present in any slot.
  - Pros: Provides bounded safety without a garbage collector.
  - Cons: Scan cost is O(number of hazard slots plus retire list).
  - Scenarios: Threshold-based reclamation and memory pressure control.

- [ ] RetireThresholdPolicy
  - Core Concept: Trigger scans after a retire-list threshold, memory budget, or explicit flush.
  - Pros: Makes amortized reclamation cost measurable and tunable.
  - Cons: Too-low thresholds scan too often; too-high thresholds retain memory.
  - Scenarios: Benchmarks, queue node reclamation, memory pressure experiments.

- [ ] HazardPointerSpecTests
  - Core Concept: Test protect/reload/reset/retire invariants against deliberately broken variants.
  - Pros: Catches the classic missing-reload and forgotten-reset bugs.
  - Cons: Requires controlled interleavings or deterministic hooks to be reliable.
  - Scenarios: `_lab/verify` integration and reclamation education.

- [ ] TreiberIntegration
  - Core Concept: Protect stack head before reading `head.next`; retire the old head after successful CAS.
  - Pros: Converts a toy Treiber stack into a production-grade teaching example.
  - Cons: Adds API friction and benchmark overhead.
  - Scenarios: ABA/UAF demonstration and stack package upgrade.

- [ ] MichaelScottIntegration
  - Core Concept: Protect queue nodes while reading `next` and advancing head/tail.
  - Pros: Enables safe unbounded lock-free queues.
  - Cons: Requires multiple hazard slots and careful helping paths.
  - Scenarios: `queue/` unbounded MPMC implementation.
