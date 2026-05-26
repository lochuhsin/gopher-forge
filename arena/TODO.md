# Arena Allocator Family TODO

> Category 抽象: 預先配置一大塊連續記憶體,從中 bump-allocate;整體一次 free(不需個別 free)。
> Underlying logic: 用空間換時間 — 犧牲「任意 free」換取 malloc/free 的零 latency。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **later(6mo+)** | **2.70** | 6.0/10 | 0.9 `bumpalo`/`typed-arena` | 2.0 週 | T2 |

> region 配置器;HFT 未來賭注(Citadel 落地後)。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- Bump pointer 從低到高;整個 arena 在 scope 結束時一次釋放
- 不支援個別 free — arena allocator 的語意保證
- 線程安全 arena 需要 per-thread bump pointer 或 CAS

## Inventory

| Variant | 特色 | Status |
|---------|------|--------|
| **BumpAllocator** | 最簡單;線性分配,整體 free | TODO |
| **TypedArena[T]** | 只存一種型別;對齊保證 | TODO |
| **SlabAllocator** | 固定大小 slab;freelist 回收 | TODO |
| **PoolAllocator** | 多 slab 的組合;per-size class | TODO |

## Variant contracts

### BumpAllocator

```go
type BumpAllocator struct {
    buf    []byte
    offset atomic.Uint64
    cap    uint64
}
func NewBumpAllocator(size uint64) *BumpAllocator
func (a *BumpAllocator) Alloc(size, align uint64) unsafe.Pointer  // bump + align
func (a *BumpAllocator) Reset()  // bump back to 0 (all memory reused)
```

**Contract:**
- Alloc: align-up offset → CAS (current, current+size) → return ptr
- Reset: store 0 to offset atomically; all previous allocations invalidated
- Out-of-memory: return nil (or panic — design choice)

**Key insight:** On hot path, a bump allocator trades away deallocation flexibility for O(1) guaranteed allocation latency with no GC pressure. This is the axiom behind "no allocation on the hot path" in HFT.

**Reference:** stacygaudreau.com HFT blog — "pre-allocation to prevent external fragmentation"; HRT interview prep — "allocator strategies (thread-local caches, pools/arenas)"

### TypedArena[T]

```go
type TypedArena[T any] struct {
    slab []T
    top  atomic.Int64
}
func (a *TypedArena[T]) Alloc() *T
func (a *TypedArena[T]) Reset()
```

**Contract:**
- Type-safe; no unsafe.Pointer needed
- Alignment handled by Go's slice element alignment
- Reset invalidates all previously returned pointers

### SlabAllocator

```go
type SlabAllocator struct {
    slabSize uint64
    freelist atomic.Pointer[slabNode]  // lock-free LIFO freelist
    backing  *BumpAllocator
}
func (s *SlabAllocator) Alloc() unsafe.Pointer  // pop freelist or bump
func (s *SlabAllocator) Free(p unsafe.Pointer)   // push to freelist
```

**Contract:**
- Fixed-size objects; reuse via freelist (unlike pure bump allocator)
- Freelist is lock-free LIFO — O(1) alloc/free
- **Note:** ABA hazard on freelist if using raw CAS; use HP or tagged pointer

**Reference:** vLLM KV cache BlockSpaceManager is a slab-style allocator; pre-allocated fixed-size blocks for KV cache pages.

## Use case

| Package | 用途 |
|---------|------|
| `queue/` SPSC ring buffer | Pre-allocate ring buffer backing array |
| `ratelimit/` hot path | Token counter storage (avoid GC on refill) |
| vLLM KV cache | BlockSpaceManager = typed arena of KV cache pages |
| HFT order management | Per-tick arena; reset at end of tick |

## 跨語言對照

| 語言 | Variant |
|------|---------|
| Rust | `bumpalo` crate (bump allocator); `typed-arena` crate |
| C++ | `std::pmr::monotonic_buffer_resource` (C++17 PMR) |
| Go | `arena` package (experimental, Go 1.20+); `sync.Pool` (recycling) |
| C | Manual: `char buf[N]; char *p = buf;` |

## Career signal

- HFT "no allocation on hot path" is an axiom — arena is the implementation
- AI Infra: vLLM KV cache allocator is an arena; knowing this is ★★★★ signal
- FAANG: ★1 — not FAANG interview content; valuable for HFT/game-engine/AI infra specialists

## Recommended order

1. BumpAllocator (simplest; explain the concept)
2. TypedArena[T] (type-safe version)
3. SlabAllocator (add freelist reuse)
4. PoolAllocator (multi-size class combination)

## Dependencies

- → `memory/` (alignment, release-acquire for thread-safe bump CAS)
- → `hazard/` (if SlabAllocator freelist needs ABA-safe reclamation)

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 2(Composite ★2.8)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★4/5 | Required | IMC JD: "design and build performance-critical components" implies zero-allocation hot path; HRT prep: "allocator strategies (thread-local caches, pools/arenas)"; stacygaudreau.com: "pre-allocation to prevent external fragmentation" |
| **Crypto** | ★3/5 | Advanced | Firedancer C code uses arena-style allocation; Jito performance engineering (HFT background founders); Hyperliquid low-latency Rust |
| **AI Infra** | ★4/5 | Advanced | vLLM KV cache BlockSpaceManager = arena allocator with O(1) alloc/free; pre-allocated pool eliminates GC pause |
| **FAANG** | ★1/5 | Not tested | "Arena is HFT/embedded/game, not FAANG interview content" per research; ★1 honest scoring |
| **Dubai** | ★2/5 | Niche | HFT-adjacent crypto firms; not mainstream Dubai signal |
| **Composite** | **★2.8/5.0** | **Tier 2** | — |

### 必要(Required for senior infra interviews)

> 本 package 的跨 vertical 共識必要項針對 HFT 和 AI Infra 路線:

- **Bump allocator** — 最簡單的 arena;解釋 "no allocation on hot path" 的實現方式
  - Evidence: [stacygaudreau.com HFT blog](https://stacygaudreau.com/blog/cpp/low-latency-cpp-for-hft-part2/) — "pre-allocation to prevent external fragmentation"; [HRT prep](https://hackerprep.io/blog/hrt-low-latency-cpp-system-design-prep) — allocator strategies
- **Typed arena / slab allocator** — vLLM KV cache allocator 的概念等價;fixed-size block pool
  - Evidence: [AI Infra research](../docs/research/ai_infra.md) — "vLLM KV cache block allocator = pre-allocated pool, O(1) alloc/free"; [IMC JD](https://www.imc.com/us/careers/jobs/4673650101) — performance-critical component design

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator。

- **Thread-local bump allocator** — per-thread arena 避免 CAS;per-CPU slab cache 風格
  - Best for: HFT (kernel slab allocator pattern; per-CPU caches eliminate inter-CPU contention)
- **Arena + epoch-based reset** — 每個 "tick" 重置整個 arena;HFT per-tick allocation pattern
  - Best for: HFT (per-order / per-tick memory management; zero fragmentation guarantee)

### Recommended Order(本 package 內部)

1. BumpAllocator(解釋概念)
2. TypedArena[T](type-safe)
3. SlabAllocator(freelist 回收)
4. PoolAllocator(multi-size class)

### 對應的 Blog 題材(若想寫)

- "vLLM KV cache allocator 的原理:從 bump allocator 到 slab allocator"
- "HFT hot path 的記憶體管理:為什麼 malloc 不能用在 microsecond 路徑上"
