# Queue Family TODO

> Category 抽象: FIFO ordered transfer。Producer 入 tail,consumer 出 head,順序保留。
> Underlying logic: head/tail 上的協調 + concurrency profile 作為 type-level constraint。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **B(3mo·deep showcase)★ portfolio 主力** | **3.80** | 9.5/10 | 1.0 `crossbeam-queue` | 2.5 週 | T0 |

> 最高 V;Bybit 撮合 + Coinbase LMAX。補純 SPSC(rigtorp)+ Michael-Scott + benchmark。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- `enqueue(v)` happens-before `dequeue(v)` — FIFO 在 happens-before 關係下保留
- Concurrency profile (SPSC / MPSC / SPMC / MPMC) 是 *type-level* 限制,不是 runtime 屬性
- Bounded vs unbounded 決定背壓策略 (block / return false / grow)
- Profile 退化 → 可以省 atomic / fence (從 seq_cst 到 release/acquire)

## Inventory

| Variant | Profile | Bound | Impl | Status |
|---|---|---|---|---|
| MutexMPMC | MPMC | unbounded | linked + mutex | done |
| MutexMPSC | MPSC | unbounded | linked + mutex | done |
| LockFreeMPMC | MPMC | bounded ring | seq-based CAS | done |
| LockFreeMPSC | MPSC | bounded ring | producer CAS, consumer 獨佔 | done |
| LockFreePaddedMPMC | MPMC | bounded ring | + cache-line padding | done |
| **LockFreeSPSC (Rigtorp)** | SPSC | bounded ring | cached head/tail snapshot | WIP (lockfree_spsc.go 已建立) |
| **LMAX Disruptor** | 1P/NC + barrier | bounded ring | seq + dependency graph | TODO |
| **Michael-Scott unbounded MPMC** | MPMC | unbounded | linked + 2 CAS + helping | TODO |
| Vyukov intrusive MPSC | MPSC | unbounded | linked, 1 XCHG per enq | TODO |
| Wait-free Queue (Kogan-Petrank) | MPMC | unbounded | announcement + helping | TODO |
| Priority Queue (lock-free skip-list) | MPMC | unbounded | Sundell-Tsigas | TODO |

## Variant contracts (TODO 部分)

### LockFreeSPSC (Rigtorp)

**Signature:**
```
type LockFreeSPSC[T] struct {
    head        atomic.Uint64   // 只有 producer 寫 (consumer 也讀)
    cachedTail  uint64          // producer-local snapshot (non-atomic)
    _pad1       cacheLinePad
    tail        atomic.Uint64   // 只有 consumer 寫 (producer 也讀)
    cachedHead  uint64          // consumer-local snapshot (non-atomic)
    _pad2       cacheLinePad
    buf         []T
    mask        uint64
}
```

**Contract:**
- Exactly ONE producer, ONE consumer goroutine。違反前提 = data race
- Enqueue 路徑: 先讀 cachedTail。若 cachedTail 顯示「滿」才 atomic load 真實 tail 更新 cache
- Dequeue 路徑對稱
- Memory ordering: producer 對 head 是 release-store, consumer 對 head 是 acquire-load (反之亦然)

**Key insight:** cached snapshot 把 cross-core load 從每次操作降到 ~1/N 次 — 這是 Rigtorp 跟 LMAX 共同的核心優化。減少 *false coherence traffic*。

**Reference:** Erik Rigtorp "Optimizing a Ring Buffer for Throughput", 362K ops/ms, 133ns p50。boost::lockfree::spsc_queue, folly::ProducerConsumerQueue。

### LMAX Disruptor

**Signature:**
```
type Disruptor[T] struct {
    ring     []T
    cursor   atomic.Uint64           // producer claim sequence
    barriers []*ConsumerBarrier      // 各 consumer group
}
type ConsumerBarrier struct {
    seq  atomic.Uint64
    deps []*ConsumerBarrier  // 必須等的上游 consumer
}
```

**Contract:**
- Producer `Next()` → claim seq → 寫 ring[seq & mask] → publish via cursor.Store
- Consumer 等 `cursor >= mySeq && all dep.seq >= mySeq`
- Consumer **不消費** — slot 由 producer wraparound 覆寫

**Key insight:** consumer 不擁有「取出」這個 op,ring 是 broadcast 不是 transfer。dependency graph 讓多 consumer pipeline (例如 journal → replicate → match) 共用同一個 ring。

**Reference:** LMAX 2011 paper "Disruptor: High performance alternative to bounded queues",Martin Thompson Mechanical Sympathy blog。Coinbase market data 公開引用。

### Michael-Scott Unbounded MPMC

**Signature:**
```
type MSQueue[T] struct {
    head atomic.Pointer[msNode[T]]   // 永遠指 sentinel
    tail atomic.Pointer[msNode[T]]
}
type msNode[T] struct {
    val  T
    next atomic.Pointer[msNode[T]]
}
```

**Contract:**
- Enqueue 兩步: (1) CAS tail.next nil → newNode (2) CAS tail → newNode (可以由任何人代勞)
- Dequeue: CAS head 前進
- **Note:** 需要 `reclamation/` (HP 或 EBR) 才能安全 free 舊節點 — 否則 ABA + UAF

**Key insight:** "helping" 是 lock-free 演算法的核心 — 看到 tail.next != nil 但 tail 沒前進,自己幫前進。沒有單一 thread 的進度依賴。

**Reference:** Michael & Scott 1996 "Simple, Fast, and Practical Non-Blocking and Blocking Concurrent Queue Algorithms"。Java ConcurrentLinkedQueue 基於此。

### Vyukov Intrusive MPSC

**Contract:**
- Producer 一次 atomic XCHG 就完成 enqueue
- Consumer 單執行緒,可暫時看到 "stub" (新 producer 還沒 link 完 next)
- 比 M&S 簡單且快,代價是 *intrusive* (節點 owner 是 caller,不是 queue)

**Reference:** Dmitry Vyukov 1024cores blog "Non-intrusive MPSC node-based queue"。

### Wait-free Queue (Kogan-Petrank 2011)

**Contract:**
- 每個 op 在有限步驟內完成 (與其他 thread 行為無關)
- announcement array + helping protocol
- 學術價值 >> 實用價值 (constant factor 大)

### Priority Queue (Lock-free skip-list)

**Contract:**
- DeleteMin 為主要 op
- Sundell-Tsigas 2003 lock-free skip-list
- 需要 reclamation

## Career signal

- **HFT:** SPSC + LMAX 直接命中。LMAX paper 是 HFT staple
- **AI infra:** batch scheduler 用 SPSC / MPSC,pipeline 用 Disruptor 風格
- **Crypto CEX:** market data plane = LMAX style (Coinbase 公開引用)
- **面試 high-frequency:** "implement SPSC with correct memory ordering"

## Recommended order

1. LockFreeSPSC (你已開檔)
2. LMAX Disruptor (寫完就有 blog 素材)
3. Michael-Scott (要先有 hazard pointer)
4. Vyukov MPSC (cheap win,intrusive 風格教學)
5. Wait-free / Priority Queue (純學術,選做)

## Dependencies

- M&S queue → `reclamation/` (HP 或 EBR)
- LMAX Disruptor 不需要 reclamation (ring 自然覆寫)
- 全部 lock-free → `memory/` (release/acquire fence 模式)

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 0(Composite ★4.8)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★5/5 | Required | Headlands: "asked to implement a lock-free SPSC queue in C++" (confirmed Glassdoor); quantdev.blog SPSC with acquire/release + cache-line alignas() |
| **Crypto** | ★5/5 | Required (CEX) | Coinbase: LMAX ring buffer for market data — "10x fewer allocations, 24x faster than channels"; Jito bundle ingestion queue |
| **AI Infra** | ★5/5 | Required | vLLM dual-queue scheduler (waiting + running); lock-free MPMC for CUDA stream; SPSC ring buffer for GPU-CPU data transfer |
| **FAANG** | ★5/5 | Required | Google/Meta: "thread-safe bounded blocking queue" = canonical coding question; MPSC maps to Kafka partition model conceptually |
| **Dubai** | ★4/5 | Required | Bybit/OKX matching engine + market data use lock-free queue; Coinbase LMAX pattern cited |
| **Composite** | **★4.8/5.0** | **Tier 0** | — |

### 必要(Required for senior infra interviews)

> 在 **≥2 個 vertical** 被列為 Required,或 composite ≥ 3.4。

- **SPSC ring buffer (Rigtorp style)** — 最高 ROI 單一 item;head/tail 必須在不同 cache line 是面試分水嶺
  - Evidence: [hackerprep.io/company/headlands](https://hackerprep.io/company/headlands) — "implement lock-free SPSC queue" confirmed; [quantdev.blog](https://quantdev.blog/posts/spsc_lockfree_queue/index.html) — cache padding analysis
- **MPSC queue** — Michael-Scott 或 padded variant;producer CAS + exclusive consumer
  - Evidence: [BagritsevichStepan/lock-free-data-structures](https://github.com/BagritsevichStepan/lock-free-data-structures) — "used in HFT to share data between market data receiver and strategies"
- **MPMC queue (Vyukov bounded ring)** — seq-based CAS;LockFreePaddedMPMC 是 production 版
  - Evidence: [rigtorp/MPMCQueue](https://github.com/rigtorp/awesome-lockfree) — reference MPMC with padding; Google/Meta "thread-safe bounded blocking queue" coding question

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator。

- **Padded MPMC with Disruptor wait strategy** — Disruptor 的 wait strategy (BusySpin/Yield/Blocking) 是 performance tuning 的最後一層
  - Best for: HFT / Crypto CEX — [MyntBit](https://www.myntbit.com/training/disruptor-cursor-barrier) "Disruptor SPSC rated as hard quant interview question at Jane Street, Citadel, Two Sigma"
- **LMAX Disruptor (1P/NC + barrier)** — dependency graph consumer pipeline;broadcast 不是 transfer
  - Best for: HFT — [lmax-exchange.github.io/disruptor](https://lmax-exchange.github.io/disruptor/disruptor.html) — "3 orders of magnitude lower latency"; Coinbase production usage
- **Michael-Scott unbounded MPMC (with helping)** — lock-free correctness + ABA hazard
  - Best for: FAANG (Java `ConcurrentLinkedQueue` 基於 M&S; correctness reasoning signal)

### Recommended Order(本 package 內部)

1. LockFreeSPSC Rigtorp(cache padding + acquire/release)
2. LMAX Disruptor(blog 素材)
3. Michael-Scott MPMC(接 hazard pointer)
4. Vyukov Intrusive MPSC(cheap win)
5. Priority Queue / Wait-free(純學術,選做)

### 對應的 Blog 題材(若想寫)

- "Go SPSC ring buffer:從 channel 到 lock-free,benchmark 對比(throughput + p99 + cache miss)"
- "LMAX Disruptor in Go:為什麼 Coinbase 說比 channel 快 24x"
