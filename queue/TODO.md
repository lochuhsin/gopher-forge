# Queue Family TODO

> Category 抽象: FIFO ordered transfer。Producer 入 tail,consumer 出 head,順序保留。
> Underlying logic: head/tail 上的協調 + concurrency profile 作為 type-level constraint。

## 核心 invariant

- `enqueue(v)` happens-before `dequeue(v)` — FIFO 在 happens-before 關係下保留
- Concurrency profile (SPSC / MPSC / SPMC / MPMC) 是 *type-level* 限制,不是 runtime 屬性
- Bounded vs unbounded 決定背壓策略 (block / return false / grow)
- Profile 退化 → 可以省 atomic / fence (從 seq_cst 到 release/acquire)

## Inventory

| Variant | Profile | Bound | Impl | Status |
|---|---|---|---|---|
| MutexMPMC | MPMC | unbounded | linked + mutex | done |
| MutexMPSC | MPSC | unbounded | linked + mutex | done |
| LockFreeMPMC | MPMC | bounded ring | seq-based CAS | done |
| LockFreeMPSC | MPSC | bounded ring | producer CAS, consumer 獨佔 | done |
| LockFreePaddedMPMC | MPMC | bounded ring | + cache-line padding | done |
| **LockFreeSPSC (Rigtorp)** | SPSC | bounded ring | cached head/tail snapshot | WIP (lockfree_spsc.go 已建立) |
| **LMAX Disruptor** | 1P/NC + barrier | bounded ring | seq + dependency graph | TODO |
| **Michael-Scott unbounded MPMC** | MPMC | unbounded | linked + 2 CAS + helping | TODO |
| Vyukov intrusive MPSC | MPSC | unbounded | linked, 1 XCHG per enq | TODO |
| Wait-free Queue (Kogan-Petrank) | MPMC | unbounded | announcement + helping | TODO |
| Priority Queue (lock-free skip-list) | MPMC | unbounded | Sundell-Tsigas | TODO |

## Variant contracts (TODO 部分)

### LockFreeSPSC (Rigtorp)

**Signature:**
```
type LockFreeSPSC[T] struct {
    head        atomic.Uint64   // 只有 producer 寫 (consumer 也讀)
    cachedTail  uint64          // producer-local snapshot (non-atomic)
    _pad1       cacheLinePad
    tail        atomic.Uint64   // 只有 consumer 寫 (producer 也讀)
    cachedHead  uint64          // consumer-local snapshot (non-atomic)
    _pad2       cacheLinePad
    buf         []T
    mask        uint64
}
```

**Contract:**
- Exactly ONE producer, ONE consumer goroutine。違反前提 = data race
- Enqueue 路徑: 先讀 cachedTail。若 cachedTail 顯示「滿」才 atomic load 真實 tail 更新 cache
- Dequeue 路徑對稱
- Memory ordering: producer 對 head 是 release-store, consumer 對 head 是 acquire-load (反之亦然)

**Key insight:** cached snapshot 把 cross-core load 從每次操作降到 ~1/N 次 — 這是 Rigtorp 跟 LMAX 共同的核心優化。減少 *false coherence traffic*。

**Reference:** Erik Rigtorp "Optimizing a Ring Buffer for Throughput", 362K ops/ms, 133ns p50。boost::lockfree::spsc_queue, folly::ProducerConsumerQueue。

### LMAX Disruptor

**Signature:**
```
type Disruptor[T] struct {
    ring     []T
    cursor   atomic.Uint64           // producer claim sequence
    barriers []*ConsumerBarrier      // 各 consumer group
}
type ConsumerBarrier struct {
    seq  atomic.Uint64
    deps []*ConsumerBarrier  // 必須等的上游 consumer
}
```

**Contract:**
- Producer `Next()` → claim seq → 寫 ring[seq & mask] → publish via cursor.Store
- Consumer 等 `cursor >= mySeq && all dep.seq >= mySeq`
- Consumer **不消費** — slot 由 producer wraparound 覆寫

**Key insight:** consumer 不擁有「取出」這個 op,ring 是 broadcast 不是 transfer。dependency graph 讓多 consumer pipeline (例如 journal → replicate → match) 共用同一個 ring。

**Reference:** LMAX 2011 paper "Disruptor: High performance alternative to bounded queues",Martin Thompson Mechanical Sympathy blog。Coinbase market data 公開引用。

### Michael-Scott Unbounded MPMC

**Signature:**
```
type MSQueue[T] struct {
    head atomic.Pointer[msNode[T]]   // 永遠指 sentinel
    tail atomic.Pointer[msNode[T]]
}
type msNode[T] struct {
    val  T
    next atomic.Pointer[msNode[T]]
}
```

**Contract:**
- Enqueue 兩步: (1) CAS tail.next nil → newNode (2) CAS tail → newNode (可以由任何人代勞)
- Dequeue: CAS head 前進
- **Note:** 需要 `reclamation/` (HP 或 EBR) 才能安全 free 舊節點 — 否則 ABA + UAF

**Key insight:** "helping" 是 lock-free 演算法的核心 — 看到 tail.next != nil 但 tail 沒前進,自己幫前進。沒有單一 thread 的進度依賴。

**Reference:** Michael & Scott 1996 "Simple, Fast, and Practical Non-Blocking and Blocking Concurrent Queue Algorithms"。Java ConcurrentLinkedQueue 基於此。

### Vyukov Intrusive MPSC

**Contract:**
- Producer 一次 atomic XCHG 就完成 enqueue
- Consumer 單執行緒,可暫時看到 "stub" (新 producer 還沒 link 完 next)
- 比 M&S 簡單且快,代價是 *intrusive* (節點 owner 是 caller,不是 queue)

**Reference:** Dmitry Vyukov 1024cores blog "Non-intrusive MPSC node-based queue"。

### Wait-free Queue (Kogan-Petrank 2011)

**Contract:**
- 每個 op 在有限步驟內完成 (與其他 thread 行為無關)
- announcement array + helping protocol
- 學術價值 >> 實用價值 (constant factor 大)

### Priority Queue (Lock-free skip-list)

**Contract:**
- DeleteMin 為主要 op
- Sundell-Tsigas 2003 lock-free skip-list
- 需要 reclamation

## Career signal

- **HFT:** SPSC + LMAX 直接命中。LMAX paper 是 HFT staple
- **AI infra:** batch scheduler 用 SPSC / MPSC,pipeline 用 Disruptor 風格
- **Crypto CEX:** market data plane = LMAX style (Coinbase 公開引用)
- **面試 high-frequency:** "implement SPSC with correct memory ordering"

## Recommended order

1. LockFreeSPSC (你已開檔)
2. LMAX Disruptor (寫完就有 blog 素材)
3. Michael-Scott (要先有 hazard pointer)
4. Vyukov MPSC (cheap win,intrusive 風格教學)
5. Wait-free / Priority Queue (純學術,選做)

## Dependencies

- M&S queue → `reclamation/` (HP 或 EBR)
- LMAX Disruptor 不需要 reclamation (ring 自然覆寫)
- 全部 lock-free → `memory/` (release/acquire fence 模式)
