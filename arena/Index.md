# arena Index

> Learning goal: control allocation latency and lifetime by grouping memory into regions, slabs, and pools instead of relying on per-object allocation.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core invariant: allocation is cheap because lifetime is coarse; individual frees are either unsupported or constrained to fixed-size slabs.
- Arena users must accept that reset invalidates every outstanding pointer.
- Thread-safe allocation can use atomic bumping, but low-latency designs usually prefer worker-local arenas to avoid shared cache-line traffic.
- Recommended build order from the merged TODO: bump allocator, typed arena, slab allocator, then multi-size pool allocator.
- Dependencies: depends on `memory/` for alignment and atomic bumping; lock-free slab freelists may depend on `hazard/` or tagged pointers.
- Career signal: niche for FAANG interviews but strong for HFT, game-engine, database, and AI-infra memory-management work.
- Scope rule: include allocation patterns that teach concurrency and lifetime; skip GC/runtime internals that cannot be portably implemented in Go.

## Reference Trail and Go Boundary

- Reference line: Go runtime allocation is useful background, but this package should focus on implementable regions, slabs, pools, freelists, and debug lifetime checks.
- Mental model: allocation performance is a lifetime-design problem. The fastest free is usually "reset the whole region," but every outstanding pointer becomes invalid.
- Go boundary: ordinary Go cannot expose safe arbitrary object placement and manual free. Use byte buffers, typed slices, pooled objects, and narrow `unsafe` experiments only when the lifetime rule is explicit.
- Concurrency boundary: atomic bump allocation is simple but creates one hot offset; worker-local arenas trade memory balance for locality.
- ABA boundary: slab freelists are Treiber stacks in allocator clothing, so tags, hazard pointers, epochs, or GC assumptions must be named.
- Interview artifact: every allocator item should state ownership, reset/free semantics, alias invalidation, and whether returned objects can cross worker boundaries.

## Implementation Checklist

- [ ] BumpAllocator
  - Core Concept: A pointer moves through a preallocated byte buffer; individual frees are not supported.
  - Pros: O(1) allocation, predictable latency, no fragmentation inside the region.
  - Cons: Memory is reclaimed only by resetting the whole arena.
  - Scenarios: Per-request/per-tick allocation, HFT hot path, parser scratch space.

- [ ] TypedArena
  - Core Concept: A type-specific slice provides aligned storage and returns `*T` values.
  - Pros: Avoids unsafe pointer arithmetic for simple type-specific use.
  - Cons: Only supports one type per arena and reset invalidates all returned pointers.
  - Scenarios: Node pools, AST allocations, queue node batches.

- [ ] ResettableRegion
  - Core Concept: All allocations share a lifetime and are discarded together by resetting the offset.
  - Pros: Very cheap cleanup and strong lifecycle boundary.
  - Cons: Use-after-reset bugs are severe and not statically prevented in Go.
  - Scenarios: Request scopes, matching-engine tick buffers, batch processing.

- [ ] ConcurrentBumpAllocator
  - Core Concept: Multiple goroutines reserve ranges with atomic CAS or fetch-add on the offset.
  - Pros: Lock-free allocation path for fixed regions.
  - Cons: Shared offset is a cache-line hotspot under heavy allocation.
  - Scenarios: Parallel parsing, worker-local arenas, allocator benchmarks.

- [ ] ThreadLocalArena
  - Core Concept: Each worker owns an arena, avoiding shared atomic allocation.
  - Pros: Excellent locality and no inter-worker allocation contention.
  - Cons: Load imbalance can waste memory.
  - Scenarios: Thread-per-core systems, work-stealing workers, HFT services.

- [ ] PerWorkerPool
  - Core Concept: Each worker keeps local free objects and occasionally exchanges with a shared pool.
  - Pros: Reduces central lock contention and cache-line bouncing.
  - Cons: Memory can become imbalanced across workers.
  - Scenarios: Actor message pools, queue node reuse, scheduler-local buffers.

- [ ] SlabAllocator
  - Core Concept: Fixed-size blocks are reused through a freelist, backed by larger slabs.
  - Pros: O(1) allocation/free for same-size objects and lower fragmentation.
  - Cons: Lock-free freelists need ABA protection or tagged pointers.
  - Scenarios: Queue nodes, actor messages, vLLM-style fixed block pools.

- [ ] LockFreeFreelistPool
  - Core Concept: Free objects are stored in a Treiber-style freelist with ABA protection or safe reclamation.
  - Pros: Directly connects allocation to stack, ABA, and hazard-pointer lessons.
  - Cons: Easy to become unsafe without tags, epochs, or hazard pointers.
  - Scenarios: Slab internals, fixed-size object pools, reclamation labs.

- [ ] PoolAllocator
  - Core Concept: Multiple slab classes serve different size ranges.
  - Pros: Practical general-purpose pooling while keeping bounded sizes.
  - Cons: More bookkeeping and internal fragmentation.
  - Scenarios: Runtime allocator study, network buffers, cache objects.

- [ ] AllocationBenchmarkSuite
  - Core Concept: Compare per-object allocation, arena reset, slab reuse, and per-worker pools under contention.
  - Pros: Turns lifetime and locality tradeoffs into measurable data.
  - Cons: Benchmark results are sensitive to object size and escape analysis.
  - Scenarios: HFT hot paths, queue node allocation, actor message throughput.

- [ ] ArenaDebugMode
  - Core Concept: Add generation counters or poisoning to detect use after reset.
  - Pros: Helps make arena lifetime bugs observable during tests.
  - Cons: Adds overhead and cannot catch all unsafe aliasing.
  - Scenarios: Teaching lifetime discipline and validating arena users.
