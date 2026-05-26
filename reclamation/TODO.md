# Memory Reclamation Family TODO

> Category 抽象: Lock-free DS unlink 後「何時可以 free」的問題。
> **Building-block category** — 不被 application 直接使用,被其他 user-facing category 消費。
> Underlying logic: 必須證明「沒有任何 thread 還持有對該節點的指標」。三條證明路徑。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **D(6mo·crossbeam cluster)★** | **2.50** | 7.5/10 | 1.0 `crossbeam-epoch` | 3.0(含hazard) 週 | T2 |

> EBR/QSBR,跟 hazard/ 配對做,crossbeam-epoch 對應。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

| 證明路徑 | 機制 | 誰付成本 |
|---|---|---|
| Reader 聲明 | HP — reader 寫 hazard slot | Reader (fence per deref) |
| Reader pin epoch | EBR — reader 標記 epoch | Reader (兩次 load) |
| Reader 主動 quiescent | QSBR — reader 在 safe point 聲明 | Reader (0 in critical) |
| Pointer 帶 refcount | DRC | Reader (atomic add) |

## Inventory

| Variant | Reader cost | Memory bound | Status | Location |
|---|---|---|---|---|
| **Hazard Pointer (HP)** | store + fence + reload | bounded by HP slots | WIP | `hazard/` |
| **Epoch-Based Reclamation (EBR)** | 2 loads per critical section | unbounded if stuck thread | TODO | `reclamation/ebr.go` |
| **QSBR (quiescent-state)** | 0 in critical section, requires safe points | bounded if quiescence | TODO | `reclamation/qsbr.go` |
| **DRC (Deferred Ref Counting)** | atomic add per deref | bounded by refcount | TODO | `reclamation/drc.go` |
| Interval-Based Reclamation (IBR) | HP + EBR hybrid | bounded | TODO (進階) | `reclamation/ibr.go` |

## Variant contracts (TODO)

### EBR (Epoch-Based Reclamation)

**Signature:**
```
type EBR struct {
    globalEpoch atomic.Uint64
    threads     atomic.Pointer[threadList]
    limbo       [3][]retiredItem  // 3 bins, indexed by epoch mod 3
}
type threadEntry struct {
    localEpoch atomic.Uint64  // 0 = inactive; otherwise = epoch when pinned
    next       *threadEntry
}

type Guard struct { ebr *EBR; entry *threadEntry }
func (e *EBR) Pin() *Guard
func (g *Guard) Defer[T any](p *T, deleter func(*T))
func (g *Guard) Unpin()
func (e *EBR) TryAdvance()  // 全部 thread localEpoch >= global → global++
```

**Contract:**
- Pin: `e = globalEpoch.Load(); localEpoch[me].Store(e)` (acquire-release)
- 操作 lock-free DS → Defer 舊節點 到 limbo[e % 3]
- Unpin: localEpoch[me].Store(0)
- TryAdvance: 確認所有 pinned thread 的 localEpoch >= globalEpoch → global epoch++ → 兩代之前的 limbo 全部 free

**Key insight:** "epoch 翻過 2 代" 等於 "所有 reader 都至少有一次機會看到並 pin 過新 epoch" → 舊 epoch retire 的東西安全。比 HP 便宜的原因:reader fast path 只有兩個 load (而不是 store+fence)。

**Reference:** Keir Fraser PhD 2003 "Practical lock-freedom"。Crossbeam-epoch (Rust) 是業界 reference 實作。

### QSBR (Quiescent-State Based Reclamation)

**Signature:**
```
type QSBR struct {
    globalCount atomic.Uint64
    threads     []*qsbrThread
}
type qsbrThread struct {
    seenCount atomic.Uint64  // last quiescent observed count
}
func (q *QSBR) Defer[T any](p *T, deleter func(*T))
func (q *QSBR) QuiescentState()  // reader 在 safe point 呼叫
func (q *QSBR) Synchronize()     // writer 等所有 thread quiescent 過一輪
```

**Contract:**
- Reader 在 critical section 內 **完全沒有 atomic** (zero cost)
- Reader 在 safe point 主動呼叫 `QuiescentState()` 公佈 seenCount = globalCount
- Writer 等所有 thread `seenCount >= writerSnapshot` 後 reclaim

**Key insight:** 把 cost 從 reader fast path 轉嫁到 reader safe point。對 *long-lived workload* 最快;對 *short critical section* 反而麻煩 (要找 safe point)。

**Reference:** Linux kernel RCU 的 QSBR 變體。userspace-rcu (URCU) library。Hart, McKenney 等 2007 "Performance of memory reclamation for lockless synchronization"。

### DRC (Deferred Reference Counting)

**Signature:**
```
type DRC[T] struct {
    ref atomic.Int64  // split refcount avoid single cache line contention
}
func Deref[T any](slot *atomic.Pointer[drcCell[T]]) *T  // atomic inc + load
func Release[T any](cell *drcCell[T])                    // atomic dec; if 0, free
```

**Contract:**
- 每節點帶 atomic refcount
- 讀指標 = atomic inc + load + (released) dec
- Split refcount (Anderson 2021) 避免單一 cache line 競爭:把 refcount 拆成 per-cpu/per-thread bucket,sum on free check

**Key insight:** 跟 `shared_ptr` 概念一樣,但 lock-free shared_ptr 本身也是難題 (DWCAS 或 split refcount)。優點:跟現有 RC 系統互通 (e.g. Rust Arc)。

**Reference:** Anderson, Laorden, Sun 2021 "Concurrent Deferred Reference Counting with Constant-Time Overhead", PLDI。

### IBR (Interval-Based Reclamation)

**Contract:**
- 結合 HP + EBR 的優點
- Reader 標 interval (start, end);writer retire 時記錄 interval
- 只 free 不在任何 active interval 內的 retired

**Reference:** Wen, Izraelevitz, Cai 2018 "Interval-Based Memory Reclamation"。

## Variant 選擇指南

| Workload | 推薦 |
|---|---|
| Lock-free queue/stack,short critical section | HP |
| Lock-free hash map, mixed read/write | EBR |
| Read-mostly,長 critical section,可 schedule quiescent | QSBR / RCU |
| 需要跟 `shared_ptr` 互通 | DRC |
| 想要 best-of-both | IBR (進階) |

## 跨語言對照

| 語言/系統 | Reclamation |
|---|---|
| C++26 | `std::hazard_pointer`, `std::rcu_*` (進標準) |
| Rust | `crossbeam-epoch` (EBR), `haphazard` crate (HP) |
| Java | GC 直接處理 (`LongAdder` 內部有類似 logic) |
| Linux kernel | RCU (QSBR variant) |
| Go | 沒有原生 (但 GC 跟 lock-free 互動仍有問題 — 例如 unsafe pointer cast) |

## Career signal

- 補上 HP 讓 `queue/`, `stack/` 從「玩具」升級成 production-grade
- Citadel / Jane Street / HRT 面試: "your lock-free X under stress, what fails?" — 答案 = ABA + UAF,解法 = 這個 family
- AI infra: PyTorch CUDA caching allocator 也是 reclamation 問題的變體 (defer free GPU memory)
- C++26 把 HP / RCU 提到標準 → 表示業界共識「lock-free DS 必須配 reclamation」

## Recommended order

1. **HP** (在 `hazard/`,已開檔) — 對接 Treiber stack
2. **EBR** (在 `reclamation/ebr.go`) — 對接 M&S queue
3. QSBR (與 RCU 共生)
4. DRC, IBR (選做,paper-level)

## Dependencies

- → `memory/` (fence patterns)
- 被 `queue/`, `stack/`, `rcu/` (內部 reclamation), `map/` (lock-free 版本) 消費

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 2(Composite ★3.0)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★3/5 | Advanced | ABA + safe reclamation = senior HFT differentiator; Crossbeam-epoch (Rust) = Two Sigma Rust ecosystem signal |
| **Crypto** | ★3/5 | Advanced | ArcSwap in aptos-core uses epoch-like semantics; validators have many concurrent readers of state |
| **AI Infra** | ★4/5 | Advanced | Epoch-based reclamation in high-throughput inference serving; lock-free queue node reclamation in vLLM |
| **FAANG** | ★2/5 | Not tested at E5 | Crossbeam (Rust) epoch reclamation; Cloudflare/Discord Rust advanced signal |
| **Dubai** | ★3/5 | Advanced | HFT-adjacent crypto firms (Bybit/OKX) |
| **Composite** | **★3.0/5.0** | **Tier 2** | — |

### 必要(Required for senior infra interviews)

> 本 package 的跨 vertical 共識必要項集中在 EBR 與 lock-free DS 的配合:

- **EBR (Epoch-Based Reclamation)** — 比 HP 便宜的 reclamation;reader fast path = 2 loads;crossbeam-epoch 是 Rust reference
  - Evidence: [Keir Fraser PhD 2003](https://www.cl.cam.ac.uk/techreports/UCAM-CL-TR-579.pdf) "Practical lock-freedom"; Rust crossbeam-epoch widely used in Crypto/AI Infra (Tokio, Jito)

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator — 特別是 AI Infra / HFT Rust 路線。

- **QSBR (Quiescent-State Based)** — zero cost in critical section;適合 long read path;Linux kernel RCU variant
  - Best for: AI Infra (vLLM long decode steps = long critical sections; QSBR reduces overhead)
- **Hybrid schemes (IBR)** — HP + EBR;bounded reclamation + low reader cost
  - Best for: HFT (precise memory control required)
- **EBR vs QSBR tradeoff** — EBR stuck-thread = unbounded memory;QSBR requires safe point discipline
  - Best for: All — demonstrating nuanced knowledge of tradeoffs is a strong senior differentiator

### Recommended Order(本 package 內部)

1. EBR(接 M&S queue, 對比 hazard/)
2. QSBR(與 rcu/ 共生)
3. DRC(shared_ptr 互通,選做)
4. IBR(paper-level,進階)

### 對應的 Blog 題材(若想寫)

- "EBR vs HP vs QSBR:三種 lock-free memory reclamation 的工程 tradeoff"
