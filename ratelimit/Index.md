# ratelimit Index

> Learning goal: control producer speed, isolate failures, and apply concurrency primitives to service-level flow control policies.

Status legend: `[x]` implemented, `[~]` scaffold or partial implementation, `[ ]` planned.

## Package Notes

- Core question: what happens when producers are faster than consumers? The answer can be allow burst, smooth, block, drop, shed, or isolate.
- Rate limiters are policy-layer concurrency, not low-level primitives, but their correctness still depends on atomic state and time.
- Distributed limiting is much harder than local limiting because the atomic update moves to Redis, etcd, or another shared authority.
- Recommended build order from the merged TODO: token bucket, leaky bucket, sliding window counter, sliding window log, GCRA, circuit breaker, bulkhead, Hystrix-style breaker, credit-based backpressure.
- Dependencies: uses `syncx.Semaphore` for bulkheads, `scope/` for deadlines/cancellation, and atomics from the standard library.
- Career signal: top-tier for Dubai/fintech/backend interviews because distributed rate limiting and circuit breakers are common system-design topics.
- Scope rule: include service-level flow-control patterns when they are implemented with portable primitives: atomics, semaphores, queues, clocks, and cancellation.

## Implementation Checklist

- [ ] TokenBucket
  - Core Concept: Tokens refill over time up to a capacity; each request consumes tokens and bursts are allowed.
  - Pros: Most common limiter and supports controlled bursts.
  - Cons: Atomic update of `(tokens, lastRefill)` is subtle under concurrency.
  - Scenarios: API gateways, per-user quotas, exchange request limits.

- [ ] LeakyBucket
  - Core Concept: Requests drain at a fixed rate, smoothing output.
  - Pros: Produces stable throughput and is easy to reason about.
  - Cons: Burst handling is worse than token bucket and queues can grow.
  - Scenarios: Batch admission, steady outbound calls, load smoothing.

- [ ] FixedWindowCounter
  - Core Concept: Count requests in discrete time windows and reset at boundaries.
  - Pros: Very simple and constant memory.
  - Cons: Allows boundary bursts up to nearly twice the intended rate.
  - Scenarios: Teaching baseline and approximate low-value limits.

- [ ] SlidingWindowLog
  - Core Concept: Store request timestamps and count only those inside the moving window.
  - Pros: Exact rate limiting.
  - Cons: O(requests in window) memory per key.
  - Scenarios: Strict compliance limits and accuracy comparison.

- [ ] SlidingWindowCounter
  - Core Concept: Combine current and previous fixed windows with time-based weighting.
  - Pros: O(1) memory with smoother behavior than fixed windows.
  - Cons: Approximate and can still misestimate burst edges.
  - Scenarios: Practical per-key rate limits.

- [ ] GCRA
  - Core Concept: Track theoretical arrival time to implement token-bucket-equivalent limits in constant space.
  - Pros: Compact and production-proven.
  - Cons: Less intuitive than token bucket for beginners.
  - Scenarios: Cloudflare-style rate limiting and telecom traffic shaping.

- [ ] CircuitBreaker
  - Core Concept: A state machine moves Closed -> Open -> Half-Open based on failure/latency signals.
  - Pros: Prevents cascading failures and gives dependencies recovery time.
  - Cons: Thresholds are workload-sensitive and can flap.
  - Scenarios: Payments, downstream RPCs, exchange kill-switch style controls.

- [ ] LoadShedding
  - Core Concept: Reject or degrade requests when queue depth, latency, deadline, or error budget crosses a threshold.
  - Pros: Keeps overloaded systems responsive instead of letting queues grow without bound.
  - Cons: Poor policies can reject useful work or amplify traffic retries.
  - Scenarios: API gateways, inference serving, exchange overload controls.

- [ ] Bulkhead
  - Core Concept: Partition capacity by caller, dependency, or request class so one failure domain cannot consume all resources.
  - Pros: Strong isolation and direct use of semaphores.
  - Cons: Capacity fragmentation can leave resources idle.
  - Scenarios: VIP vs normal traffic, service pools, GPU memory admission.

- [ ] AdaptiveConcurrencyLimiter
  - Core Concept: Adjust allowed in-flight work based on observed latency, queueing, or success rate.
  - Pros: Responds to changing downstream capacity without fixed limits.
  - Cons: Control loops can oscillate and require careful measurement.
  - Scenarios: RPC clients, database pools, inference service admission.

- [ ] PriorityLimiter
  - Core Concept: Admit or shed work by priority class while preserving per-class limits.
  - Pros: Protects critical traffic under overload.
  - Cons: Starvation and priority inversion must be handled explicitly.
  - Scenarios: VIP exchange traffic, control-plane requests, graceful degradation.

- [ ] DistributedLimiter
  - Core Concept: Coordinate limits across processes using a shared atomic store such as Redis Lua.
  - Pros: Solves multi-instance quota enforcement.
  - Cons: Adds network latency, clock consistency issues, and store availability risk.
  - Scenarios: Global API limits, exchange/account quotas, multi-region systems.

- [ ] CreditBasedBackpressure
  - Core Concept: Consumers grant credits; producers can only send while holding credits.
  - Pros: Prevents unbounded queues and models reactive streams.
  - Cons: Credit accounting and recovery after failure are tricky.
  - Scenarios: Market-data fan-out, pipeline stages, network flow control.

- [ ] QueueDepthBackpressure
  - Core Concept: Use bounded queue occupancy to slow, block, or reject producers.
  - Pros: Simple and directly observable.
  - Cons: Queue depth is a lagging signal and does not capture downstream latency alone.
  - Scenarios: Pipeline stages, actor mailboxes, worker pools.

- [ ] CoDel
  - Core Concept: Controlled Delay (Nichols-Jacobson 2012) is a parameterless AQM that tracks the minimum queue sojourn time over an interval and sheds when a standing queue persists past a target.
  - Pros: Self-tuning ("no knobs") backpressure that targets latency, not just queue length.
  - Cons: Tracks time-in-queue not depth, so it needs per-item enqueue timestamps.
  - Scenarios: Server request queues, latency-based load shedding, bufferbloat-style overload control.

- [ ] AdaptiveLIFO
  - Core Concept: Serve FIFO under normal load but switch to LIFO once a queue forms so fresh, un-timed-out requests run first (Facebook, paired with CoDel).
  - Pros: Maximizes the fraction of requests served within deadline during overload.
  - Cons: Reorders work and can starve old requests, so it pairs with a shedding policy.
  - Scenarios: Overload latency control, deadline-aware serving, API gateway protection.

- [ ] GradientConcurrencyLimiter
  - Core Concept: Netflix Gradient limiters set newLimit = limit x (RTTnoload/RTTactual) + queue, shrinking the concurrency limit as latency rises.
  - Pros: Adapts to changing downstream capacity using only latency, with no fixed RPS to go stale.
  - Cons: Needs a reliable no-load RTT baseline and can oscillate without smoothing (Gradient2).
  - Scenarios: RPC client admission, autoscaling backends, inference gateway concurrency.

- [ ] VegasConcurrencyLimiter
  - Core Concept: A TCP-Vegas-style limiter estimates queued work as L x (1 - minRTT/sampleRTT) and nudges the limit up or down around alpha/beta thresholds.
  - Pros: Delay-based, smooth limit adjustment that detects queuing before loss.
  - Cons: Sensitive to minRTT estimation and clock noise.
  - Scenarios: Adaptive concurrency limits, latency-sensitive admission, concurrency-limits study.

- [ ] LittlesLawConcurrency
  - Core Concept: Little's Law (L = arrival rate x latency) ties a concurrency limit to observed RPS and latency for capacity reasoning.
  - Pros: Gives a principled way to size and validate concurrency limits.
  - Cons: Assumes a stable system in steady state, which bursty traffic violates.
  - Scenarios: Capacity planning, limit validation, queueing-theory grounding for limiters.

- [ ] DeficitRoundRobin
  - Core Concept: DRR (Shreedhar-Varghese 1995) gives each flow a per-round quantum plus a carried deficit, serving up to quantum+deficit bytes in O(1).
  - Pros: O(1) fair queueing across flows with variable item sizes.
  - Cons: Quantum choice trades latency against fairness granularity.
  - Scenarios: Per-tenant fair scheduling, multi-flow shaping, weighted-share serving.

- [ ] WeightedFairQueueing
  - Core Concept: WFQ approximates generalized processor sharing by assigning virtual finish times so flows get bandwidth proportional to weights.
  - Pros: Strong fairness and weighted isolation between competing flows.
  - Cons: Virtual-time bookkeeping is O(log n) per item, heavier than DRR.
  - Scenarios: QoS scheduling, weighted tenant isolation, fair-queueing comparison.

- [ ] HierarchicalTokenBucket
  - Core Concept: HTB arranges token buckets in a class hierarchy where idle parents lend spare rate to busy children within ceilings.
  - Pros: Expressive classful shaping with borrowing, the model behind Linux tc HTB.
  - Cons: Class-tree configuration and borrow accounting are complex to tune.
  - Scenarios: Multi-class bandwidth shaping, tenant hierarchies, egress rate control.
