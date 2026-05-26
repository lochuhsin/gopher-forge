# Channel & Rendezvous Family TODO

> Category 抽象: Typed message passing — producer 發訊息,consumer 收訊息。
> 包含 *rendezvous* (buffer=0) 跟 *buffered* (buffer>0) 兩種 — semantic 是 qualitatively 不同。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **later(6mo+)** | **2.60** | 6.5/10 | 0.6(最低,Go-specific) | 1.5 週 | T0(全球)但 R_corr 最低 |

> 全球 T0,但 channel 是 Go 特有,Rust transfer 差 → Dubai 排後。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- Send 在 channel 未滿時不阻塞;滿時 block 或 fail
- Receive 在 channel 非空時不阻塞;空時 block 或 fail
- **Buffer 0 = rendezvous:** send 必須等到有 receiver pickup 才返回 (synchronization)
- **Buffer N>0 = decoupling:** send 不等 receiver,只要 buffer 有空 (buffering)

## Inventory

| Variant | 來源 | 觀念 | Status |
|---|---|---|---|
| Go built-in `chan` | runtime | rendezvous + buffer + select | done (使用層) |
| **OrderedChannel utility** | this repo | re-order via ring buffer | done |
| **UnorderedChannel utility** | this repo | drain extras via buffer | done |
| **Rendezvous (SynchronousQueue)** | Scherer-Lea-Scott 2005 | dual-stack lock-free handoff | TODO |
| **Broadcast channel** | tokio::broadcast | 1 sender, N receivers, slot-based | TODO |
| **Watch channel** | tokio::watch | latest-value-only, multiple receivers | TODO |
| **OneShot channel** | tokio::oneshot | single send, single receive | TODO |
| **Bounded MPMC channel** | wrap LockFreeMPMC | typed wrapper | TODO |
| **Select primitive** | own impl | 多 channel 等待 | TODO (進階) |

## Variant contracts (TODO)

### Rendezvous (SynchronousQueue 風格,Lock-free)

**Signature:**
```
type Rendezvous[T any] struct {
    // Dual data structure: stack of pending offers OR stack of pending requests
    stack atomic.Pointer[rdvNode[T]]
}
type nodeKind int32
const (
    offerKind  nodeKind = iota   // producer 等 consumer
    requestKind                   // consumer 等 producer
)
type rdvNode[T any] struct {
    kind   nodeKind
    val    T
    match  atomic.Pointer[rdvNode[T]]  // CAS-target for handoff
    next   *rdvNode[T]
}
func (r *Rendezvous[T]) Offer(v T)    // block until taken
func (r *Rendezvous[T]) Take() T      // block until offered
```

**Contract:**
- Producer 推 offer node, spin / park 等 match
- Consumer CAS pop offer node,設 match 指向自己 → wake producer
- 反向同樣 — consumer 推 request node 等 producer match
- 「stack」會切換 mode — 同 mode 累積,opposite mode 直接 match

**Key insight:** 跟 size-0 channel 同樣語意,但 lock-free。Scherer-Lea-Scott 設計用 *dual stack* 讓 producer 跟 consumer 共用同一個資料結構 — 一個 stack 同時表示「等 consumer 的 producer 列」或「等 producer 的 consumer 列」(因為兩者不可能同時存在)。

**Reference:** Scherer, Lea, Scott 2006 "Scalable Synchronous Queues", PPoPP。Java `SynchronousQueue` 是這個演算法。

### Broadcast Channel (1 producer → N consumer)

**Signature:**
```
type Broadcast[T any] struct {
    ring        []slot[T]
    seq         atomic.Uint64        // producer cursor
    subscribers sync.Map              // id → *Receiver[T]
    mask        uint64
}
type Receiver[T any] struct {
    bcast *Broadcast[T]
    head  atomic.Uint64
}
func (b *Broadcast[T]) Send(v T)
func (b *Broadcast[T]) Subscribe() *Receiver[T]
func (r *Receiver[T]) Recv() (T, error)   // error if lagged & overwritten
```

**Contract:**
- Producer 寫 ring[seq & mask],seq++
- 每 receiver 有自己 head;讀到 head < seq 為止
- 慢 receiver 落後超過 ring size → `RecvLagged` error (跳到當前 seq)

**Key insight:** 跟 buffered channel 對比 — broadcast 沒有「消費」概念,每個 receiver 看完整 stream;但 slow receiver 可能 drop 訊息。設計選擇:drop oldest (LMAX-style) vs lag-error (tokio-style)。

**Reference:** `tokio::sync::broadcast`。

### Watch Channel (latest-value-only)

**Signature:**
```
type Watch[T any] struct {
    version atomic.Uint64
    value   atomic.Pointer[T]
    notify  *Notify  // (from latch_TODO.md)
}
type Watcher[T any] struct {
    watch *Watch[T]
    seen  uint64
}
func (w *Watch[T]) Send(v T)
func (w *Watch[T]) Subscribe() *Watcher[T]
func (r *Watcher[T]) Changed() <-chan struct{}    // closed on change
func (r *Watcher[T]) Latest() T
```

**Contract:**
- Producer 覆寫 value,version++,notify all watchers
- Watcher 等到 version > seen → Latest() 拿最新值
- *不保留歷史* — slow watcher 跳過中間值,只看到最新

**Key insight:** 跟 broadcast 對比 — watch 是 "snapshot" semantic (永遠最新),broadcast 是 "stream" semantic (每個都要)。對 config reload, leader election, service health 完美。

**Reference:** `tokio::sync::watch`。

### OneShot Channel

**Signature:**
```
type OneShot[T any] struct {
    state atomic.Uint32  // 0=pending, 1=sent, 2=closed
    val   T
    done  chan struct{}
}
func NewOneShot[T any]() (*OneShotSender[T], *OneShotReceiver[T])
type OneShotSender[T any] struct { os *OneShot[T] }
type OneShotReceiver[T any] struct { os *OneShot[T] }
func (s *OneShotSender[T]) Send(v T) bool       // panic / error if second send
func (r *OneShotReceiver[T]) Recv() (T, bool)   // bool false if sender dropped
```

**Contract:**
- Send 一次,Receive 一次
- 等價於 `Promise<T>` 但 enforced single-receive (move semantic in Rust)
- Sender drop without Send → Receiver 收 closed (跟 Go chan close 同)

**Key insight:** OneShot ≈ Promise/Future。Rust tokio 把這個獨立出來給 channel 風格的使用,API 對齊 mpsc。

**Reference:** `tokio::sync::oneshot`。

### Bounded MPMC Channel (typed wrapper)

**Signature:**
```
type BoundedMPMC[T any] struct {
    q queue.LockFreeMPMC[T]  // 你已實作
}
func (c *BoundedMPMC[T]) Send(v T)
func (c *BoundedMPMC[T]) Recv() T
func (c *BoundedMPMC[T]) TrySend(v T) bool
func (c *BoundedMPMC[T]) TryRecv() (T, bool)
```

**Contract:**
- 是 `queue/LockFreeMPMC` 的 typed channel wrapper
- Send 滿了 block (用 cond / park)
- Recv 空了 block

**Use case:** 比 Go built-in chan 快 (你的 LockFreeMPMC 不需要 mutex)。

### Select Primitive (自刻 Go's select)

**Signature:**
```
type SelectCase[T any] struct {
    ch  *Channel[T]
    op  SelectOp   // Send or Recv
    val T          // for Send
}
type SelectOp int
const (SelectSend SelectOp = iota; SelectRecv)
func Select(cases ...SelectCase) (chosenIdx int, val any, ok bool)
```

**Contract:**
- 同時等待多 channel 任一 ready
- 多個同時 ready → random choice (公平性)
- 所有 channel 都不 ready → block until 任一 ready
- 取消的 case 必須從其他 channel 的 waiter list 清掉

**Key insight:** 實作上要 atomic enqueue self 到 *所有* channel 的 waiter list;當任一 channel ready,CAS-claim 那個 op 並 remove self from 其他 channel。這是 Go runtime `selectgo` 的核心難點。

**Reference:** Go runtime `src/runtime/select.go` `selectgo` 函式。

## 跨語言對照

| Variant | Go | Rust (tokio) | Java | C++ | Erlang |
|---|---|---|---|---|---|
| Unbuffered (rendezvous) | `make(chan T)` | (none direct);`oneshot` 近 | `SynchronousQueue` | n/a | `!` 是 mailbox |
| Buffered | `make(chan T, N)` | `mpsc::channel(N)`, `crossbeam::bounded` | `LinkedBlockingQueue(N)` | n/a | mailbox 無 bound |
| Broadcast | n/a (自刻) | `broadcast::channel` | n/a | n/a | `gen_event` |
| Watch | n/a | `watch::channel` | n/a | n/a | n/a |
| OneShot | `chan T` of size 1 | `oneshot::channel` | `CompletableFuture` | `std::promise` | n/a |
| Select | `select { ... }` | `tokio::select!` | n/a (用 cond) | n/a | `receive ... end` |

## Use case

- Rendezvous: synchronous handoff (thread pool task submission)
- Broadcast: pub/sub within process, market data fan-out
- Watch: config reload, leader change notification
- OneShot: RPC reply, future-equivalent
- Bounded MPMC: 高性能 worker pool input

## Career signal

- 自刻 SynchronousQueue + 解釋 dual stack 演算法 = Java AQS / 系統知識
- 自刻 Go select = Go runtime internals 強 signal (Russ Cox 設計的核心)
- 自刻 tokio broadcast / watch = Rust async infra signal
- 寫 blog "Tokio sync 全家族對應 Go 怎麼做" = 跨語言 senior signal

## Recommended order

1. **OneShot** (跟 future_TODO.md 同步做)
2. **Bounded MPMC wrapper** (現有 queue 直接用上)
3. **Broadcast** (pub/sub 範式,實用)
4. **Watch** (broadcast 的簡化版,概念對立)
5. **Rendezvous (lock-free SynchronousQueue)** (進階,Scherer-Lea-Scott)
6. **Select primitive** (Go runtime 對照,最難)

## Dependencies

- → `park/` (block when not ready)
- → `memory/` (release-acquire on slot publication)
- → `queue/` (Bounded MPMC 用現有 LockFreeMPMC)
- → `syncx/latch.go` Notify (Watch 用 Notify 喚醒 watcher)
- 跟 `syncx/future_TODO.md` OneShot 同源

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 0(Composite ★4.2)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★4/5 | Advanced (Go-specific) | Pipeline concurrency: feed handler → strategy → risk → OMS = channel pipeline; Disruptor-style bounded MPMC is HFT standard |
| **Crypto** | ★4/5 | Required | Coinbase/Kraken Go-heavy; Narwhal network layer uses actor-like message passing; block subscription management |
| **AI Infra** | ★4/5 | Required | vLLM MultiProcExecutor uses broadcast message queues; request cancellation pipeline via scope + channel |
| **FAANG** | ★5/5 | Required | Go channels = primary concurrency abstraction tested everywhere; pipeline/fan-out/fan-in is senior Go coding question |
| **Dubai** | ★4/5 | Required | Go-heavy market; Bybit/Binance use channel-based service coordination |
| **Composite** | **★4.2/5.0** | **Tier 0** | — |

### 必要(Required for senior infra interviews)

> 在 **≥2 個 vertical** 被列為 Required,或 composite ≥ 3.4。

- **Fan-out / Fan-in pattern** — Go senior 標準題;vLLM worker dispatch = fan-out;FAANG parallel HTTP downloader
  - Evidence: [FAANG research](../docs/research/faang.md) — ★★★★★ at FAANG; pipeline pattern appears in Google/Cloudflare interviews
- **Pipeline with backpressure** — bounded channel + cancellation;HFT feed handler pipeline 直接對應
  - Evidence: HFT pipeline is feed handler → normalizer → strategy → risk → OMS; each stage is a pipeline channel
- **Channel as semaphore** (`chan struct{}`) — 解釋 why it works = Go internals depth;`chan struct{}` vs `sync.WaitGroup` tradeoff
  - Evidence: FAANG research — "channel as semaphore is a test of Go internals depth"
- **Unbuffered vs buffered channel deadlock 分析** — Go 面試必考;rendezvous semantics
  - Evidence: Cloudflare/Datadog/Uber Go senior interview: "tricky Golang interview questions" series on DEV Community

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator,或 composite < 3.4 但有特定 vertical 看重。

- **Priority channel** — 多優先級路由;AI Infra 推理 scheduler VIP vs standard
  - Best for: AI Infra (LLM serving admission control)
- **Bounded MPMC channel wrapper** — 高性能 worker pool input;比 Go built-in chan 快
  - Best for: HFT / Crypto (market data fan-out needs lock-free MPMC)
- **Broadcast channel (1:N)** — market data pub/sub within process;Disruptor 語義
  - Best for: HFT (1 producer → N consumer pattern); Crypto CEX market data
- **Watch channel (latest-value-only)** — config reload, leader election;Rust tokio 對照
  - Best for: AI Infra (model weight hot-reload); FAANG (service mesh config update)
- **Rendezvous (SynchronousQueue lock-free)** — Java SynchronousQueue 對照;dual-stack algorithm
  - Best for: FAANG Java interviews (SynchronousQueue = thread pool handoff)
- **Select primitive 自刻** — Go runtime `selectgo` 對照;最難;Go runtime internals signal
  - Best for: FAANG (demonstrates Go runtime depth)

### Recommended Order(本 package 內部)

1. OneShot(接 Future TODO)
2. Bounded MPMC wrapper(現有 queue 直接用)
3. Broadcast(pub/sub 範式)
4. Watch(broadcast 簡化版)
5. Rendezvous lock-free SynchronousQueue(進階)
6. Select primitive(最難,Go runtime 對照)

### 對應的 Blog 題材(若想寫)

- "Go channel pattern 全攻略:fan-in/out, backpressure, select deadlock 分析"
- "Tokio sync primitives 的 Go 等價:watch/broadcast/oneshot 如何自刻"
