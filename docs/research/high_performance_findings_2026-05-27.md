# High-Performance Rust/Go Research Findings — 2026-05-27

This note corrects the roadmap objective: optimize for high-compensation,
high-moat engineering, not ordinary backend role volume.

The auditable source list is:

```text
docs/research/high_performance_source_catalog_2026-05-27.tsv
```

## Scope

Included:

- Rust / Go senior systems engineering.
- HFT, market making, exchange core, market data, crypto derivatives.
- Crypto L1 / validator / MEV / parallel execution.
- FAANG and top-tier infra only when the work is performance, database,
  streaming, network, observability, or runtime infrastructure.
- AI / agent infrastructure only when the work is runtime, inference serving,
  scheduling, GPU/cloud control plane, distributed training, or high-throughput
  orchestration.
- Fintech only when the work is payment core, ledger, idempotency, fraud/risk,
  rate limiting, event streaming, or hot-path reliability.
- Database, storage engine, and streaming system roles.

Excluded:

- CRUD backend.
- Product API work without performance or correctness constraints.
- AI application wrapper work without runtime/platform ownership.
- Admin dashboards and low-scale business logic.

## Main Findings

### 1. UAE is an access point; the moat is global performance depth

VARA and Dubai's VASP market create local access to crypto/exchange employers.
However, the highest-salary moat comes from skills that are globally portable:
lock-free queues, memory ordering, p99 measurement, market-data pipelines,
database/storage internals, streaming engines, and Rust unsafe/atomic competence.

### 2. HFT and exchange core push SPSC/Disruptor above generic backend topics

For low-latency systems, SPSC ring buffers, cache-line padding, Disruptor-style
sequence barriers, deterministic replay, and slow-consumer policies are stronger
portfolio signals than generic LRU/rate-limiter implementations.

### 3. Go is useful, but Rust proof is required for Rust roles

Go implementations prove the concepts and make the repo productive. Rust roles
still need inspectable Rust code: `UnsafeCell`, `Send`/`Sync`, `Acquire`/`Release`,
and clear memory-safety arguments. A small Rust companion beats a large Go-only
claim.

### 4. Database and streaming systems belong in the target set

Scylla/Seastar, Redpanda, TigerBeetle, ClickHouse, RocksDB, TiKV, FoundationDB,
Kafka, Aeron, and Chronicle Queue all reward the same primitives:

- shard-per-core design,
- append-log and group commit,
- bounded queues and backpressure,
- cache-aware data structures,
- deterministic replay,
- p99 and tail-latency measurement.

### 5. AI/agent infrastructure must be filtered hard

High-moat AI work is not "LLM app backend." It is:

- inference serving queues,
- KV-cache memory management,
- continuous batching,
- actor/task runtime,
- backpressure and admission control,
- distributed training runtime,
- NCCL/AllReduce/barrier algorithms,
- GPU/cloud scheduling.

### 6. Fintech is useful only when it touches the hot path

Rate limiting, idempotency, ledgers, payment state machines, risk gates, and
streaming reconciliation are relevant. CRUD fintech workflow services are not.

## Package Priority Changes

Highest priority:

1. `memory/`
2. `queue/lockfree_spsc.go`
3. benchmark/p99 reports
4. Rust SPSC companion
5. Disruptor
6. single-writer matching engine lab
7. database/streaming shard-per-core + append-log lab

Interview-floor but not deepest moat:

1. `syncx/cond.go`
2. bounded blocking queue
3. sharded LRU
4. `ratelimit/`

Deep specialization:

1. `hazard/` + `reclamation/`
2. Michael-Scott queue
3. Rust `Arc<T>` from scratch
4. Treiber + `crossbeam-epoch`
5. advanced barriers + toy AllReduce for AI training infra

Lower priority unless targeting a specific niche:

1. CRDT suite
2. full STM / Block-STM
3. complete RCU
4. actor framework beyond minimal runtime
5. extra lock variants after the core benchmark story is complete

## Resulting Strategy

The repo should be framed as:

> A Rust/Go high-performance systems forge for learning and demonstrating memory
> ordering, cache behavior, queue design, p99 latency, backpressure, and
> correctness in exchange, streaming, database, and AI-runtime systems.

The first 12 weeks should produce artifacts a performance interviewer can inspect:

- code,
- tests,
- benchmarks,
- p99 report,
- Rust unsafe/atomic proof,
- exchange or streaming demo,
- database-style replay/log demo.
