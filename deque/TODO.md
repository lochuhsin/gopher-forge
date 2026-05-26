# Work-Stealing Deque Family TODO

> Category 抽象: 雙端,所有者一端 LIFO push/pop (cheap, no atomic on fast path),其他 thread 另一端 FIFO steal (rare contention)。
> Underlying logic: asymmetric API → asymmetric cost。Owner fast path 零 atomic 是工作竊取調度器的基石。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **D(6mo·crossbeam cluster)★** | **3.50** | 7.0/10 | 1.0 `crossbeam-deque` | 2.0 週 | T3 |

> Chase-Lev **就是** crossbeam-deque,1:1 Rust 對應,work-stealing。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- Owner 端 push/pop bottom — fast path **沒有 atomic** (只在 deque 將空時加 fence)
- Thief 端 steal from top — **永遠 CAS**
- 衝突點只在「deque 剩 1 個」時 — owner pop 跟 thief steal 同時要那一個

## Inventory

| Variant | 來源 | 觀念 | Status |
|---|---|---|---|
| **Chase-Lev deque** | Chase & Lev 2005 | 經典實作,x86/TSO 下最少 fence | TODO |
| **THE protocol deque** | Cilk-5 (Frigo 1998) | 三 fence,owner-thief 對 last 競爭 | TODO |
| **Bounded WS deque** | Le, Pop, et al. 2013 | weak memory model 下 fence 最少 | TODO |
| **Idempotent WS deque** | Michael, Saraswat, Vechev 2009 | 容許重複,fence 最少 | TODO |

## Variant contracts

### Chase-Lev deque (首選實作)

**Signature:**
```
type ChaseLev[T any] struct {
    top    atomic.Int64                       // thief steals from here
    bottom atomic.Int64                       // owner push/pop here
    buf    atomic.Pointer[circularArray[T]]   // growable
}
type circularArray[T any] struct {
    log_size uint64
    seg      []atomic.Pointer[T]
}

func (d *ChaseLev[T]) Push(v T)            // owner only
func (d *ChaseLev[T]) Pop() (T, bool)      // owner only
func (d *ChaseLev[T]) Steal() (T, bool)    // anyone
```

**Contract (簡化版,參考 Le et al. 2013 對應的 ARM 弱 model 補 fence):**

**Push (owner):**
```
b = bottom.Load(relaxed)
t = top.Load(acquire)
a = buf.Load(relaxed)
if b - t >= a.size - 1: a = a.grow()
a.put(b, v)
fence(release)
bottom.Store(b + 1, relaxed)
```

**Pop (owner):**
```
b = bottom.Load(relaxed) - 1
a = buf.Load(relaxed)
bottom.Store(b, relaxed)
fence(seq_cst)
t = top.Load(relaxed)
if t > b: bottom.Store(b+1, relaxed); return empty
v = a.get(b)
if t < b: return v   // deque 不空,owner 獨吞
// t == b: 跟 thief 競爭最後一個
if !top.CAS(t, t+1, seq_cst, relaxed): v = empty
bottom.Store(b+1, relaxed)
return v
```

**Steal (anyone):**
```
t = top.Load(acquire)
fence(seq_cst)
b = bottom.Load(acquire)
if t >= b: return empty
a = buf.Load(consume)
v = a.get(t)
if !top.CAS(t, t+1, seq_cst, relaxed): return empty
return v
```

**Key insight:**
- Owner Push 完全沒 CAS — 只有兩個 store 加一個 release fence
- Owner Pop 只在「剩最後一個」時 CAS top — 平均情況也沒有 CAS
- Steal 永遠 CAS top
- "剩 1 個" 的衝突解決:owner pre-decrements bottom,然後 fence,再讀 top。如果 top > b 表示 thief 拿走了。如果 top == b 表示同時想拿,CAS 決勝。

**Reference:** Chase & Lev 2005 "Dynamic Circular Work-Stealing Deque", SPAA。Le et al. 2013 "Correct and Efficient Work-Stealing for Weak Memory Models" 修正 weak memory 下的 fence 配置。

### Bounded Chase-Lev (Go runtime 風格)

**Contract:**
- 固定容量,push 滿了 fallback 到 global queue (Go runtime 行為)
- 不需要 grow,memory 預配
- Steal 一次拿一半 (batched steal,Go runtime 行為)

**Reference:** Go runtime `runtime/proc.go` `runqput / runqget / runqsteal`。

### Idempotent WS deque

**Contract:**
- 允許 task 被執行多次 (caller 必須冪等)
- 換取最少 fence (適合 ARM / POWER)

**Reference:** Michael, Saraswat, Vechev 2009 "Idempotent Work Stealing"。

## Go runtime 對照

`runtime/proc.go` 的 P-local runq 就是 **bounded Chase-Lev**:

- `runqput`: owner push (no atomic on fast path, batched flush to global queue when full)
- `runqget`: owner pop
- `runqsteal`: 從另一個 P 偷一半 task

讀 Go runtime 這幾個函式對應到 Chase-Lev paper 是極好的學習。

## 跨語言對照

| 語言 / 系統 | Variant |
|---|---|
| Cilk-5 | 原始 (Frigo, Leiserson, Randall 1998) |
| Rust rayon | `crossbeam-deque` crate (Chase-Lev) |
| Java ForkJoinPool | WorkQueue class (Chase-Lev 變體) |
| Go runtime | per-P bounded Chase-Lev |
| Folly | `folly::executors::ThreadPoolExecutor` 內部 |
| Tokio | `tokio::runtime::scheduler` work-stealing scheduler |
| TBB | `tbb::task_scheduler` |

## Use case 對應

- Fork-join parallelism (recursive divide-conquer)
- 任意 user-space scheduler (Go GMP, Tokio, Rayon)
- AI infra training task scheduler (per-GPU work queue)
- HFT internal task dispatch (per-thread task queue + steal)

## Career signal

- 自己刻 Chase-Lev = Go runtime / Rust tokio / Java ForkJoin internals 證明
- 寫一篇 "Go runtime 的 P-local runq 是 bounded Chase-Lev" blog = 稀有內容,Go infra signal
- AI infra training scheduler 是 work-stealing 的 application
- HFT 偶爾考 (用在內部 dispatcher),不是必考但加分

## Recommended order

1. **Chase-Lev (x86 / TSO 版本)** — 先做最少 fence 的版本
2. **Bounded variant** — 對接 Go runtime 學習
3. **Le et al. weak memory model 版** — 解釋為什麼 ARM 上需要更多 fence
4. (選做) Idempotent variant — 給 paper 級興趣的人

## Dependencies

- → `memory/` (release/acquire fence 配對,seq_cst 在剩 1 個的衝突點)
- 不需要 `reclamation/` (circular array growth 是 epoch-style 一次性,且 owner-only 沒有並行 free 問題)

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 3(Composite ★2.4)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★2/5 | Advanced (niche) | No HFT interview source directly asked about Chase-Lev; HFT internal dispatcher pattern but not tested |
| **Crypto** | ★2/5 | Advanced | Block-STM collaborative scheduler uses atomic counters rather than work-stealing; conceptually related |
| **AI Infra** | ★3/5 | Advanced | Per-GPU work queue for training task scheduler = work-stealing application |
| **FAANG** | ★3/5 | Advanced | Go GMP scheduler P-local runq = bounded Chase-Lev; Java ForkJoinPool internals (Databricks/Spark) |
| **Dubai** | ★2/5 | Niche | Low priority in Dubai market |
| **Composite** | **★2.4/5.0** | **Tier 3** | — |

### 必要(Required for senior infra interviews)

> 本 package 沒有跨 vertical 共識必要項;進階信號為主。

- **Chase-Lev deque 自刻** — Go runtime P-local runq 等價;展示 work-stealing scheduler internals
  - Evidence: [crossbeam-deque Rust crate](https://docs.rs/crossbeam/0.3.2/crossbeam/sync/chase_lev/index.html) — reference implementation; Go runtime `runqsteal` is bounded Chase-Lev

### 進階(Advanced / Senior-to-Staff Differentiator)

> Tier 3 — 主要 signal 是 Go runtime / Java ForkJoinPool 深度知識。

- **Bounded Chase-Lev (Go runtime 風格)** — 對接 Go GMP scheduler internals;owner push 零 atomic
  - Best for: FAANG (Go runtime深度knowledge; Uber/Cloudflare Go senior interviews)
- **自己實作 work-stealing scheduler** — 把 Chase-Lev 接成完整 scheduler;deque + global queue + steal
  - Best for: AI Infra (per-GPU task dispatching; Anyscale Ray scheduler internals)
- **Le et al. 2013 ARM weak memory 版** — 解釋為什麼 x86 fence 不夠 ARM
  - Best for: HFT (ARM-based trading servers 需要 DMB fences)

### Recommended Order(本 package 內部)

1. Chase-Lev x86 版本(最少 fence)
2. Bounded variant(Go runtime 對照)
3. Le et al. ARM 版(weak memory model 教學)
4. 自刻 work-stealing scheduler(接 deque)

### 對應的 Blog 題材(若想寫)

- "Go runtime 的 P-local runq 是 bounded Chase-Lev:從論文到 Go source"
