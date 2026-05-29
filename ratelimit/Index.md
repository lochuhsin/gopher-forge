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

## Reference Trail and Go Boundary

- Primary Go references: Go rate-limiting wiki (`https://go.dev/wiki/RateLimiting`) and `x/time/rate` token bucket (`https://pkg.go.dev/golang.org/x/time/rate`).
- Classic policy line: token bucket, leaky bucket, sliding windows, GCRA, circuit breakers, bulkheads, and credit-based flow control.
- Mental model: admission control is a state machine over time plus capacity. The key question is whether excess work waits, fails, drops, or replaces older work.
- Go boundary: local limiters can use atomics, mutexes, semaphores, and injected clocks. Distributed limiters are a separate consistency problem because the atomic update crosses process and network boundaries.
- Time boundary: use `clock.ManualClock`/`DeadlineHeap` for tests; wall-clock sleeps hide bugs and make benchmarks noisy.
- Interview artifact: every limiter should document burst behavior, fairness, cancellation, clock skew assumptions, and overload policy.

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
