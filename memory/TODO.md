# Memory Ordering Primitives TODO

> **Building-block category** — 所有 lock-free / lock-based 程式的底層共通語言。
> Underlying logic: 把硬體 memory model + 編譯器 reordering 抽象成 6 種 ordering,讓演算法描述 *語意* 而不是 *指令*。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **B(3mo·deep showcase)★** | **4.00** | 8.0/10 | 1.0 `std::atomic Ordering` | 2.0 週 | T1 |

> Rust 系統面試核心;Bybit infra 極限。ordering + atomics + OnceCell。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- C++ / Rust / Go atomic 的 6 種 ordering: `relaxed`, `consume`, `acquire`, `release`, `acq_rel`, `seq_cst`
- 同步靠 *配對* — release on writer + acquire on reader 才形成 happens-before
- x86 強 model: release/acquire 幾乎免費 (regular MOV);只有 seq_cst store 要 MFENCE
- ARM / POWER 弱 model: 每個 acquire/release 都要 fence (DMB / SYNC / LWSYNC)

## 應該實作什麼

本 package 不是 DS,是 **primitive 工具 + 教學示範**。

### Phase 1: 文檔 + 範例 (TODO)

| 內容 | Status |
|---|---|
| Cheatsheet: 6 ordering 對應的 x86 / ARM 指令 | TODO |
| Singleton (double-checked locking) 範例 — release/acquire 配對 | TODO |
| Seqlock writer/reader fence dance (跟 `syncx/lock_TODO.md` 對接) | TODO |
| Publisher/subscriber pattern (immutable publish) | TODO |
| Store buffer + invalidate queue 視覺化 (ARM 為什麼需要 DMB) | TODO |

### Phase 2: 抽象封裝 (TODO)

| API | 用途 |
|---|---|
| `Fence` 系列函式 | Go 沒有 explicit fence;用 `runtime.KeepAlive` + atomic CAS 模擬 |
| `Once.Do` 自刻版 | 教學: DCL + memory ordering |
| `OnceCell[T]` | lazy initialization + happens-before |
| `Atomic[T]` wrapper | 統一 atomic API,記錄要求的 ordering |

## Variant contracts

### Once / OnceCell (教學版)

**Signature:**
```
type Once struct {
    done atomic.Uint32  // 0=not done, 1=done
    mu   sync.Mutex
}
func (o *Once) Do(f func())

type OnceCell[T] struct {
    state atomic.Uint32  // 0=empty, 1=initing, 2=done
    val   T              // populated by initer; readable after state==2
    mu    sync.Mutex
}
func (c *OnceCell[T]) GetOrInit(initer func() T) T
```

**Contract:**
- `done.Store(1)` 必須是 release;`done.Load() == 1` 必須是 acquire
- Fast path: 一次 acquire load,success 直接返回
- Slow path: 拿 mutex,double-check,initialize,release-store done=1

**Key insight:** DCL 在 Java 1.4 之前是 *broken* 的 — singleton 物件可能在 constructor 完成之前被別的 thread 看到 (writer 把 `instance = new Foo()` 切成 alloc + assign 兩步,並 reorder)。Java 5 引入 volatile 語意修正、C++11 用 memory_order_acquire/release 修正。Go 的 `sync/atomic` 都是 seq_cst → 不會錯但代價是過度同步。

**Reference:** Java Memory Model paper (Manson, Pugh, Adve 2005)。C++11 memory model (Boehm, Adve)。

### Fence 工具 (Go 特化)

Go 的 atomic 是 seq_cst,沒有 fine-grained ordering API。但在 weak memory hardware (ARM64 Mac) 上,有時候你只需要 acquire/release 不需要 seq_cst — 過度同步浪費。

**TODO:**
- 寫一個 build-tag 區分 amd64 / arm64 的 `Fence` 包裝
- 在 amd64 上多數 fence 是 no-op
- 在 arm64 上 emit DMB

### Memory model visualization

**TODO:** 寫文件解釋:
- Store Buffer (x86):writer 寫到 store buffer,reader 可能看到 stale value 直到 buffer drain。MFENCE 強制 drain
- Invalidate Queue (ARM):reader 收到 invalidate 但 lazily 處理。DMB 強制處理
- 配對 fence: release-store 把 store buffer drain;acquire-load 把 invalidate queue drain

## 跨語言對照

| 語言 | Ordering API |
|---|---|
| C++11+ | `std::memory_order_relaxed` / acquire / release / acq_rel / seq_cst / consume |
| Rust | `Ordering::Relaxed` / Acquire / Release / AcqRel / SeqCst |
| Go | 只有 seq_cst (`sync/atomic`);別的要 build tag + assembly |
| Java | `volatile` (acquire+release for that var), `VarHandle.acquireFence()` (Java 9+) |
| Linux kernel | `smp_rmb / smp_wmb / smp_mb`, `READ_ONCE / WRITE_ONCE`, `smp_load_acquire / smp_store_release` |
| C# | `Volatile.Read / Write`, `Thread.MemoryBarrier` |

## Reference

- Sutter & Alexandrescu 2011 "C++ and Beyond: atomic<> Weapons" — 必看 talk
- Preshing on Programming blog (全系列) — 最好的 memory model 入門
- Linux kernel `Documentation/memory-barriers.txt` — 硬核
- "C/C++11 mappings to processors" by Peter Sewell

## Career signal

- HFT C++ 面試直問 `memory_order` (Citadel, Jane Street, HRT, Optiver baseline)
- 你 Go primary 但能講清楚 *為什麼* ARM 上 SPSC queue 要顯式 fence = 強 signal
- "Why does the Linux kernel have both `smp_mb()` and `barrier()`?" — 一個是 CPU 一個是 compiler。能講清楚 = senior 水準

## Recommended order

1. 寫 cheatsheet (一頁紙就夠) — 給自己跟未來面試 prep 用
2. 自刻 Once / OnceCell 並寫詳細註解講 release/acquire
3. 找一個 Go race detector 抓不到的 ordering bug,寫成 case study

## Dependencies

- 被所有 lock-free DS 消費 (queue, stack, deque, rcu, reclamation, hazard)
- 沒有 dependency — 是最底層

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 1(Composite ★3.4)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★5/5 | Required | HRT verbatim: "When is acquire/release sufficient, and when do you need stronger ordering?"; brocbyte HFT-03 dedicated memory model tutorial |
| **Crypto** | ★4/5 | Advanced | Coinbase LMAX: "10x fewer allocations, 24x faster" — understanding GC pressure requires memory ordering; Jito JD: "5+ years systems-level programming" |
| **AI Infra** | ★3/5 | Required (RWMutex/ordering) | vLLM KV cache block table uses RWMutex ordering; SpinLock = CUDA atomicCAS-based shared memory locks |
| **FAANG** | ★2/5 | Advanced only | Go memory model (happens-before via channels/mutex) tested at Cloudflare/Snowflake; Go race detector is more FAANG-relevant than ordering theory |
| **Dubai** | ★3/5 | Advanced | Relevant for Dubai HFT-adjacent roles (Bybit/OKX matching engine) |
| **Composite** | **★3.4/5.0** | **Tier 1** | — |

### 必要(Required for senior infra interviews)

> 在 **≥2 個 vertical** 被列為 Required,或 composite ≥ 3.4。

- **Acquire/Release 配對拆解** — HFT 最被引用的單一 topic(31 個 sources 引用);release on writer + acquire on reader 的 happens-before chain
  - Evidence: [brocbyte HFT-03](https://brocbyte.substack.com/p/hft-03-how-you-could-invent-the-c) — dedicated HFT memory model tutorial; [HRT prep guide](https://hackerprep.io/blog/hrt-low-latency-cpp-system-design-prep) — verbatim question
- **Publication safety patterns** — immutable publish + acquire load;OnceCell DCL 修正
  - Evidence: brocbyte HFT-01 — "Build your own std::mutex" using futex internals + memory ordering
- **Double-checked locking 三錯一對** — Java 1.4 broken → volatile 修正;C++11 atomic 修正;Go atomic = seq_cst 不會錯但可優化
  - Evidence: [FAANG research](../docs/research/faang.md) — memory ordering tested at Cloudflare (Rust) and Snowflake (Java JMM)
- **OnceCell[T]** — lazy initialization under concurrent access;state 0=empty → 1=initing → 2=done
  - Evidence: FAANG ★★★★ — "`sync.Once` implementation from scratch using atomics" is senior Go interview question

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator。

- **Litmus tests** (放 `_lab/verify/`) — 展示 formal correctness reasoning;x86 vs ARM reordering 對比
  - Best for: HFT (Citadel C++26 early adoption; Herb Sutter memory safety emphasis)
- **Atomic refcount** — Arc/shared_ptr 等價;release on decrement + acquire if zero
  - Best for: HFT (C++ equivalent of `std::shared_ptr` atomic refcount)
- **x86 vs ARM fence 分析** — x86 release/acquire 是 free (regular MOV);ARM 需要 DMB;MFENCE 只在 seq_cst store 需要
  - Best for: HFT (Citadel/HRT/XTX;C++ memory model interview distinguishes senior from junior)

### Recommended Order(本 package 內部)

1. 寫 cheatsheet:6 ordering 對應 x86/ARM 指令
2. Once / OnceCell 自刻(release/acquire 詳細注解)
3. 找 Go race detector 抓不到的 ordering bug,寫 case study
4. Litmus tests(放 _lab/verify/)

### 對應的 Blog 題材(若想寫)

- "x86 vs ARM memory ordering:為什麼你的 SPSC queue 在 Mac M1 上需要顯式 fence"
- "OnceCell:從破損的 DCL 到正確的 lazy initialization 三種語言對比"
