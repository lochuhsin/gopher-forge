# Latch / Event Family TODO

> Category 抽象: "閘門" — N 個 waiter 等待一個 binary trigger。
> **Note:** 你的 `latch.go` 目前是空的,整個 category 待補。
> Underlying logic: 從 0→1 的單向狀態轉移 + waiter wake-up policy。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **A(3mo·廉價完成)** | **7.70** | 5.5/10 | 0.7 tokio | 0.5 週 | T1 |

> WaitGroup 已實作;補 CountDownLatch 一次性閘門。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- State 從 "未觸發" → "已觸發" 的單向轉移 (one-shot variants)
- 已觸發狀態下 Wait() 立即返回 (publication safety 必須保證)
- 觸發前 Wait() 的 goroutine 必須在觸發後被喚醒
- Reusable variants 必須有 phase / epoch 區分前後 trigger

## Inventory

| Variant | Reset? | 喚醒語意 | Status |
|---|---|---|---|
| **CountDownLatch** | one-shot | counter 到 0 喚醒全部 | TODO |
| **WaitGroup (自刻教學版)** | reusable via Add | Add / Done / Wait;state-packed | TODO |
| **ManualResetEvent** | manual Set / Reset | set 後永遠通 (直到 Reset) | TODO |
| **AutoResetEvent** | auto-reset on wake | set 喚醒 *一個* 後自動 reset | TODO |
| **Notify (tokio-style)** | one-shot or counted | wake one / wake all + permit | TODO |
| **Pulse (edge-triggered)** | edge,無狀態 | 只喚醒當下 waiter | TODO |

## Variant contracts (TODO)

### CountDownLatch

**Signature:**
```
type CountDownLatch struct {
    counter atomic.Int32
    done    chan struct{}  // closed when counter == 0
}
func NewCountDownLatch(n int32) *CountDownLatch
func (l *CountDownLatch) CountDown()  // counter--; if reaches 0, close(done)
func (l *CountDownLatch) Wait()       // <-done
func (l *CountDownLatch) Count() int32
```

**Contract:**
- N 次 CountDown 後,所有 (現在 + 未來) Wait 都立即返回
- 不能 reset (one-shot)
- CountDown 多於 N 次 = no-op (or panic,設計選擇)

**Key insight:** 用 `close(channel)` 做 broadcast wake — Go-idiomatic。其他語言通常用 condvar + boolean flag。CountDown 跟 close 之間有 happens-before: `counter.Add(-1) == 0 ⟹ close(done)`。

**邊界:** CountDown 的 dec 跟 close 之間不能 race — 用 CAS-once-to-zero pattern:
```
if counter.Add(-1) == 0 { close(done) }  // 只有一個 thread 能讓 counter 到 0
```

**Reference:** Java `CountDownLatch`, C++20 `std::latch`, .NET `CountdownEvent`。

### WaitGroup (自刻教學版,對標 Go sync.WaitGroup)

**Signature:**
```
type WaitGroup struct {
    state atomic.Uint64  // [counter:32][waiters:32]
    sema  uint32          // park/unpark via runtime_Semacquire
}
func (w *WaitGroup) Add(delta int)
func (w *WaitGroup) Done()       // Add(-1)
func (w *WaitGroup) Wait()
```

**Contract:**
- Add(positive) 必須在 Wait() *之前* happens-before;否則 race (Go 標準庫 panic on detect)
- Add 把 counter 變 0 時喚醒所有 waiter
- Wait 看到 counter == 0 立即返回

**State machine:**
- `Add(delta)`: CAS state with counter += delta;if counter < 0 panic;if counter == 0 && waiters > 0 → semrelease ALL waiters
- `Wait()`: CAS state with waiters++ if counter > 0;然後 semacquire(&sema);若 counter == 0 直接返回

**Key insight:** 64-bit state 同時編碼 counter + waiter count → 一個 CAS 就 atomic 更新兩個。這跟 Phaser 同樣技巧。沒這個 packing,Add 跟 Wait 之間有 race。

**參考實作:** Go stdlib `sync/waitgroup.go` source 是最好的 reference。

### ManualResetEvent

**Signature:**
```
type ManualResetEvent struct {
    set  atomic.Uint32
    sema uint32
    mu   sync.Mutex     // 保護 waiter 計數
    waiters int
}
func (e *ManualResetEvent) Set()       // 觸發
func (e *ManualResetEvent) Reset()     // 重新 lock 閘門
func (e *ManualResetEvent) Wait()      // 等 set
func (e *ManualResetEvent) IsSet() bool
```

**Contract:**
- Set: store set=1 → 喚醒所有當前 waiter → 後續 Wait 直接通過
- Reset: store set=0 → 後續 Wait 重新阻塞 (但已 wake 的不會 unwake)
- IsSet() 純查詢 (snapshot)

**邊界:** Set 跟 Wait 的競態 — Wait 必須 *enqueue self 之前* 檢查 set,*之後* 再檢查一次;否則 enqueue 跟 Set 之間 race 可能漏 wake。

**跨語言對照:** .NET `ManualResetEventSlim`, Python `threading.Event` (本質是 manual reset)。

### AutoResetEvent

**Signature:**
```
type AutoResetEvent struct {
    permit atomic.Uint32  // 0 or 1
    sema   uint32
}
func (e *AutoResetEvent) Set()
func (e *AutoResetEvent) Wait()
```

**Contract:**
- Set: 喚醒 *一個* waiter,自動 Reset (有 waiter) 或留 permit (沒 waiter)
- 沒 waiter 時 Set 留 permit,下一個 Wait 立即通過,但只給一個

**Key insight:** 等價於 "binary semaphore" — 一個 permit 的 counting semaphore。AutoResetEvent vs ManualResetEvent 是 Windows / .NET 系統最重要的 dichotomy。

### Notify (tokio-style)

**Signature:**
```
type Notify struct {
    permit atomic.Uint32  // 0 or 1
    waiters list.List
    mu     sync.Mutex
}
func (n *Notify) NotifyOne()              // 喚醒一個或留 permit
func (n *Notify) NotifyWaiters()          // 喚醒當下所有 waiter,不留 permit
func (n *Notify) Notified() (waiter, await func())  // 兩階段 API
```

**Contract:**
- `NotifyOne`: 有 pending waiter → wake 一個;沒有 → 留 permit (permit 不會累積超過 1)
- `NotifyWaiters`: 喚醒當下所有 waiter,不留 permit
- 兩階段 API: `Notified()` 先 register 再等 — 避免 wake-before-wait 漏訊

**Reference:** `tokio::sync::Notify`,設計理念來自 Java LockSupport permit semantics。

### Pulse (edge-triggered, 教學用)

**Contract:**
- 只喚醒當下 waiter,沒人在等就什麼都不做 (不留 permit)
- 跟 ManualResetEvent / AutoResetEvent / Notify 對比鮮明 — 完全 edge-triggered

**Key insight:** 教學價值:Pulse 是 "broken" semantic — 容易漏喚醒。從 .NET 的 `Monitor.Pulse` 學到的反面教材。

## 跨語言對照

| Variant | Java | C++ | .NET | Python | Rust |
|---|---|---|---|---|---|
| CountDownLatch | `CountDownLatch` | `std::latch` (C++20) | `CountdownEvent` | `Barrier` (一次性) | `tokio::sync::Barrier` |
| WaitGroup | `Phaser` (近) | n/a | n/a | n/a | n/a (Go 獨有) |
| ManualResetEvent | wait/notify on flag | `std::atomic<bool>` + cond | `ManualResetEvent` | `threading.Event` | `tokio::sync::Notify` |
| AutoResetEvent | `Semaphore(1)` | `binary_semaphore` | `AutoResetEvent` | semaphore | semaphore |
| Notify | n/a | n/a | n/a | n/a | `tokio::sync::Notify` |

## Use case

- CountDownLatch: 等多個 worker 都 init 完;等多個 task 都完成
- WaitGroup: Go 慣用 — fork-join, worker pool 等
- ManualResetEvent: 一次性 startup signal (server ready)
- AutoResetEvent: producer-consumer single-item handoff
- Notify: tokio async signaling (between tasks)

## Career signal

- **WaitGroup 自刻**是 Go 面試經典題 (state packing technique,sync.WaitGroup source 是 reference)
- **CountDownLatch** 是 Java 面試標配,Java 8+ 改名 `Phaser` 變體
- **AutoResetEvent vs ManualResetEvent** 的差別 = Windows / .NET 系統知識
- Wake-before-wait race 的解決 = 通用系統 senior signal

## Recommended order

1. **CountDownLatch** — 最容易、入門 latch 概念
2. **WaitGroup (自刻版)** — 學 state packing,Go source 對照
3. **ManualResetEvent + AutoResetEvent** — 一組做完,概念對立
4. **Notify (tokio-style)** — permit semantics,Rust async 對照
5. (選做) Pulse — 教學反面教材

## Dependencies

- → `park/` (Wait 必須 park 不能 spin;`runtime_Semacquire` wrapper)
- → `memory/` (Set 是 release, Wait 是 acquire)
- 跟 `syncx/future_TODO.md` 概念近 — Future 是 set-value latch

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 1(Composite ★3.4)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★3/5 | Required (WaitGroup) | Two Sigma Rust async framework uses structured concurrency; fork-join for parallel backtesting |
| **Crypto** | ★3/5 | Required | "Appears in every concurrent crypto service" (Latch/WaitGroup); Chainlink OCR async oracle aggregation uses latch-like coordination |
| **AI Infra** | ★3/5 | Required | WaitGroup/Latch for distributed barrier before checkpoint; all workers must sync before saving state |
| **FAANG** | ★5/5 | Required | `sync.WaitGroup` = THE most commonly used Go concurrency primitive; Java `CountDownLatch` = required senior Java |
| **Dubai** | ★3/5 | Required | Common in async coordination at Bybit/Binance Go services |
| **Composite** | **★3.4/5.0** | **Tier 1** | — |

### 必要(Required for senior infra interviews)

> 在 **≥2 個 vertical** 被列為 Required,或 composite ≥ 3.4。

- **WaitGroup(自刻教學版)** — Go 面試第一個並發原語;64-bit state packing technique 是 senior 必答
  - Evidence: [FAANG research](../docs/research/faang.md) — ★★★★★ at FAANG; "sync.WaitGroup is THE most commonly used Go concurrency primitive"
- **CountDownLatch** — Java 面試標配;C++20 `std::latch` 對照;one-shot gate 概念
  - Evidence: [onsites.fyi Amazon SDE3 guide](https://www.onsites.fyi/blog/article/amazon-sde-iii-software-engineer-interview-questions) — Java CountDownLatch cited; FAANG ★★★★★
- **ManualResetEvent** — "server ready" startup pattern;Python `threading.Event` 對照
  - Evidence: AI Infra research — training workers use ManualResetEvent-style coordination for checkpoint sync

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator,或 composite < 3.4 但有特定 vertical 看重。

- **AutoResetEvent** — binary semaphore 語義;Windows/.NET 面試對比教學;producer-consumer handoff
  - Best for: FAANG Java/C# interviews (Snowflake/Databricks)
- **Notify (tokio-style)** — permit semantics;Rust async 面試對照;wake-before-wait race 解法
  - Best for: Crypto/AI Infra (Rust-heavy shops: Jito, Helius, vLLM)
- **Pulse (edge-triggered)** — 反面教材;.NET `Monitor.Pulse` 漏喚醒場景
  - Best for: 教學用 — 展示正確 vs 錯誤 wake-on-set design

### Recommended Order(本 package 內部)

1. CountDownLatch(最容易,入門)
2. WaitGroup 自刻版(64-bit state packing,Go source 對照)
3. ManualResetEvent + AutoResetEvent(概念對立一組)
4. Notify(tokio-style permit semantics)
5. Pulse(教學反面教材)

### 對應的 Blog 題材(若想寫)

- "sync.WaitGroup 原始碼解析:64-bit state packing 的 CAS 技巧"
- "CountDownLatch vs WaitGroup:Java 跟 Go 的 fork-join 慣用法"
