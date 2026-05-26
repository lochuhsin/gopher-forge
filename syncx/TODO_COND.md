# Condition Variable / Monitor Family TODO

> Category 抽象: 在 lock 內 atomic 釋放 + 睡眠 + 重獲。等待 predicate 變 true。
> **Note:** 你的 `cond.go` 目前是空的;之前 `CondSemaphore` 是 *使用* sync.Cond,不是自刻。
> Underlying logic: Mesa vs Hoare semantics — signal 後誰先跑 (signaler vs signaled)。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **later(已大致完成)** | **—** | 4.5/10 | 0.7 `std::Condvar` | 0.3 週 | T3 |

> Mesa 變體已實作,完成度高,career 排序低。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- `Wait()` 必須在 hold lock 時呼叫 — atomic 釋放 lock + 進 wait queue
- `Signal()` 喚醒一個 waiter,但 waiter **不能假設 predicate 為真** — 必須 *while loop* 重檢
- **Mesa semantics:** signal 後 signaler 繼續持有 lock,signaled 排隊等 lock (Go / Java / POSIX / Rust 都是 Mesa)
- **Hoare semantics:** signal 立刻 transfer lock 給 signaled — 只在教科書與少數 academic systems

## Inventory

| Variant | Style | Status |
|---|---|---|
| Mesa Cond (wrap sync.Cond) | wrap | 已用在 CondSemaphore |
| **Self-implemented Mesa Cond** | 自刻 + park/unpark | TODO |
| **Hoare-style monitor** | signal transfers lock | TODO (教學用,不實用) |
| **Monitor (整合 lock + cond)** | one object = lock + cond | TODO |
| **OneShotCond** | publish-once, no Reset | TODO |
| **Cond with timeout** | WaitTimeout / WaitContext | TODO |

## Variant contracts (TODO)

### Mesa Cond (自刻版,教學)

**Signature:**
```
type Cond struct {
    L       sync.Locker
    waiters list.List   // FIFO queue of *waiter
    mu      sync.Mutex  // 保護 waiters queue
}
type waiter struct {
    sema uint32         // 用 runtime_Semacquire 來 park
}
func (c *Cond) Wait()            // L.Unlock + park + L.Lock
func (c *Cond) Signal()          // wake one
func (c *Cond) Broadcast()       // wake all
```

**Wait() 內部 protocol:**
```
w := &waiter{}
c.mu.Lock()
c.waiters.PushBack(w)   // (1) enqueue BEFORE unlock L
c.mu.Unlock()
c.L.Unlock()             // (2) release outer lock
runtime_Semacquire(&w.sema)  // (3) park
c.L.Lock()               // (4) re-acquire outer lock
```

**Signal() 內部 protocol:**
```
c.mu.Lock()
w := c.waiters.PopFront()
c.mu.Unlock()
if w != nil { runtime_Semrelease(&w.sema) }
```

**Contract:**
- Caller 必須 *while loop*:
  ```
  c.L.Lock()
  for !predicate() { c.Wait() }
  // do stuff
  c.L.Unlock()
  ```
- Signal 喚醒一個 waiter (FIFO),但 waiter 必須跟新來的人重新爭 L

**Key insight (順序很重要):** "Wait 必須 *enqueue before unlock L*" — 如果先 unlock L 再 enqueue,signal 可能在中間發生,看不到我這個 waiter,我就被 *永遠掛起*。先 enqueue 確保 signal 一定看得到。

**為什麼 Mesa 必須 while loop:**
1. *Spurious wakeup* — OS / runtime 偶爾無故喚醒 (POSIX 明確允許)
2. *Stolen wakeup* — 醒來之後拿 L 之前,另一個 thread 拿 L 並改變 predicate
3. Mesa 不保證 signaled 是「下一個」執行的 — predicate 可能又變了

這三個原因任一個都讓 `if (!predicate) wait()` broken。

**Reference:** Lampson, Redell 1980 "Experience with Processes and Monitors in Mesa", CACM。

### Hoare-style Monitor (教學)

**Contract:**
- Signal 立刻 transfer 鎖給 signaled,signaler 進另一個 queue (urgent queue)
- Waiter 醒來時 predicate 必為 true (因為 signaler 看到了才 signal)
- 可以用 `if` 不用 `while` (semantic 保證)
- 實際 OS 都不用 — performance 差且實作複雜 (鎖的 ownership 要 transfer)

**Reference:** Hoare 1974 "Monitors: An Operating System Structuring Concept"。

### Monitor (整合 lock + cond)

**Signature:**
```
type Monitor[State any] struct {
    state State
    mu    sync.Mutex
    cond  *Cond
}
func (m *Monitor[S]) With(f func(*S))                 // 在 lock 內執行 f
func (m *Monitor[S]) WaitFor(pred func(S) bool)       // 等 predicate 成立
func (m *Monitor[S]) Notify()                          // signal one
func (m *Monitor[S]) NotifyAll()                       // broadcast
```

**Contract:**
- 把 lock + cond + state 包成一個物件 — Java synchronized blocks 的精神
- 內部 state 只能在 With() / WaitFor() 內存取
- 強迫 caller 不能誤用 (cond 之外的 read / write)

**Key insight:** Java `synchronized` + `wait/notify` 隱含這個 pattern。Go 沒有 sugar,所以多數人在 cond 上犯錯 (忘 while loop, 忘 lock-before-wait)。Monitor 抽象把這些錯誤從 API 層消除。

### OneShotCond (publish-once)

**Contract:**
- 第一次 Notify 後狀態凍結 (等同 ManualResetEvent.Set 後不能 Reset)
- 之後 Wait 立即返回
- 簡化版 — 不需要 while loop (because 沒 spurious wake 設計)

**Use case:** 一次性 publish (例如 server-ready signal)。但通常用 `chan struct{}` close 即可,實用價值低。

### Cond with Timeout / Context

**Signature:**
```
func (c *Cond) WaitTimeout(d time.Duration) bool   // returns true if signaled, false if timeout
func (c *Cond) WaitContext(ctx context.Context) error
```

**Contract:**
- Wait 在 timeout / ctx.Done 時返回,自動 cleanup from waiter list
- 跟 TimeoutSemaphore 一樣的 cleanup race 問題

**Key insight:** Go 的 sync.Cond 沒有 timeout API — Russ Cox 認為 cond + select 不易結合,推薦用 channel + select pattern。但教學上實作一個 timeout cond 可以理解 cleanup race。

## 跨語言對照

| 語言 | API |
|---|---|
| Go | `sync.Cond` (Mesa,需手動配 Locker) |
| Java | `Object.wait/notify` (隱含 lock = this);`Condition` (在 ReentrantLock 上,Mesa) |
| C++ | `std::condition_variable` (Mesa) |
| POSIX | `pthread_cond_wait` (Mesa) |
| Rust | `parking_lot::Condvar`, `std::sync::Condvar` (Mesa) |
| Python | `threading.Condition` (Mesa) |
| .NET | `Monitor.Wait / Pulse / PulseAll` (Mesa) |

## Use case

- Producer / consumer (典型例子) — 但更好的選擇是 channel
- Custom blocking primitive 的內部實作 (semaphore, latch 等)
- 多 condition 共享 lock (Java 的 multi-condition pattern)

## Career signal

- 自刻 cond 證明懂 *atomic 釋放 + park* 為什麼必要 — *enqueue-before-unlock* ordering
- 解釋 Mesa 為什麼 **必須** while loop = 多執行緒系統 baseline 知識
- 講得清 *spurious wakeup* 的三個來源 (spurious / stolen / Mesa 本質) = senior signal
- Java AQS 內部用 ConditionObject,理解 cond 對 AQS 學習有幫助

## Recommended order

1. **Mesa Cond (自刻)** — 核心 protocol,enqueue-before-unlock 是教學重點
2. **Monitor wrapper** — 把易錯 API 包成易用 API
3. **Cond with Timeout** — cleanup race
4. (選做) Hoare-style 教學文件;OneShotCond

## Dependencies

- → `park/` (park/unpark 是底層 — 自刻 cond 必須用 runtime_Semacquire 或 futex)
- → 一個 lock (任意 lock family member;通常用 sync.Mutex 或自刻 MutexLock)
- → `memory/` (waiter 入 queue 跟 signal 的 happens-before)

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 3(Composite ★2.4)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★2/5 | Required (entry bar only) | HRT prep: condition variables as baseline topic; rarely a differentiator at Tier-1 HFT |
| **Crypto** | ★2/5 | Required (CEX) | Matching engine wait-notify patterns; Coinbase/Kraken producer-consumer patterns |
| **AI Infra** | ★3/5 | Required | Request batch assembler waiting for minimum batch size before dispatch to GPU |
| **FAANG** | ★3/5 | Advanced | `sync.Cond` is used but often replaced by channels in Go; Java `wait()/notify()` = senior Java interview question |
| **Dubai** | ★2/5 | Niche | Minor role in Bybit/Binance services |
| **Composite** | **★2.4/5.0** | **Tier 3** | — |

### 必要(Required for senior infra interviews)

> 本 package 的跨 vertical 共識必要項不多;以下是跨 ≥2 vertical 的基礎信號。

- **Mesa Cond 自刻 + while loop 原因解釋** — 多執行緒 baseline 知識;spurious wakeup / stolen wakeup 三個來源
  - Evidence: FAANG research — "Understanding condition variables separates senior from mid-level"; AI Infra: batch assembler Cond pattern

### 進階(Advanced / Senior-to-Staff Differentiator)

> 這是 Tier 3 package — 主要信號集中在展示正確性推理能力。

- **enqueue-before-unlock ordering** — Wait 必須先 enqueue 再 unlock L;反向是 live-lock 反面教材
  - Best for: FAANG (demonstrates understanding of atomic lock + park protocol)
- **Hoare Monitor 對比** — signal 後 signaler vs signaled 誰先執行;教科書 Lampson-Redell 1980
  - Best for: FAANG PLT interviews (OS/PL background candidates at Google/Meta)
- **Monitor wrapper** — 把 lock + cond + state 包成一個物件;Java `synchronized` 的 Go 等價
  - Best for: FAANG (Java Snowflake/Databricks 面試中 synchronized pattern 是 required)
- **Cond with Timeout** — cleanup race pattern;`sync.Cond` 沒有 timeout API = Russ Cox design note
  - Best for: AI Infra (batch assembler with deadline; go 的 select + channel 替代方案)

### Recommended Order(本 package 內部)

1. Mesa Cond 自刻(核心 protocol)
2. Monitor wrapper(易錯 API 包成易用 API)
3. Cond with Timeout(cleanup race)
4. Hoare-style 教學文件(選做)

### 對應的 Blog 題材(若想寫)

- "Mesa vs Hoare:為什麼所有現代 OS 選 Mesa,while loop 是必須的"
- "`sync.Cond` 的三個正確使用 pattern 和三個常見錯誤"
