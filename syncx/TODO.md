# syncx — Synchronization Primitives Roadmap

`syncx/` 是同步原語的總目錄。各 family 已拆出獨立 sub-TODO,本檔是**整個 repo 的分類總索引**。

---

## 0. 分類框架(Category vs Family vs Exercise)

| 類型 | 標準 | 範例 | 去處 |
|------|------|------|------|
| **Family** | 單一原語的多個實作變體,只是 implementation 不同 | Lock 的 Spin/Ticket/MCS;Semaphore 的 Channel/Cond/Lockfree | `syncx/` 單檔 |
| **Category** | 獨立概念 + 多個子題目 + 有自己的型別系統 | Future、Cancellation、Logical Clock、CRDT | 獨立 package |
| **Exercise** | 用既有 primitive **解問題**(不寫新 primitive) | Dining Philosophers、Sleeping Barber | `excercise/` |
| **Tool** | 驗證 / 觀測 concurrent code 的工具 | Linearizability checker、Deadlock detector | `verify/` |

---

## 1. Repo-wide 全景

### 1.1 syncx/ 內(Family 層級)

| Family | 狀態 | 已實作 | 子 TODO |
|--------|------|--------|---------|
| **Lock** | ✅ | SpinLock, TicketLock, MCSLock, RCULock, MutexLock, RWMutexLock | [TODO_LOCK.md](TODO_LOCK.md) |
| **Semaphore** | ✅ | Channel, Mutex, Cond, Lockfree, Runtime | [TODO_SEMAPHORE.md](TODO_SEMAPHORE.md) |
| **Channel helpers** | ✅ | OrderedChannel, UnorderedChannel, RingQueue | [TODO_CHANNEL.md](TODO_CHANNEL.md) |
| **Latch / WaitGroup** | ✅ | SpinLatch, ChanLatch, SemaLatch, NotifyListLatch, WaitGroup | [TODO_LATCH.md](TODO_LATCH.md) |
| **Cond** | ✅ | MesaQueueCond, MesaNotifyListCond | [TODO_COND.md](TODO_COND.md) |
| **Barriers** | 🚧 | Counting, SenseReversing(✅);Tournament/CombiningTree/StaticTree/Dissemination/Butterfly(stub) | [TODO_BARRIERS.md](TODO_BARRIERS.md) |
| **Future / Promise** | 💡 | (`future.go` 空檔) | [TODO_FUTURE.md](TODO_FUTURE.md) |
| **Event** | 💡 | 從 `latch.go` 分到 `event.go`(ManualReset / AutoReset / Notify / Pulse) | 併在 TODO_LATCH.md |
| **Once** | 💡 | 尚未開檔(`once.go`),`sync.Once` 自刻版 | — |
| **STM** | 💡 | (`stm.go` 空檔)— 未來可能升級成獨立 package | [TODO_STM.md](TODO_STM.md) |

### 1.2 Repo 其他 Category packages

| Package | 狀態 | 主題 | TODO |
|---------|------|------|------|
| [queue/](../queue/) | ✅ | MPSC / MPMC concurrent queues | (已有實作) |
| [stack/](../stack/) | ✅ | MPMC concurrent stacks(含 elimination-backoff) | (已有實作) |
| [map/](../map/) | 💡 | Sharded / lock-free maps | [map/TODO.md](../map/TODO.md) |
| [deque/](../deque/) | 💡 | Chase-Lev work-stealing deque | [deque/TODO.md](../deque/TODO.md) |
| [memory/](../memory/) | 💡 | Memory ordering / fence / OnceCell 教學 | [memory/TODO.md](../memory/TODO.md) |
| [hazard/](../hazard/) | 💡 | Hazard pointers | [hazard/TODO.md](../hazard/TODO.md) |
| [reclamation/](../reclamation/) | 💡 | EBR / QSBR | [reclamation/TODO.md](../reclamation/TODO.md) |
| [rcu/](../rcu/) | 💡 | Read-Copy-Update 完整版 | [rcu/TODO.md](../rcu/TODO.md) |
| [arena/](../arena/) | 💡 | Region allocator | (待補) |
| [park/](../park/) | 💡 | park / unpark 抽象 | [park/TODO.md](../park/TODO.md) |
| **[scope/](../scope/)** ⭐ | 💡 | **Cancellation + Structured Concurrency** | [scope/TODO.md](../scope/TODO.md) |
| **[clock/](../clock/)** ⭐ | 💡 | **Lamport / Vector / HLC** | [clock/TODO.md](../clock/TODO.md) |
| **[crdt/](../crdt/)** ⭐ | 💡 | **G-Counter / OR-Set / LWW / RGA** | [crdt/TODO.md](../crdt/TODO.md) |
| **[parallel/](../parallel/)** ⭐ | 💡 | **Parallel Scan / Sort / BFS / Map-Reduce / Pipeline** | [parallel/TODO.md](../parallel/TODO.md) |
| **[ratelimit/](../ratelimit/)** ⭐ | 💡 | **Token Bucket / Sliding Window / Circuit Breaker** | [ratelimit/TODO.md](../ratelimit/TODO.md) |
| **[actor/](../actor/)** ⭐ | 💡 | **Mailbox + Scheduler + Supervisor** | [actor/TODO.md](../actor/TODO.md) |

### 1.3 `_lab/` 純學習(底線開頭,Go 不會編譯)

| Path | 主題 | TODO |
|------|------|------|
| [_lab/pattern/](../_lab/pattern/) | Active Object / Reactor / Disruptor / Half-Sync | [_lab/pattern/TODO.md](../_lab/pattern/TODO.md) |
| [_lab/verify/](../_lab/verify/) | Linearizability / Lockset / Deadlock detect | [_lab/verify/TODO.md](../_lab/verify/TODO.md) |
| [_lab/excercise/](../_lab/excercise/) | 經典 puzzles(Philosophers, Smokers, H2O…) | [_lab/excercise/TODO.md](../_lab/excercise/TODO.md) |

> ⭐ = 本次新增的 package

---

## 2. 為什麼這樣分?

### Family 的判準(留 syncx)
- **單一介面,多個實作**:`Lock` interface 有 Spin/Ticket/MCS 等實作,概念都是「mutual exclusion」
- **變體互換不影響呼叫端**:換 `SpinLock` 變 `MCSLock` 只是 perf 改變
- **彼此引用低**:每個 family 不需要 import 其他 family

→ 適合放同一個 package,各檔案獨立

### Category 的判準(獨立 package)
- **獨立概念,不是 primitive 的變體**:`scope/` 解決生命週期,跟 syncx primitive 正交
- **有自己的型別系統**:`crdt/G-Counter` 跟 `crdt/OR-Set` 共享 `Mergeable` 介面,但跟 lock 無關
- **可能成長很大**:`pattern/` 收所有 architectural pattern,各 pattern 各自 100+ 行
- **避免 syncx 變成「什麼都裝」的大袋子**

→ 獨立 package 維持 syncx 的純度

### Exercise 的判準(放 excercise/)
- **只 import 既有原語**,不寫新原語
- **題目本身有教學價值**,但個別實作不可重用
- 每題 30–100 行,用兩三種 primitive 各刻一次對比

---

## 3. 邊界案例(為什麼這樣決定)

| 項目 | 決定 | 理由 |
|------|------|------|
| **Event** | 留 syncx,新檔 `event.go` | 是 latch 家族的變體(0/1 broadcast),拆走會割裂概念 |
| **Future / Promise** | 留 syncx,新檔 `future.go` | Primitive 層級,跟 Latch / Cond 同源(set-value latch) |
| **Once** | 留 syncx,新檔 `once.go` | 是 Future 的退化(value 是 side-effect),小 primitive |
| **Singleflight** | 留 syncx | 單一 primitive,小,跟 Once 同檔即可 |
| **STM** | **暫留** syncx,長大可移出 | 目前 `stm.go` 空檔,設計階段;成熟後可獨立 `stm/` package |
| **Disruptor** | 放 `pattern/` | 是 architectural pattern(single-writer + wait strategy),不是純 queue |
| **Actor** | 獨立 `actor/` 而非 `pattern/` | 是整套 paradigm,跟 channel-based 並列 |
| **Work-Stealing Pool** | 放 `pattern/` 用 `deque/` | Deque 是原始結構,pool 是組合 |
| **Singleflight** | 留 syncx(`singleflight.go`) | 小 primitive,不值得獨立 |
| **Backpressure** | 同時在 `ratelimit/`(credit)跟 `parallel/`(pipeline)出現 | 概念跨領域,各 package 寫自己角度 |

---

## 4. 接下來的路線

### 路線 P:**收斂 syncx 本體**(2–3 週)
1. 補完 Barrier 4 個 stub(Tournament / CombiningTree / StaticTree / Dissemination / Butterfly)
2. 開 `event.go` 從 latch 分出 ManualReset / AutoReset / Notify
3. 寫 `future.go` 的 Promise + Future + SharedFuture
4. 開 `once.go` 自刻 `sync.Once`

→ 結果:`syncx` 變完整教學庫

### 路線 Q:**深入 lock-free + runtime**(1–2 個月)
1. 升級 `queue/` 加 Michael-Scott unbounded queue
2. 寫 `hazard/` + `reclamation/` 套回 MS queue / Treiber stack
3. 寫 `deque/` Chase-Lev,接 `pattern/` 做 work-stealing pool
4. 自刻 `sync.Pool` per-P cache

→ 結果:能讀 `runtime/proc.go`

### 路線 R:**訓練 concurrency 思維**(每週 1–2 題,長期)
1. `excercise/` 每週一題(Philosophers → Smokers → H2O…)
2. `parallel/` 每月一個演算法(Scan → BFS → Sort)
3. `verify/` 寫 linearizability checker,反向測自己的 queue/stack

→ 結果:把「會刻原語」變「會驗證 + 會解問題」

### 路線 S:**進階 paradigm**(看興趣)
- `scope/` — Rust async 模型
- `actor/` — Erlang / Akka 模型
- `clock/` + `crdt/` — distributed system 入門
- `pattern/` — POSA2 工業架構
- `ratelimit/` — 系統設計面試題

---

## 5. 推薦下一步

**走路線 P 第 1 步**(補完 Barriers)— stub 已經在 [barriers.go](barriers.go),補完 + 跑 benchmark,直接寫一篇 blog,對 AI infra signal 最強。詳見 [TODO_BARRIERS.md](TODO_BARRIERS.md) 的 D / H 兩節。

若想換口味,**`scope/` 是最小可獨立做完的新 package**(`CancellationToken` + `Nursery` 兩個檔案就能跑),也是 Rust async 模型的入口 — 詳見 [scope/TODO.md](../scope/TODO.md)。
