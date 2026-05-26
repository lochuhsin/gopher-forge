# Hazard Pointer (HP)

> Category 抽象: Hazard Pointer 是 [reclamation](../reclamation/TODO.md) family 的一個 variant。
> 本 package 專門實作 HP scheme。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **D(6mo·crossbeam cluster)★** | **2.50** | 7.5/10 | 1.0 `crossbeam-epoch` | 3.0(含reclamation) 週 | T2 |

> 無 GC lock-free 回收 = Rust 最硬 signal。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- Reader 在 deref 前先 `protect(ptr)` — 把指標寫入 own HP slot,加 fence
- Writer unlink 後 `retire(ptr)` — 加進 own retire list
- 週期性 scan: 收集所有 thread 的 HP slot,只 free 不在任何 HP 裡的 retired 節點

**核心 protocol — Read fence semantics:**

```
again:
  p = atomic.Load(slot)
  hp[me] = p            // protect
  fence
  p2 = atomic.Load(slot) // reload
  if p != p2: goto again
  // now safe to deref p
```

Protect 之後必須 **reload** 確認沒變;沒變才能安全 deref。這個 double-check 是 HP 的精髓。

## 公開 API (signature 草案)

```go
type Domain struct {
    head    atomic.Pointer[HPRec]   // 所有 thread 的 HP record linked list
    retired *retireList             // per-domain retired pool
    threshold int                   // scan trigger
}

type Holder struct {
    domain *Domain
    slots  []atomic.Pointer[byte]   // K 個 HP slots (e.g. K=2)
    retired []retiredItem
}

// Protect a pointer; returns the safely-pinned pointer (after reload check)
func (h *Holder) Protect[T any](slot int, src *atomic.Pointer[T]) *T

// Drop protection on a slot
func (h *Holder) Reset(slot int)

// Mark a pointer for deferred deletion
func (h *Holder) Retire[T any](p *T, deleter func(*T))

// Scan all hazard slots and free retired pointers that are not pinned
func (h *Holder) Reclaim()
```

**Contract:**
- 每個 goroutine 一個 Holder,持有 K 個 HP slot (K 通常 = 2 for queue / stack)
- Retire list size 超過閾值 (e.g. `2 * num_threads * K`) 觸發 Reclaim
- Domain 可分 (per-DS 一個,降低 scan 範圍) 或 global

## Key insight

HP 的成本不對稱:

| 路徑 | 成本 |
|---|---|
| Reader fast path | 1 store + 1 fence + 1 reload check |
| Writer retire | amortized O(1) |
| Reclaim | O(N_threads × K + retire_list_size) |

對 read-mostly DS 不利 (reader 仍要 fence)。read-mostly 應該用 RCU / EBR / QSBR (見 [reclamation/TODO.md](../reclamation/TODO.md))。

## 跨語言對照

- **C++26:** `std::hazard_pointer` 進標準了 (P1121)
- **Folly:** `folly::hazptr_holder` / `folly::hazptr_obj_base`
- **Rust:** `haphazard` crate
- **Linux kernel:** 沒有 (kernel 用 RCU)
- **Java:** 沒有原生 (用 AtomicReference + version stamp 或 GC)

## Reference

- Michael 2004 "Hazard Pointers: Safe Memory Reclamation for Lock-Free Objects", TPDS
- Maged Michael 多篇 follow-up
- C++ proposal P1121 (C++26 入標準)
- Folly source `folly/synchronization/Hazptr.h`

## 對 gopher-forge 的整合 (TODO)

| 消費者 | 用途 |
|---|---|
| `queue/lockfree_mpmc.go` | M&S unbounded 版的 Enqueue/Dequeue 用 HP 保護節點讀取 |
| `stack/lockfree_mpmc.go` | Treiber Pop 用 HP 保護 head.next 讀取 |
| `rcu/` | 可選:RCU 內部用 HP 偵測 grace period |

## Recommended order

1. **Phase 1:** 單一 global domain + 固定 thread 數的 HP slot 表 + Reset + Retire + naive scan
2. **Phase 2:** 動態 thread registration (per-goroutine TLS via runtime.LockOSThread or sync.Pool)
3. **Phase 3:** 接到 Treiber stack 上跑 stress test
4. **Phase 4:** 加 amortized reclaim 閾值

## Career signal

- C++26 把 HP 提到標準 → signal 強
- Folly HP 是面試常考的 reference impl
- 自己刻過 HP 並用在 Treiber stack = production-grade lock-free 的證明
- Citadel / Jane Street / HRT 面試會問 "your lock-free stack does X under workload Y, what fails?" — 答案就是 ABA + UAF,解法是 HP

## Dependencies

- → `memory/` (fence patterns)
- 被 `queue/`, `stack/`, 可選地 `rcu/` 消費

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 2(Composite ★3.0)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★3/5 | Advanced | ABA problem + reclamation cited as senior HFT topic; Folly HP reference impl; Citadel/HRT "lock-free DS under stress: what fails?" |
| **Crypto** | ★3/5 | Advanced | Block-STM uses ArcSwap (similar to hazard pointer principle) for safe concurrent access; Firedancer needs safe concurrent memory access patterns |
| **AI Infra** | ★4/5 | Advanced | Qdrant/Milvus concurrent HNSW graph traversal uses hazard pointer principle; LWN.net HP proposed for Linux kernel 2024 |
| **FAANG** | ★2/5 | Not tested at E5 | MongoDB + Facebook Folly use HP; signal for Rust/C++ infra specialists at Cloudflare/Discord |
| **Dubai** | ★3/5 | Advanced | Relevant for HFT-adjacent crypto firms |
| **Composite** | **★3.0/5.0** | **Tier 2** | — |

### 必要(Required for senior infra interviews)

> 本 package 在 FAANG 標準 E5 loop 不 Required;以下針對 HFT / AI Infra / Crypto 路線:

- **Hazard Pointer announcement protocol** — protect + reload double-check;解釋 ABA + UAF 問題
  - Evidence: [Maged Michael hazard pointers paper](https://www.cs.otago.ac.nz/cosc440/readings/hazard-pointers.pdf) — foundational for lock-free memory safety; [LWN.net 2024](https://lwn.net/Articles/992704/) — HP proposed for Linux kernel
- **Integration with queue/ and stack/** — 讓 lock-free DS 升級到 production-grade
  - Evidence: [quantlabsnet.com HFT interview guide](https://www.quantlabsnet.com/post/how-to-ace-the-hardest-c-interview-questions-in-hft) — "ABA + reclamation = disqualifier for senior roles if you can't discuss it"

### 進階(Advanced / Senior-to-Staff Differentiator)

> Tier 2 — 針對 AI Infra (Qdrant/Milvus vector DB) 和 HFT Rust/C++ 路線。

- **Optimized announcement + amortized reclaim** — threshold-based Reclaim 避免 O(N) scan 過於頻繁
  - Best for: HFT (C++26 `std::hazard_pointer` standard; Folly `hazptr_holder` reference)
- **Per-domain hazard pointers** — domain 可分 (per-DS) 降低 scan 範圍;降低 cross-DS interference
  - Best for: AI Infra (concurrent HNSW traversal in Qdrant/Milvus)

### Recommended Order(本 package 內部)

1. Phase 1: global domain + fixed threads + naive scan
2. Phase 2: dynamic goroutine registration
3. Phase 3: integrate with Treiber stack stress test
4. Phase 4: amortized reclaim threshold

### 對應的 Blog 題材(若想寫)

- "ABA problem + hazard pointer:從玩具 lock-free stack 到 production-grade"
