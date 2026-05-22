# Future To-Do — Concurrency / Runtime 深化

從目前 base（Vyukov MPMC、Treiber + elimination stack、channel/mutex semaphore）自然延伸的鍛鍊清單。
難度：★ = 熱身，★★★★★ = 啃論文。

---

## 0. Recommended Path（建議路線）

1. [ ] **Michael-Scott unbounded queue** — 鋪 ABA / 安全回收的 motivation
2. [ ] **Hazard Pointers** — 套回 MS queue + Treiber stack
3. [ ] **MCS Lock** — 把 `MutexSemaphore.waiters` 正式做成 scalable lock
4. [ ] **SPSC wait-free ring → LMAX Disruptor** — 反向走 single-writer 極致
5. [ ] **讀 `runtime/chan.go`** — 把所有東西連回 Go 本身

---

## 1. Concurrent Data Structures（更深的）

- [ ] **Michael-Scott unbounded queue** ★★★
  linked-list MPMC，直面 ABA。tail 推進有 helper pattern。
- [ ] **Hazard Pointers** ★★★★
  解 ABA + 安全回收 node 的工業標準。做完才算「真的」lock-free。
- [ ] **Epoch-Based Reclamation (EBR / QSBR)** ★★★★
  hazard pointer 的對手方案，throughput vs latency 取捨。
- [ ] **Lock-free Skiplist (Fraser / Harris)** ★★★★★
  marked pointer + helping。學 logical-delete pattern。
- [ ] **Lock-free Hash Map (split-ordered list, Shalev-Shavit)** ★★★★★
  lock-free resize 怎麼做？這題是答案。
- [ ] **RCU (Read-Copy-Update)** ★★★★
  reader zero-overhead 的代價是什麼。Linux kernel 主力 pattern。

---

## 2. Synchronization Primitives（從零打造）

- [ ] **Ticket Lock** ★★
  FIFO 公平鎖，暴露 cache-line bouncing 災難。
- [ ] **MCS Lock / CLH Lock** ★★★
  scalable spinlock — waiter spin 在自己的 cache line。你 `MutexSemaphore.waiters` 鏈是這個方向。
- [ ] **Seqlock** ★★★
  optimistic read，writer 不被 reader 擋。Linux jiffies 用這個。
- [ ] **RWMutex 三種變體**（reader-biased / writer-biased / fair）★★★
  為什麼 std `sync.RWMutex` 在重 reader 下還是會塞。
- [ ] **Barrier / CountDownLatch / CyclicBarrier** ★★
  N-goroutine rendezvous，比 semaphore 多一個 phase 概念。
- [ ] **Eventcount (Dekker-style condvar)** ★★★★
  把「等待」從 mutex 解耦的 primitive，Folly 用得很重。
- [ ] **手刻 `sync.Once` / `sync.Map` / `sync.Pool`** ★★★
  fast-path / slow-path 設計範本。
- [ ] **手刻 `sync.WaitGroup`** ★★★
  counter + parker，看 `runtime.Semacquire` 怎麼接。

---

## 3. 協調 / Concurrency Patterns

- [ ] **Singleflight** ★★
  cache stampede 防雪崩，經典 dedup pattern。
- [ ] **Work-Stealing Pool (Chase-Lev deque)** ★★★★
  Go runtime 排程器本人的算法。做完能讀 `runtime/proc.go`。
- [ ] **Rate Limiter 全家桶** ★★★
  token bucket / leaky bucket / sliding window / GCRA — 同需求四種實作。
- [ ] **Circuit Breaker** ★★
  state machine + sliding-window 失敗率，重 atomic 計數。
- [ ] **Actor mailbox** ★★★
  每 actor 一個 SPSC inbox + scheduler，和 channel-based 是兩種世界觀。
- [ ] **Futures / Promises** ★★
  one-shot channel 的泛化，學 chaining / fan-in semantics。
- [ ] **Pipeline with backpressure + cancellation** ★★
  context propagation + bounded buffer 的正確姿勢。
- [ ] **errgroup with semaphore-bounded concurrency** ★
  你 `Semaphore` 的直接 application。

---

## 4. Performance / Memory Specials

- [ ] **SPSC wait-free ring buffer** ★★
  MPMC 拿掉 multi 後能簡化到什麼程度？答案：沒 CAS、只 store/load。
- [ ] **LMAX Disruptor** ★★★★
  sequenced ring + dependency graph，single-writer principle 的極致。
- [ ] **Object Pool with per-P cache** ★★★
  `sync.Pool` 的精神。學 `runtime.procPin()` 和 GMP 對應。
- [ ] **Arena Allocator** ★★★
  把 GC 換成 region-based，看 Go 1.24 `arena` (experimental)。
- [ ] **Atomic Refcount (Arc-like)** ★★★
  drop semantics + 與 GC 共存的 manual reclamation。
- [ ] **NUMA-aware sharded counter / LongAdder** ★★
  為什麼單一 atomic counter 在高 contention 下慘輸。
- [ ] **False-sharing 反向實驗** ★
  故意拿掉 padding，量化 RTT 變化。

---

## 5. Go Runtime / 語言精細

- [ ] **讀 `runtime/chan.go`** ★★★
  hchan + sudog parking 機制。channel-based semaphore 之後最該讀的。
- [ ] **Go Memory Model 驗證測試** ★★★
  寫故意觸發 reorder 的 case，跑 `-race` 看抓得到嗎。
- [ ] **GMP 排程實驗** ★★
  `LockOSThread` / `GOMAXPROCS` / `Gosched` 對 throughput 的影響。
- [ ] **Escape Analysis 對照表** ★★
  `go build -gcflags='-m'` 看每種寫法逃 heap 的條件。
- [ ] **`//go:linkname` + `runtime.Semacquire` 直接呼叫** ★★★★
  偷用 runtime 內部 parker。
- [ ] **手寫一段 amd64 / arm64 assembly atomic** ★★★★
  讀 `sync/atomic/asm_amd64.s`，加一個自己的 primitive。
- [ ] **Custom scheduler in user-space** ★★★★★
  coroutines on `runtime.LockOSThread` workers。

---

## 6. Verification / Tools

- [ ] **跑 `-race` 故意做出會被抓的 bug** — 理解 happens-before 怎麼被偵測。
- [ ] **`pprof.Mutex` + `pprof.Block`** — 對比 mutex vs channel semaphore 的 contention profile。
- [ ] **手寫 deadlock detector** — goroutine wait-for graph + cycle detection。
- [ ] **[`go-deadlock`](https://github.com/sasha-s/go-deadlock) 風格 wrapper** — lock ordering 的工程做法。
