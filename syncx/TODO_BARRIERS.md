# TODO: Barriers Family — Synchronization Primitives

> A **barrier** synchronizes N goroutines: all must arrive before any can proceed.
> 比 lock 更高層次的協調 — lock 保護 critical section, barrier 同步 phase。

## 家族界線 (重要!)

不要混淆兩個相鄰的家族:

| 家族 | API 對稱性 | 語意 | 代表 |
|---|---|---|---|
| **Barrier** | 對稱,所有人都呼叫 `Wait()` 並 block | N-way rendezvous (集合點) | CyclicBarrier, Static Tree Barrier |
| **Latch / Completion** | 不對稱,signaler 不 block,只有 waiter block | "等別人做完" | CountDownLatch, `sync.WaitGroup` |

判別法: **發 "我到了" 信號的人會不會停下來等其他人?**
- 會 → barrier
- 不會,直接繼續做事 → latch

> 本文件主軸是 Barrier 家族;Latch 家族另設一節 (§G) 收錄,避免混淆。
> Herlihy & Shavit 把 CountDownLatch 放在 barriers 章節,因為計數機制相似,但 API 上它是 latch。

---

## A. 核心必懂 (Must-Know Barriers)

### 1. Centralized Counting Barrier
- **Idea**: 一個共享 counter,每個 goroutine 到達時遞減,最後一個喚醒全部,**所有人 block 直到全到齊**。
- **API**: `b.Wait()` (N 個 goroutine 都呼叫後一起返回)
- **問題**: 不能重用 — 第二次使用時會有 race (還沒離開上一輪的 goroutine 看到 counter 已被重置)。
- **價值**: 理解 barrier 的本質,以及為什麼需要 sense reversal。
- **參考**: 任何 OS 教科書 (e.g., OSTEP §28)

### 2. Sense-Reversing Barrier
- **Idea**: 在 Centralized Barrier 加上一個 `sense` flag (bool),每輪翻轉。
  - Goroutine 進入時記住目前的 sense (local),離開條件是 global sense != local sense。
- **解決**: Centralized Barrier 的 reuse race。
- **API**: 同上,但可重複呼叫 `Wait()`。
- **複雜度**: O(N) contention on counter,但 correctness 已正確。
- **參考**: Herlihy & Shavit, *The Art of Multiprocessor Programming*, Ch. 17

### 3. CyclicBarrier (Java-style)
- **Idea**: 給定 parties=N,所有 N 個到達後一起返回,自動重置,可再次使用。
- **API**:
  ```
  NewCyclicBarrier(n int, action func()) *CyclicBarrier
  b.Await() (generation int, err error)
  b.Reset()
  ```
- **多了什麼**: 可選的 `barrierAction` (最後一個到達者執行),broken state 處理 (有人 timeout/cancel 則全部釋放並標記 broken)。
- **參考**: `java.util.concurrent.CyclicBarrier`

---

## B. 可擴展 / 高效能 (Scalable Barriers)

> 為什麼需要這些: Centralized barrier 在 N 變大時,單一 cache line 上會有 O(N²) coherence traffic。

### 4. Combining Tree Barrier
- **Idea**: 把 N 個 goroutine 分成樹狀結構,每個內部節點是一個小的 Centralized Barrier (fan-in = k)。
- **複雜度**: O(log_k N) latency,contention 分散到 N/k 個 cache line。
- **trade-off**: tree depth vs per-node contention。
- **參考**: Yew, Tzeng & Lawrie (1987)

### 5. Static Tree Barrier (Mellor-Crummey & Scott, 1991)
- **Idea**: 兩棵樹 — **arrival tree** (子告訴父我到了) + **wakeup tree** (父告訴子可以走了)。
- **關鍵設計**: 每個 goroutine 只在自己的 cache line 上 spin → 沒有 hot spot。
- **複雜度**: O(log N) latency,O(N) space,**O(1) remote memory references per processor**。
- **論文**: Mellor-Crummey & Scott, *Algorithms for Scalable Synchronization on Shared-Memory Multiprocessors*, TOCS 1991。
- **與 MCS Lock 同論文** — 你已實作 MCS Lock,可以理解這篇的 barrier 設計動機。

### 6. Tournament Barrier
- **Idea**: 模擬單淘汰賽 — 配對,一邊是 "winner" 一邊是 "loser",loser 在固定位置 spin 等 winner,winner 進入下一輪。
- **特性**: 靜態分配誰是 winner/loser (編譯期決定),沒有 atomic CAS,只有 store + spin-on-flag。
- **複雜度**: O(log N) latency。
- **適用**: 已知固定 N 的場景。
- **參考**: Hensgen, Finkel & Manber (1988)

### 7. Dissemination Barrier
- **Idea**: log N 輪,第 k 輪 goroutine i 通知 i + 2^k (mod N),且等待 i - 2^k 的通知。
- **特性**: 沒有中心狀態,完全對稱;**支援任意 N (不必 2 的次方)**。
- **複雜度**: O(log N) latency,O(N log N) total messages。
- **參考**: Hensgen, Finkel & Manber (1988)

### 8. Butterfly Barrier
- **Idea**: Dissemination 的特例,N 必須是 2 的次方,log N 階段,每階段 pairwise 交換。
- **vs Dissemination**: 訊息對稱 (i ↔ j 互通),只支援 2^k。
- **參考**: Brooks (1986)

---

## C. 進階 (Advanced)

### 9. Phaser (Java 7+)
- **Idea**: CyclicBarrier 的廣義版。支援:
  - **動態 party**: `Register()` / `Deregister()` 隨時加入退出。
  - **多 phase**: 每次 advance 都帶 phase number,可監聽 phase 變化。
  - **Tiered**: phaser 可以有 parent,形成層級。
- **複雜度**: 比 CyclicBarrier 顯著高,但表達力強。
- **適用**: fork-join、iterative algorithms 中 worker 動態加入退出的場景。
- **參考**: `java.util.concurrent.Phaser`

### 10. Hybrid (Spin-then-Park) Barrier
- **Idea**: 任何上述 barrier 都可以做成 hybrid — 短 spin (~微秒級) 再 park 進 scheduler。
- **動機**: spin 對 short critical section 快,但 N 個 goroutine 同時 spin 浪費 CPU。
- **Go 特化**: 用 `runtime.Gosched()` 或基於 channel 的 park。
- **這是實作面 tuning,不是新算法**。

---

## D. 實作順序建議

1. **Centralized Counting Barrier** (§A.1) — 暖身,理解問題
2. **Sense-Reversing Barrier** (§A.2) — 體會 reuse race 的解法
3. **CountDownLatch** (§G.L1) — 簡單但有用,認識 latch 家族
4. **CyclicBarrier** (§A.3) — 完整 API (action, broken state, generation, context)
5. **Static Tree Barrier (MCS)** (§B.5) — 銜接你的 MCS Lock,理解 scalable 設計
6. **Dissemination Barrier** (§B.7) — 無中心,優雅
7. **(選) Tournament / Combining Tree** (§B.6 / §B.4) — 對照組
8. **(選) Phaser** (§C.9) — 挑戰題

---

## E. 共通設計考量

| 面向 | 選擇 |
|---|---|
| Reusable? | 一次性 (Latch) / 可重用 (CyclicBarrier) |
| 動態 N? | 固定 (CyclicBarrier) / 動態 (Phaser, WaitGroup) |
| 等待方式 | spin / park / hybrid |
| 取消/timeout | `context.Context` 支援? broken state? |
| Action on release | 最後一個到達者執行任務? (CyclicBarrier action) |
| 公平性 | 通常 N-way symmetric,無 fairness 議題 |
| Memory model | release on arrival, acquire on departure (Go: atomic + happens-before) |

---

## F. 參考資料

- Herlihy & Shavit, *The Art of Multiprocessor Programming*, 2nd ed., Ch. 17 (Barriers)
- Mellor-Crummey & Scott, "Algorithms for Scalable Synchronization on Shared-Memory Multiprocessors", ACM TOCS 9(1), 1991
- Hensgen, Finkel & Manber, "Two Algorithms for Barrier Synchronization", Int. J. Parallel Programming, 1988
- `java.util.concurrent`: CyclicBarrier, CountDownLatch, Phaser source

---

## G. 鄰近家族: Latch / Completion (不是 barrier,但常被混為一談)

> 共同特徵: **signaler 不 block**,只有 waiter block。本節列出來是因為它們常和 barrier 一起被討論,也建議一起實作(API 簡單,且 CyclicBarrier 內部可以用 latch 思路理解)。

### L1. CountDownLatch
- **Idea**: 初始 count=N,`CountDown()` 遞減(不 block),`Await()` 等到 0。**一次性**,不可重用。
- **不對稱**: 呼叫 CountDown 的人不一定要 Await,反之亦然。
- **典型用途**: main goroutine 等待 N 個 worker 啟動完成 / 完成初始化。
- **API**:
  ```
  NewLatch(n int) *Latch
  l.CountDown()
  l.Await()
  l.AwaitContext(ctx) error
  ```
- **vs One-Shot Counting Barrier**: 數學機制相同(計數到 0),但 barrier 的 N 個 participants 都 block 在 Wait;latch 只有 waiter block。
- **參考**: `java.util.concurrent.CountDownLatch`

### L2. WaitGroup (Go 慣用)
- **Idea**: Go 內建的 `sync.WaitGroup`。`Add(n)`, `Done()`, `Wait()`。
- **vs Latch**: WaitGroup 支援**動態 Add**(只要在 Wait 之前完成),Latch 固定 N。
- **重點**: 理解 `sync.WaitGroup` 為何用 uint64 同時儲存 counter + waiter count (compact state, single CAS)。
- **練習價值**: 自己寫一個練 atomic + park/unpark 概念,理解 Go runtime 的 `runtime_Semacquire` / `runtime_Semrelease`。

### L3. (相關) Event / Notification (manual-reset / auto-reset)
- **Idea**: 1 bit 的 latch — `Set()` / `Wait()`。`Reset()` 可選。
- **vs CountDownLatch**: 沒有計數,純粹 signal/wait。Windows `Event`、Python `threading.Event` 是這個。
- **不在主要實作清單,提及作參考**。

### Latch ↔ Barrier 的轉換關係
- 一個 CyclicBarrier 可以看成「重複使用的 latch + 自動 reset + 最後一個 arrival 觸發」。
- 一個 CountDownLatch 可以看成「one-shot counting barrier 的 asymmetric 版本」。
- 學完後,自己畫一張表比較 API 行為(blocking vs non-blocking、reusable vs one-shot、symmetric vs asymmetric),會把整個家族打通。
