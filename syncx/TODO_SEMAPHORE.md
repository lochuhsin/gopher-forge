# Semaphore Family TODO

> Category 抽象: 維持 N 個 permit;Acquire 等到有 permit,Release 歸還一個。
> Underlying logic: Counting state + waiter queue + park/unpark。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **A(3mo·廉價完成)** | **5.53** | 6.5/10 | 0.85 `tokio::sync::Semaphore` | 1.0 週 | T1 |

> 5 變體已實作;補 weighted/timeout/priority。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- 同時持有 permit 的 caller ≤ initial N (counting semaphore)
- 同時持有 permit 的 caller ≤ 1 (binary semaphore,= mutex 但 ownership 可移轉)
- Acquire 在 permits == 0 時 block (不忙等就要 park)
- Release 喚醒一個 waiter (或留 permit 給下一個來的人)
- Permits 可以由不同 goroutine acquire / release (與 mutex 不同,mutex 必須 same owner)

## Inventory

| Variant | 喚醒機制 | Status |
|---|---|---|
| ChannelSemaphore | channel buffer | done |
| MutexSemaphore | mutex + per-waiter mutex queue | done |
| CondSemaphore | sync.Cond | done |
| LockfreeSemaphore | CAS + lock-free waiter queue | done (但 waiter 用 spin,可改 park) |
| **WeightedSemaphore** | acquire N permits atomically | TODO |
| **TimeoutSemaphore** | Acquire with deadline / context | TODO |
| **Priority Semaphore** | FIFO + priority class | TODO |
| **Binary Semaphore** (init=1 變體) | 與 mutex 對比教學 | TODO (概念性,可選) |
| **Counting Semaphore via Futex** | C++20 `std::counting_semaphore` 風格 | TODO |

## Variant contracts (TODO)

### WeightedSemaphore (golang.org/x/sync/semaphore 對照)

**Signature:**
```
type WeightedSemaphore struct {
    cur     int64
    size    int64
    waiters list.List  // 每個 waiter 記錄要的 weight + ready channel
    mu      sync.Mutex
}
func (s *WeightedSemaphore) Acquire(ctx context.Context, n int64) error
func (s *WeightedSemaphore) TryAcquire(n int64) bool
func (s *WeightedSemaphore) Release(n int64)
```

**Contract:**
- 一次拿 N 個 permit (例如 GPU memory size, network bandwidth)
- 不夠就 enqueue 等
- Release 後 head waiter 拿不到那麼多 → 繼續等 (head-of-line blocking)
- *不能讓 head 跳過*,否則可能 deadlock (large request 永遠拿不到)

**Key insight:** Head-of-line blocking 是 weighted semaphore 的本質特性。為了避免 starvation 必須 FIFO drain。`golang.org/x/sync/semaphore` 就是這個設計,被 grpc-go / kubelet 等用。

### TimeoutSemaphore / Context-aware

**Contract:**
- `Acquire(ctx)` 在 `ctx.Done()` 時返回 `ctx.Err()`
- 涉及兩個併行 race: Acquire 等待 permit vs ctx 取消
- 取消的 waiter 必須從 queue 移除 — 否則 *cleanup race* (Release 喚醒已取消的 waiter,permit 丟失)

**Key insight:** Cleanup race 必須 atomic 決定誰贏。Go x/sync 用 channel 自然解這個 (select on ctx.Done + sem-channel)。手刻 lock-free 版需要 CAS waiter state from `waiting` 到 `cancelled` 或 `signalled`。

### Counting Semaphore via Futex (C++20 風格)

**Signature:**
```
type CountingSemaphore struct {
    count atomic.Int64  // 剩餘 permit 數;< 0 表示 |count| 個 waiter
}
func (s *CountingSemaphore) Acquire()
func (s *CountingSemaphore) Release()
func (s *CountingSemaphore) TryAcquire() bool
```

**Contract:**
- Acquire: `n = count.Add(-1); if n < 0: futex_wait(&count, n)`
- Release: `n = count.Add(1); if n <= 0: futex_wake(&count, 1)`
- Negative count 編碼 waiter 數量

**Key insight:** 把 permit count + waiter count 用一個 atomic Int 編碼 — 正數 = 剩餘 permit,負數 = waiter 數的相反數。一個 atomic add 同時更新兩個狀態。

**Reference:** C++20 `std::counting_semaphore`, Linux glibc sem_t。

### Priority Semaphore

**Contract:**
- 多個優先級 class (e.g. high, normal, low)
- Release 先喚醒 high priority 的 waiter
- 內部多個 FIFO queue (per priority)
- 注意 priority inversion 風險

**Use case:** rate limiter with priority classes, AI inference scheduler。

### Binary Semaphore vs Mutex (教學)

**對比表:**

| 特性 | Mutex | Binary Semaphore |
|---|---|---|
| Ownership | Lock 跟 Unlock 必須 same owner | 任何 goroutine 都可以 Release |
| 初始值 | unlocked | 可以是 0 (initially locked) 或 1 |
| 用途 | 保護 critical section | signaling (一邊 wait, 另一邊 release) |
| Recursive | 有 recursive variant | 沒意義 |

**Key insight:** Binary semaphore 跟 mutex 在 signature 上像但 *語意* 完全不同。Java 用 `Semaphore(1, fair)` 模擬 mutex 是 anti-pattern (失去 mutex 的 ownership 檢查)。

## 跨語言對照

| 語言 | API |
|---|---|
| Go | `golang.org/x/sync/semaphore` (weighted) |
| Java | `Semaphore` (fair / unfair option), `tryAcquire(n, timeout)` |
| C++20 | `std::counting_semaphore<N>`, `std::binary_semaphore` |
| Rust | `tokio::sync::Semaphore` |
| Python | `threading.Semaphore`, `asyncio.Semaphore` |
| POSIX | `sem_t`, `sem_wait` / `sem_post` |
| Win32 | `CreateSemaphore`, `WaitForSingleObject` |

## Use case

- Rate limiter (允許同時 N 個 outbound request)
- Connection pool (限制同時 connection 數)
- Producer-consumer 緩衝控制
- GPU memory budget (weighted: 不同模型不同 size)
- Bulkhead pattern (microservice 隔離)

## Career signal

- 你已做 4 個 impl → semaphore family 在 gopher-forge 基本飽和
- 補 **Weighted** 是「實用 → production-grade」最後一塊 (golang.org/x/sync 是 reference)
- 補 **Context-aware Timeout** 證明懂 cleanup race
- **Counting via Futex** 是 C++20 對照 → HFT C++ signal
- Rate limiter / connection pool 在所有 backend 面試都會出現

## Recommended order

1. **Weighted Semaphore** (x/sync 對照,實用)
2. **Context-aware Timeout** (cleanup race 教學)
3. **Counting via Futex** (C++ 對照 + 接到 `park/`)
4. (選做) Priority Semaphore, Binary vs Mutex 教學文件

## Dependencies

- → `park/` (LockfreeSemaphore 目前用 CPU burn waiter,該改成 park-based)
- → `memory/` (release-acquire on permit count)
- 內部用 mutex + cond,或 park/unpark

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 1(Composite ★3.4)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★3/5 | Required (basic) / Advanced (lock-free) | Optiver Glassdoor: "asked about semaphore to achieve thread synchronization"; HFT pre-trade risk controls use semaphore-style throttling |
| **Crypto** | ★3/5 | Required | RPC rate limiting (Helius, QuickNode); block subscription management; Chainlink OCR submission throttling |
| **AI Infra** | ★3/5 | Required (weighted) | GPU memory budget enforcement — each request reserves N KV cache blocks; vLLM uses weighted semaphore semantics |
| **FAANG** | ★4/5 | Required | Amazon SDE3: "implement a distributed semaphore for rate limiting across 100 nodes"; `chan struct{}` as semaphore is classic Go interview trick |
| **Dubai** | ★4/5 | Required | ratelimit/ 的底層原語;Bybit/Binance infra 常用 |
| **Composite** | **★3.4/5.0** | **Tier 1** | — |

### 必要(Required for senior infra interviews)

> 在 **≥2 個 vertical** 被列為 Required,或 composite ≥ 3.4。

- **Counting Semaphore** — 跨所有 vertical 都 Required;bounded goroutine pool 是 Go 面試標準題
  - Evidence: [Optiver Glassdoor](https://www.interviewquery.com/interview-guides/optiver-software-engineer) — "asked about semaphore to achieve thread synchronization"
- **Binary Semaphore vs Mutex 對比** — ownership semantics 差異是面試教學點;Java `Semaphore(1, fair)` anti-pattern
  - Evidence: FAANG research — semaphore ★★★★ at E5; `chan struct{}` as semaphore is Go senior question
- **WeightedSemaphore** — AI Infra Required:GPU memory budget / KV cache block reservation
  - Evidence: [vLLM scheduler design](https://github.com/vllm-project/vllm) — weighted semaphore for KV cache budget enforcement; mirrors `golang.org/x/sync/semaphore`

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator,或 composite < 3.4 但有特定 vertical 看重。

- **Context-aware Timeout Semaphore** — cleanup race 問題是進階面試考點;理解 cancel waiter 正確移除
  - Best for: FAANG / AI Infra (production code 必備)
- **Priority Semaphore** — AI Infra inference scheduler 用;VIP vs standard traffic
  - Best for: AI Infra (LLM serving multi-priority admission control)
- **Counting via Futex (C++20 style)** — 負數編碼 waiter 數;`std::counting_semaphore` 對照
  - Best for: HFT (C++ interview 對照)

### Recommended Order(本 package 內部)

1. WeightedSemaphore(x/sync 對照,實用)
2. Context-aware Timeout(cleanup race 教學)
3. Counting via Futex(C++20 對照,接 park/)
4. Priority Semaphore(AI Infra inference scheduler)

### 對應的 Blog 題材(若想寫)

- "Go semaphore 完全指南:從 chan struct{} 到 weighted semaphore"
- "GPU memory budget with weighted semaphore:vLLM KV cache 設計"
