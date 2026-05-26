# Stack Family TODO

> Category 抽象: LIFO transfer。Push/Pop 都動 head 指標。
> Underlying logic: 單一熱點 (head) → contention 從 single CAS spot 爆炸。三條優化路徑:elimination、flat combining、bounded array。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **A(3mo·廉價完成)** | **9.90** | 5.5/10 | 0.9 `crossbeam`(Treiber) | 0.5 週 | T2 |

> 4 變體已實作;polish + benchmark 即可,ROI 高因剩餘工時極低。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- LIFO order: 在 concurrent 下 push/pop linearization 順序 = LIFO of *commit* order
- 所有 variant 的瓶頸都在 head 上的 CAS contention
- ABA 是 Treiber stack 的經典坑 — 需要 HP / tagged pointer 或 EBR

## Inventory

| Variant | Impl | Status |
|---|---|---|
| MutexSliceMPMC | slice + mutex | done |
| MutexLinkedMPMC | linked-list + mutex | done |
| LockFreeMPMC (Treiber) | CAS on head, linked-list | done (**Note:** 還沒接 reclamation,有 ABA + UAF 風險) |
| EliminationBackoffMPMC | Treiber + elimination array | done |
| **FlatCombining stack** | leader 代跑所有 op,其他 thread 提交 + spin own slot | TODO |
| **Bounded array-based** | array + atomic top | TODO |
| **TreiberStack + HazardPointer** | 把現有 Treiber 接上 reclamation | TODO (production-grade 升級) |

## Variant contracts

### Flat Combining

**Signature:**
```
type FlatCombiningStack[T] struct {
    head    atomic.Pointer[node[T]]
    lock    atomic.Bool
    pubList atomic.Pointer[publicationRecord[T]]  // thread-local 提交 slots
}
type publicationRecord[T] struct {
    op   atomic.Int32  // PUSH / POP / DONE
    val  T
    next *publicationRecord[T]
    lastUsed uint64  // for GC of inactive records
}
```

**Contract:**
- Caller 寫 own publication record (op + val),嘗試 CAS 成為 leader
- Leader 巡邏 publication list,sequential 執行所有 pending ops on local stack
- Non-leader 在 own record 上 spin 等 `op == DONE`
- Leader 解鎖前一次寫所有結果

**Key insight:** 在高 contention 下,*放棄並行* 反而比硬要 CAS 競爭快。把 N 個 CAS war → 1 個 sequential code path。Trade fairness for throughput。

**Reference:** Hendler, Incze, Shavit, Tzafrir 2010 "Flat Combining and the Synchronization-Parallelism Tradeoff", SPAA。Folly 有實作。

### Bounded Array-based

**Signature:**
```
type BoundedArrayStack[T] struct {
    buf  []T
    top  atomic.Int64  // -1 = empty
    cap  int64
}
```

**Contract:**
- Push: CAS top 從 t 到 t+1,寫 buf[t+1]
- Pop: CAS top 從 t 到 t-1,讀 buf[t]
- Push 滿了 fail (or block);Pop 空了 fail
- **Note:** publication safety — Push 寫 buf 必須在 CAS 之後完成 (release on buf write before CAS publish);Pop 對稱

**Key insight:** 固定容量,不需要 reclamation。適合 thread-pool 的 task buffer。但有 wraparound 限制。

### TreiberStack + HazardPointer

**Contract:**
- 修改 `lockfree_mpmc.go`:
  - Pop: protect head.next via HP slot,reload check,再 CAS
  - Push: 不需要 HP (CAS-only)
  - 任何 Pop 成功的舊節點 → retire

**Key insight:** 補上這層後,Treiber stack 才是 production-grade。沒這層 = 玩具實作。

## Career signal

- Stack 在 HFT 用得少 (queue 更常見);但 elimination + flat combining 是 "進階 lock-free 思維" 的 signal
- 寫一篇 blog 比較 3 條優化路徑 (Treiber → Elimination → Flat Combining) 在不同 contention 下的曲線 = 強 differentiator
- AI infra: task pool 偶爾用 stack-of-work

## Recommended order

1. **TreiberStack + HazardPointer** (production-grade 升級,優先!)
2. FlatCombining (新 variant + blog 素材)
3. Bounded array-based (簡單 + 跟 task buffer 對接)

## Dependencies

- Treiber → `reclamation/` (HP) for production
- Elimination 用現有的 backoff,不需要 reclamation
- 全部 lock-free → `memory/`

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 2(Composite ★2.6)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★3/5 | Required (theory) / Advanced (elimination) | ABA problem cited as "classic HFT question"; Herlihy "Art of Multiprocessor Programming" referenced at Jump |
| **Crypto** | ★3/5 | Advanced | Treiber stack as primitive building block for validator memory management; HFT-style crypto (Hyperliquid) uses similar primitives |
| **AI Infra** | ★2/5 | Niche | Task pool occasionally uses stack-of-work; lower ROI than queue/ |
| **FAANG** | ★3/5 | Advanced | Lock-free stack via CAS = canonical lock-free DS interview; ABA problem illustration |
| **Dubai** | ★2/5 | Niche | Less tested; stack not in top Dubai signals |
| **Composite** | **★2.6/5.0** | **Tier 2** | — |

### 必要(Required for senior infra interviews)

> 本 package 的跨 vertical 共識必要項以 Treiber stack + ABA 問題為核心。

- **Treiber stack (lock-free CAS)** — 經典 lock-free DS interview question;ABA 問題的教科書案例
  - Evidence: [quantlabsnet.com HFT interview guide](https://www.quantlabsnet.com/post/how-to-ace-the-hardest-c-interview-questions-in-hft) — ABA cited as "classic HFT question"
- **ABA 問題 + Hazard Pointer 接法** — 沒有 reclamation 的 Treiber stack = 玩具;解釋 ABA 是 senior 分水嶺
  - Evidence: [Herlihy Art of Multiprocessor Programming Ch.11](https://www.quantblueprint.com/job/jump-c-software-engineer) — "referenced at Jump Trading"; Maged Michael hazard pointer paper

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator。

- **Elimination-backoff stack** — 高 contention 下比 Treiber 快;pair Push/Pop 在 side array 直接交換
  - Best for: HFT / Crypto (HFT-style shops: Hyperliquid)
- **Flat Combining stack** — 放棄並行換 throughput;Folly 有實作;反直覺但在高 contention 勝
  - Best for: HFT (Hendler-Incze-Shavit-Tzafrir 2010 SPAA paper)
- **ABA-safe with tagged pointer** — 64-bit 指針上借用高位存 version;不需要 HP
  - Best for: HFT (C++ 面試題;Go 用 HP 方案)

### Recommended Order(本 package 內部)

1. TreiberStack + HazardPointer(production-grade 升級,優先)
2. FlatCombining(新 variant + blog 素材)
3. Bounded array-based(簡單 + task buffer 接口)

### 對應的 Blog 題材(若想寫)

- "Treiber Stack → Elimination → Flat Combining:三條 lock-free stack 優化路徑的 benchmark"
