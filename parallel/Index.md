# parallel Index

> Learning goal: learn how to decompose work, reason about work/span, coordinate stages, and connect algorithms to synchronization primitives.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core model: reason about work `W`, span `S`, and available parallelism `W/S`; speedup is limited by span and scheduling overhead.
- Algorithms here should document associativity, partitioning, synchronization points, and memory movement.
- Pipeline algorithms should use bounded queues and `scope/` cancellation so slow downstream stages produce backpressure.
- Recommended build order from the merged TODO: map/reduce, two scans, merge sort, BFS, local MapReduce, pipeline with backpressure, AllReduce, then fork-join on `deque/`.
- Dependencies: consumes `syncx/` WaitGroup/barriers/semaphores, `queue/`, `scope/`, and `deque/`.
- Career signal: very strong for AI infra and crypto execution because scan/reduce/pipeline/all-reduce map to real production systems.
- Scope rule: every algorithm should state its work, span, partition strategy, synchronization points, cancellation behavior, and whether the operation must be associative or commutative.

## Reference Trail and Go Boundary

- Classic parallel line: Blelloch prefix sums (`https://www.cs.cmu.edu/~guyb/papers/Ble93.pdf`), Cilk/work stealing, Go pipelines (`https://go.dev/blog/pipelines`), and NCCL collectives (`https://developer.nvidia.com/blog/massively-scale-deep-learning-training-nccl-2-4/`).
- Mental model: throughput comes from reducing span and overhead, not just starting more goroutines.
- Go boundary: model worker pools, deques, queues, and cancellation explicitly; do not rely on hidden runtime scheduling behavior for algorithm correctness.
- Algebra boundary: reduce/scan/filter must name associativity, commutativity, stability, and floating-point determinism before promising results.
- Backpressure boundary: pipeline stages need bounded queues and scope cancellation, otherwise "parallel" code becomes a goroutine leak under early exit.
- Interview artifact: every algorithm should include work/span notes and a small adversarial workload that shows scheduler or synchronization overhead.

## Implementation Checklist

- [ ] ParallelMap
  - Core Concept: Split independent element transformations across workers.
  - Pros: Simple first parallel primitive and easy to test.
  - Cons: Scheduling overhead can dominate small inputs.
  - Scenarios: Batch transforms, CPU-bound preprocessing, introduction to worker partitioning.

- [ ] ParallelFor
  - Core Concept: Partition an index range into chunks and run a body function across workers.
  - Pros: Lowest-level data-parallel API and a base for map, reduce, and filter.
  - Cons: Grain-size choice dominates performance.
  - Scenarios: Numeric loops, simulations, vectorized preprocessing, scheduling experiments.

- [ ] WorkerPool
  - Core Concept: A bounded set of workers pulls tasks from a queue and executes them until shutdown.
  - Pros: Practical baseline for limiting concurrency and reusing goroutines.
  - Cons: Queues can hide overload and shutdown semantics are easy to under-specify.
  - Scenarios: Server jobs, CPU task execution, comparison with semaphore-based limiting.

- [ ] ParallelReduce
  - Core Concept: Reduce partitions locally, then combine partial results in a tree.
  - Pros: Teaches associativity and tree-shaped span.
  - Cons: Only valid for associative operations; floating-point results may differ by grouping.
  - Scenarios: Aggregation, metrics, gradient accumulation.

- [ ] ParallelScanHillisSteele
  - Core Concept: Compute prefix results in log rounds by repeatedly combining values at increasing offsets.
  - Pros: Conceptually simple and GPU-friendly.
  - Cons: O(n log n) work, so it is work-inefficient.
  - Scenarios: Prefix sums, offsets, teaching work/span tradeoffs.

- [ ] ParallelScanBlelloch
  - Core Concept: Use upsweep and downsweep phases to compute prefix sums with O(n) work and O(log n) span.
  - Pros: Work-efficient and canonical parallel scan.
  - Cons: More complex than Hillis-Steele and often requires power-of-two padding.
  - Scenarios: Stream compaction, attention offsets, parallel filter.

- [ ] ParallelFilter
  - Core Concept: Mark kept items, scan marks to compute output positions, then scatter.
  - Pros: Demonstrates composition of map, scan, and scatter.
  - Cons: Requires extra memory and stable ordering choices.
  - Scenarios: Query engines, vector processing, GPU-style algorithms.

- [ ] ParallelMergeSort
  - Core Concept: Recursively sort partitions and merge them in parallel.
  - Pros: Good divide-and-conquer example.
  - Cons: Extra memory and merge scheduling complexity.
  - Scenarios: Parallel sorting, database operators, fork/join practice.

- [ ] ParallelBFS
  - Core Concept: Process graph frontiers level by level with a barrier between levels.
  - Pros: Connects algorithms with synchronization phases.
  - Cons: Frontier imbalance and visited-set contention are significant.
  - Scenarios: Graph analytics, dependency traversal, AI infra scheduling graphs.

- [ ] PipelineWithBackpressure
  - Core Concept: Stages communicate through bounded queues and propagate cancellation upstream.
  - Pros: Models real production pipelines.
  - Cons: Shutdown/drain rules are subtle.
  - Scenarios: Feed handler pipelines, inference batching, ETL stages.

- [ ] ForkJoin
  - Core Concept: Recursive tasks fork subtasks and join results, ideally backed by work stealing.
  - Pros: Natural for divide-and-conquer algorithms.
  - Cons: Requires `deque/` to avoid central scheduler bottlenecks.
  - Scenarios: Parallel sort, tree processing, fork/join runtime study.

- [ ] WorkStealingScheduler
  - Core Concept: Workers execute local tasks first and steal from peers when idle.
  - Pros: Balances irregular recursive work while preserving locality.
  - Cons: Correctness depends on `deque/` last-item races, shutdown, and cancellation.
  - Scenarios: Fork/join runtime, graph traversal, recursive parallel algorithms.

- [ ] CompletionService
  - Core Concept: Submit tasks and consume completed results in finish order rather than submission order.
  - Pros: Useful for racing tasks, speculative execution, and streaming partial results.
  - Cons: Requires cancellation, buffering, and error policy decisions.
  - Scenarios: Parallel RPC fan-out, fastest-response wins, search workloads.

- [ ] AllReduceRing
  - Core Concept: Workers exchange chunks around a ring to reduce and distribute results.
  - Pros: Bandwidth-optimal for large tensors.
  - Cons: Latency grows with participant count.
  - Scenarios: Distributed training and NCCL algorithm study.

- [ ] AllReduceTree
  - Core Concept: Reduce up a tree and broadcast down the tree.
  - Pros: Lower latency than ring for small messages.
  - Cons: Bandwidth utilization can be worse and tree imbalance matters.
  - Scenarios: AI infra collectives and barrier/topology comparisons.

- [ ] MapReduceLocal
  - Core Concept: Map records to partitions, shuffle/group locally, then reduce per key.
  - Pros: Teaches data-parallel decomposition and shuffle costs.
  - Cons: Memory-heavy and skew can dominate.
  - Scenarios: Query processing, log aggregation, distributed-system prep.

- [ ] ParallelFrontierEngine
  - Core Concept: Repeatedly process frontier batches, build the next frontier, and synchronize phases.
  - Pros: Generalizes BFS to graph and dependency-propagation workloads.
  - Cons: Load imbalance and duplicate suppression can dominate.
  - Scenarios: Parallel BFS, graph analytics, scheduler dependency expansion.

- [ ] AllReduceRecursiveHalvingDoubling
  - Core Concept: Rabenseifner's algorithm does all-reduce as a recursive-halving reduce-scatter followed by a recursive-doubling all-gather.
  - Pros: Bandwidth- and latency-efficient for power-of-two participant counts, a standard MPI/NCCL approach.
  - Cons: Non-power-of-two sizes need extra steps and the two phases must be tuned by message size.
  - Scenarios: Distributed training collectives, gradient all-reduce, NCCL/MPI algorithm study.

- [ ] AllReduceDoubleBinaryTree
  - Core Concept: Sanders-Speck-Traff overlay two binary trees so each rank is interior in one and a leaf in the other, achieving full bandwidth at logarithmic latency.
  - Pros: Combines tree latency with near-ring bandwidth, used by NCCL at scale.
  - Cons: Tree construction and pipelining are more complex than a single tree or ring.
  - Scenarios: Large-scale training all-reduce, topology-aware collectives, NCCL comparison.

- [ ] ReduceScatterAllGather
  - Core Concept: Decompose all-reduce into a reduce-scatter, where each rank ends with a reduced slice, followed by an all-gather of those slices.
  - Pros: The reusable primitive behind ring and halving-doubling all-reduce, clarifying bandwidth cost.
  - Cons: Requires even chunking and an extra communication phase to reassemble.
  - Scenarios: Collective decomposition teaching, custom all-reduce, sharded gradient reduction.

- [ ] CilkWorkStealingScheduler
  - Core Concept: The Cilk scheduler (Blumofe-Leiserson) runs a work-first policy with per-worker deques and randomized stealing, with provable work/span bounds.
  - Pros: Provably efficient scheduling for fork-join parallelism with good locality.
  - Cons: Correctness depends on the deque last-item protocol and randomized victim selection.
  - Scenarios: Fork-join runtimes, divide-and-conquer parallelism, scheduler bound analysis.

- [ ] BSPSuperstep
  - Core Concept: Bulk-Synchronous Parallel (Valiant) structures computation as supersteps of local compute, communication, then a global barrier.
  - Pros: Simple cost model and deterministic phase structure for parallel and distributed jobs.
  - Cons: The barrier serializes on the slowest worker, so stragglers dominate.
  - Scenarios: Graph processing (Pregel), iterative ML, distributed-algorithm cost modeling.
