# parallel — Parallel Algorithms

> **訓練主題**:**「分解」問題的能力**。原語會用是一回事,把問題切成可並行的單位是另一回事。
>
> **為什麼獨立 package**:
> - 跟 syncx primitive 正交 — parallel 是**演算法**,不是 primitive
> - 每個演算法都有獨立的 work / span 分析,值得獨立檔案
> - 對應 PRAM / CREW 等理論模型,自成領域

---

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **E(6mo·容器/並行)** | **2.38** | 7.5/10 | 0.95 `rayon` | 3.0 週 | T0 |

> `rayon` 對應;G42 AI 橋接 + crypto parallel exec。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心概念

```
Sequential:   for i in 0..n: a[i] = f(a[i])           O(n) work, O(n) span
Parallel:     for_all_parallel i in 0..n: ...         O(n) work, O(1) span
              (limited by # cores: O(n/p) actual)

關鍵指標:
  work (W)  — 所有 op 加總(等於 sequential time)
  span (S)  — critical path,無限核也要這麼久
  parallelism (W/S) — 理論最大並行度
```

---

## Inventory

### A. 基礎 building block

| 名稱 | Work | Span | 一句話 |
|------|------|------|--------|
| **Parallel Map** | O(n) | O(1) | 每元素獨立 |
| **Parallel Reduce** | O(n) | O(log n) | Tree reduce,需 associative op |
| **Parallel Scan / Prefix Sum** | O(n) | O(log n) | Hillis-Steele(work-inefficient)或 Blelloch(work-efficient) |
| **Parallel Filter** | O(n) | O(log n) | 用 scan 計算 output index |

### B. 排序 / 搜尋

| 名稱 | Work | Span | 一句話 |
|------|------|------|--------|
| **Parallel Merge Sort** | O(n log n) | O(log² n) | divide & conquer,merge 階段也可平行 |
| **Sample Sort** | O(n log n) | O(log n) | 抽 sample 決定 bucket,適合 distributed |
| **Bitonic Sort** | O(n log² n) | O(log² n) | GPU friendly,oblivious algorithm |
| **Parallel Quicksort** | O(n log n) avg | O(log² n) avg | partition 也可平行 |

### C. 圖演算法

| 名稱 | Work | Span | 一句話 |
|------|------|------|--------|
| **Parallel BFS** | O(V+E) | O(D log V) | frontier-based,每層 barrier |
| **Parallel SSSP (delta-stepping)** | O(V+E) | O(D · max_delta) | Dijkstra 的並行近親 |
| **Parallel Connected Components** | O(V+E) | O(log V) | union-find with concurrent merge |

### D. 高層次組合

| 名稱 | 一句話 |
|------|--------|
| **Map-Reduce** | shuffle 階段的 partition + sort,單機版練 |
| **Pipeline with Backpressure** | stage1 → stage2 → stage3,每段 worker pool 獨立 |
| **Fork-Join Framework** | recursive task spawn,接 work-stealing deque |
| **Bulk Synchronous Parallel (BSP)** | compute → barrier → exchange,迭代式 |

---

## 建議實作順序

```
1. Parallel Map / Reduce       ← 暖身,需要 syncx/WaitGroup
2. Parallel Scan(兩個版本)    ← Hillis-Steele vs Blelloch,benchmark 對比
3. Parallel Merge Sort         ← divide & conquer 第一題
4. Parallel BFS                ← 接 syncx/Barrier(每層 sync)
5. Map-Reduce(in-process)     ← shuffle 怎麼做
6. Pipeline + Backpressure     ← 接 scope/ 做 cancel
7. Fork-Join                   ← 接 deque/ work-stealing
8. (進階) Sample Sort / 平行圖演算法
```

---

## 設計重點

### Parallel Scan 為什麼有兩個版本?
- **Hillis-Steele**:O(n log n) work,O(log n) span — work-inefficient 但 GPU 親和
- **Blelloch**:O(n) work,O(log n) span — work-efficient,up-sweep + down-sweep 兩階段
- **教學價值**:看到「work 跟 span 的 trade-off」

### Pipeline backpressure
- Bounded queue 配 `Semaphore`(credit-based)或 channel(blocking)
- **gotcha**:cancel 從下游往上傳,所有 stage 都要 respond,接 `scope/`

### Fork-Join 為什麼難?
- 遞迴 spawn,worker 數動態變化
- 需要 work-stealing deque(從別人那偷任務)
- Cilk / Java ForkJoinPool / Go runtime 都是這架構

---

## Dependencies

- → `syncx/`(WaitGroup, Barrier, Semaphore)
- → `scope/`(cancellation propagation)
- → `deque/`(Fork-Join 需要 work-stealing)
- → `queue/`(Pipeline 的 stage 間 buffer)

---

## Career signal

- **AI infra**:training 是大量 reduce / scan / all-reduce,對應 NCCL collective ops
- **Databases**:平行 query execution(Spark / Presto / DuckDB)就是 parallel relational algebra
- **GPU programming**:CUDA 的 thrust / cub 庫就是這套 primitive

---

## 參考資料

- Blelloch 1996 *"Programming Parallel Algorithms"* — 教科書級
- Cilk paper(Blumofe & Leiserson 1995)— work-stealing 的源頭
- CMU 15-210 *Parallel and Sequential Data Structures and Algorithms* 課程

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 0(Composite ★4.0)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★3/5 | Required (pipeline) | HFT pipeline: feed handler → normalizer → strategy → risk → OMS = parallel pipeline; work-stealing for parallel backtesting at Two Sigma |
| **Crypto** | ★5/5 | Required at L1 | Block-STM IS parallel execution + conflict detection; Firedancer tile pipeline = pipeline parallel; Monad pipelined consensus + async execution |
| **AI Infra** | ★5/5 | Required | AllReduce = THE defining operation in DDP/FSDP gradient sync; parallel scan = cumulative attention mask offsets for continuous batching |
| **FAANG** | ★4/5 | Required | Parallel map/filter/reduce tested at Google/Amazon/Databricks; `errgroup` for parallel work = senior Go coding question |
| **Dubai** | ★3/5 | Required | G42 AI training infra; Bybit/Binance parallel order processing |
| **Composite** | **★4.0/5.0** | **Tier 0** | — |

### 必要(Required for senior infra interviews)

> 在 **≥2 個 vertical** 被列為 Required,或 composite ≥ 3.4。

- **Parallel Map / Reduce** — 基礎;WaitGroup-based;DDP gradient sync 的概念對應
  - Evidence: [FAANG research](../docs/research/faang.md) — "parallel map/filter/reduce tested at Google, Amazon, Databricks"; errgroup = senior Go pattern
- **Parallel Scan (Hillis-Steele + Blelloch)** — prefix sum;兩版本 work/span 對比;GPU CUDA thrust/cub
  - Evidence: [AI Infra research](../docs/research/ai_infra.md) — "Parallel Scan = computing cumulative attention masks, sequence length offsets for continuous batching"
- **Parallel Pipeline with Backpressure** — bounded channel + cancellation;每 stage 獨立 worker pool
  - Evidence: Crypto — Firedancer tile pipeline (each tile = pipeline stage); HFT: feed handler → strategy → OMS pipeline

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator — 特別是 AI Infra。

- **AllReduce 三種拓樸 (Ring / Tree / Dissemination)** — NCCL 的三種演算法;signal 極高 at AI Infra
  - Best for: AI Infra — [NCCL developer blog](https://developer.nvidia.com/blog/fast-multi-gpu-collectives-nccl/) — ring = bandwidth-optimal, tree = latency-optimal; Anthropic/OpenAI 面試直擊
- **Parallel BFS** — frontier-based;每層 barrier 接 syncx/Barrier
  - Best for: AI Infra (Pathways dependency graph BFS); FAANG (Google parallel graph query)
- **Fork-Join Framework** — recursive spawn;接 deque/ work-stealing;Java ForkJoinPool = Databricks Spark executor
  - Best for: FAANG (Databricks Spark analogy); AI Infra (distributed training task scheduler)

### Recommended Order(本 package 內部)

1. Parallel Map / Reduce(暖身)
2. Parallel Scan 兩版本(Hillis-Steele vs Blelloch)
3. Parallel Merge Sort
4. Parallel BFS(接 syncx/Barrier)
5. Map-Reduce in-process
6. Pipeline + Backpressure(接 scope/)
7. AllReduce 三種拓樸(AI Infra signal)
8. Fork-Join(接 deque/)

### 對應的 Blog 題材(若想寫)

- "AllReduce 在 Go:ring / tree / dissemination 三種拓樸實作與 NCCL 對應"
- "Parallel Scan:Hillis-Steele vs Blelloch work-span 分析 + Go benchmark"
