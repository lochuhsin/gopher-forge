# Future / Promise Family TODO

> Category 抽象: 單一 write-once / read-many value 的非同步交付。
> **Note:** 你的 `future.go` 目前是空的,整個 category 待補。
> Underlying logic: state machine `Pending → Set → Read`,加上 waiter 喚醒。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **B(3mo·deep showcase)★** | **3.75** | 7.5/10 | 1.0 `Future` trait+tokio | 2.0 週 | T1 |

> **Rust async 核心** — 自刻 Future 是 Rust infra 入門信號最強。從零做。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- Set 只能成功一次 (write-once);多次 Set 第二次起 fail 或 panic
- Set 之前 Get 必須 block (或 ready=false)
- Set 之後 Get 必須立即返回 value (publication safety)
- Get 必須是 multi-reader safe — 多個 caller 可以同時 Get 同一個 value

## Inventory

| Variant | 寫端 | 模型 | Status |
|---|---|---|---|
| **Future (eager)** | producer push | producer 主動寫 | TODO |
| **Promise (split write end)** | promise.Set | promise.Set → future.Get | TODO |
| **CompletableFuture (composable)** | .Then / .Map / .Combine | callback chain / monad | TODO |
| **PackagedTask** | wrap fn + run | fn 結果自動寫進 future | TODO |
| **Lazy / OnceCell** | first reader computes | memoize on first Get | TODO (與 `memory/OnceCell` 看分工) |
| **SharedFuture** | multi-read by-value | 同一個 value 多人讀 | TODO |
| **CancellableFuture** | future-level cancel | cancel propagation | TODO |

## Variant contracts (TODO)

### Future + Promise (Split)

**Signature:**
```
type Promise[T any] struct {
    state atomic.Uint32  // 0=pending, 1=set
    val   T
    err   error
    done  chan struct{}  // closed on Set
}
type Future[T any] struct { p *Promise[T] }

func NewPromise[T any]() (*Promise[T], *Future[T])
func (p *Promise[T]) Set(v T) bool          // ok if first
func (p *Promise[T]) SetError(err error) bool
func (f *Future[T]) Get() (T, error)        // block until set
func (f *Future[T]) Poll() (T, error, bool) // non-blocking
func (f *Future[T]) GetContext(ctx context.Context) (T, error)
```

**Contract:**
- Set: CAS state 0→1。Success 才寫 val、err、close(done)
- Get: `<-done` (block) → 讀 val/err
- *Publication safety:* val 必須在 `close(done)` 之前寫完 (release-before-publish)
- Multiple Set: 第二次起 CAS fail,return false (或 panic,設計選擇)

**Key insight:** 三相 state machine:
1. *write-once invariant* 用 CAS 守 (set state 0→1)
2. *set 之前的 waiter* 用 channel close 喚醒 (Go-idiomatic)
3. *set 之後到的 reader* fast-path 不 block — Get 先 check state,if set 直接讀 val

這個三相是所有 lazy initialization 的核心。`sync.Once` 是 Promise 的退化 (value 是 side-effect)。

**Reference:** C++ `std::promise / std::future`, Java `Future` / `CompletableFuture`。

### CompletableFuture (composable)

**Signature:**
```
type Future[T any] struct { ... }
func (f *Future[T]) Then(fn func(T)) *Future[struct{}]                       // 副作用 callback
func Map[T, U any](f *Future[T], fn func(T) (U, error)) *Future[U]            // functor
func ThenAsync[T, U any](f *Future[T], fn func(T) *Future[U]) *Future[U]       // monadic bind
func Combine[A, B, C any](a *Future[A], b *Future[B], fn func(A, B) C) *Future[C]
func WhenAll[T any](fs ...*Future[T]) *Future[[]T]
func WhenAny[T any](fs ...*Future[T]) *Future[T]
```

**Contract:**
- `Then` 註冊 callback — set 已發生則立即 schedule fn;set 未發生則 enqueue 等
- `Map` 是 functor: `Future[T] → Future[U]`
- `ThenAsync` 是 monad bind: `Future[T] → (T → Future[U]) → Future[U]`
- Callback registration 是 thread-safe — 同時 .Then 跟 .Set 不能漏 callback

**Key insight:** Future 是 monad — 跟 `Option` / `Result` 同形。`.then` 是 monadic operation。這是 functional concurrency 的入門。

實作上 callback 的 thread-safe 註冊是核心難點:必須 CAS state from `pending-no-callbacks` 到 `pending-with-callbacks`,Set 時 atomic 把 list flip 出來執行。

**Reference:** Java 8 `CompletableFuture` (Doug Lea), Scala `Future`, JS `Promise`。

### PackagedTask

**Signature:**
```
type PackagedTask[T any] struct {
    fn      func() (T, error)
    promise *Promise[T]
}
func NewPackagedTask[T any](fn func() (T, error)) *PackagedTask[T]
func (t *PackagedTask[T]) Run()              // 執行 fn,結果 set 進 promise
func (t *PackagedTask[T]) Future() *Future[T]
```

**Contract:**
- Run() 執行 fn,把結果 set 進 promise
- 可以 schedule 到 thread pool / goroutine 後再 Future().Get()
- 對應 C++ `std::packaged_task`

**Use case:** 把 sync function 變成 async — schedule 到 executor,caller 拿 future。

### Lazy / OnceCell

**Signature:**
```
type OnceCell[T any] struct {
    state atomic.Uint32  // 0=empty, 1=initing, 2=done
    val   T
    mu    sync.Mutex
}
func (c *OnceCell[T]) GetOrInit(init func() T) T
```

**Contract:**
- First reader 拿 mu, double-check, run init, release-store state=2
- Subsequent readers fast-path: acquire-load state==2 → return val

**Note:** 跟 `memory/` package 的 OnceCell 是同一個東西。建議分工:
- `memory/OnceCell` — 教學版,重點在 release/acquire pairing 的解說
- `syncx/future.go` Lazy — production 版,接 Future API

### CancellableFuture

**Signature:**
```
func (f *Future[T]) Cancel() bool
func (f *Future[T]) IsCancelled() bool
type Future[T] struct {
    ...
    state atomic.Uint32  // 0=pending, 1=set, 2=cancelled
}
```

**Contract:**
- Cancel 嘗試 CAS state pending → cancelled
- 已 set 的 future cancel fail
- 上游 cancel 要 propagate (Rust async 整套靠這個 — drop = cancel)
- Producer 應該偶爾 check IsCancelled (cooperative cancellation)

**Reference:** Rust async cancellation model (drop = cancel), JS `AbortController`。

### SharedFuture

**Contract:**
- 多 reader 同時 Get 同一個 value (大部分 future 設計都允許,explicit type 強調這點)
- C++ `std::future` 只能單 reader (move-only);`std::shared_future` 才能 multi-read

## 跨語言對照

| 語言 | API |
|---|---|
| C++ | `std::future`, `std::promise`, `std::packaged_task`, `std::shared_future`, `std::async` |
| Java | `Future`, `CompletableFuture` (composable, Java 8+), `FutureTask` |
| Scala | `scala.concurrent.Future` / `Promise` (composable from day 1) |
| JS | `Promise` (built-in composable) |
| Rust | `std::future::Future` trait + waker model, `tokio::sync::oneshot` |
| Go | 沒有原生 (channel(struct, 1) 是 minimal form) |
| C# / .NET | `Task<T>` / `TaskCompletionSource<T>` (CompletableFuture 對應) |

## Use case

- Async I/O completion
- Parallel computation result
- Pipeline stage hand-off (HFT)
- Lazy initialization (DB connection, config load)
- RPC reply (caller 拿 future,callee 拿 promise)

## Career signal

- **Rust async runtime (tokio) 整套建立在 Future trait** — 自刻 future 是 Rust infra 入門
- C++ HFT 用 `std::promise` 做 pipeline stage handoff
- Java HotSpot biased locking 撤銷流程用 safepoint promise
- 寫 monadic `Map` / `ThenAsync` 能對接 functional programming → ML infra / 學術派 signal
- JS / TypeScript Promise 是 web infra 必備

## Recommended order

1. **Promise + Future split** (核心,write-once 三相 state machine)
2. **PackagedTask** (簡單包裝,證明可用)
3. **CompletableFuture / Then / Map** (composable,functional 風味)
4. **CancellableFuture** (cancellation propagation,Rust async 對照)
5. **Lazy / OnceCell** (跟 `memory/` 分工)
6. (選做) SharedFuture,WhenAll / WhenAny combinator

## Dependencies

- → `park/` (Get 必須 park 不能 spin;`runtime_Semacquire` 或 channel close)
- → `memory/` (Set 是 release, Get 是 acquire)
- 跟 `syncx/latch_TODO.md` 概念近 — `CountDownLatch` 是 N→0 latch,Future 是 set-value latch
- `ThenAsync` 內部需要 executor → 跟 `deque/` work-stealing 對接

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 1(Composite ★3.4)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★3/5 | Required (Two Sigma) | Citadel: "async programming with sender/receiver"; Two Sigma Rust async framework uses Future-based patterns |
| **Crypto** | ★3/5 | Required | Chainlink OCR: async oracle aggregation via Future; async Rust Futures in Helius/Kraken backend |
| **AI Infra** | ★4/5 | Advanced | Ray remote function calls return Futures; vLLM async engine returns futures for request completion; Pathways TPU dispatch uses future-based async dataflow |
| **FAANG** | ★4/5 | Required | Java `CompletableFuture` is Amazon SDE3 prep tool; goroutine + channel as Future = senior Go question |
| **Dubai** | ★3/5 | Required | BNPL async composition (Tabby/Tamara); async service coordination |
| **Composite** | **★3.4/5.0** | **Tier 1** | — |

### 必要(Required for senior infra interviews)

> 在 **≥2 個 vertical** 被列為 Required,或 composite ≥ 3.4。

- **Promise + Future split** — write-once 三相 state machine;基礎 async programming pattern
  - Evidence: [Citadel blog](https://www.citadelsecurities.com/careers/career-perspectives/technical-spotlight-async-programming-with-sender-receiver/) — "async programming with sender/receiver" for concurrent systems
- **SharedFuture (multi-reader)** — C++ `std::shared_future` 對照;Ray remote return values = SharedFuture
  - Evidence: [FAANG research](../docs/research/faang.md) — "goroutine + channel as Future is senior Go interview question"
- **PackagedTask** — 把 sync function 轉 async;thread pool / goroutine 接 Future 結果
  - Evidence: Java `FutureTask` is FAANG Java interview prerequisite; maps to Go goroutine + channel pattern

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator,或 composite < 3.4 但有特定 vertical 看重。

- **CompletableFuture style (Then/Map/WhenAll/WhenAny)** — AI Infra 最高信號;Pathways async dataflow graph edges
  - Best for: AI Infra (Ray, Pathways, vLLM); FAANG (Java CompletableFuture cited in Amazon SDE3 guide)
- **CancellableFuture** — Rust async cancellation 模型 (drop = cancel);cooperative cancellation
  - Best for: Crypto (Rust-heavy shops: Helius, Jito, Kraken); AI Infra (request preemption)

### Recommended Order(本 package 內部)

1. Promise + Future split(核心三相)
2. PackagedTask(簡單包裝,可用)
3. CompletableFuture / Then / Map(composable)
4. CancellableFuture(Rust async 對照)
5. SharedFuture + WhenAll / WhenAny combinator

### 對應的 Blog 題材(若想寫)

- "Go Future/Promise:從 channel 到 CompletableFuture 風格的 monadic 組合"
- "Ray remote 的底層原語:Future + Actor = 分布式計算圖"
