# Lock Family TODO

> Category 抽象: Mutual exclusion — 同一時間只允許一個 holder 進入 critical section。
> Underlying logic: 從 spin (CPU burn) → queue-based (cache-local spin) → sleeping (futex)。Trade CPU vs latency vs fairness。

## 核心 invariant

- Acquire ↔ Release 必須成對 (per goroutine)
- Lock state 的修改必須是 atomic
- 喚醒順序決定 fairness 跟 latency

## Inventory

| Variant | Type | Fair | Status |
|---|---|---|---|
| SpinLock | TAS spinlock | no | done |
| TicketLock | FIFO bakery | yes | done |
| MCSLock | queue-based, local spin | yes | done |
| RWMutexLock | wraps sync.RWMutex | depends | done (wrap only) |
| RCULock | publication protocol | n/a | skeleton (移到 `rcu/` 包,見 `rcu/TODO.md`) |
| MutexLock (futex-based sleeping) | spin-then-sleep, 3-state | no | skeleton — **TODO** |
| **CLHLock** | queue-based, spin on predecessor | yes | TODO |
| **Adaptive Mutex** | spin a few then sleep | no | TODO |
| **Recursive / Reentrant Mutex** | owner-tracked, count-based | depends | TODO |
| **TryLock + Timeout variants** | non-blocking try | n/a | TODO |
| **HemLock** (2021 NUMA-aware) | per-socket queue | yes | TODO (進階) |
| **Read-Preferring RWLock** | readers go through | reader bias | TODO |
| **Write-Preferring RWLock** | new readers wait if writer queued | writer bias | TODO |
| **Fair RWLock (FIFO)** | one queue both R/W | yes | TODO |
| **Seqlock** | optimistic reader, exclusive writer | n/a | TODO |
| **StampedLock** (Java 8) | optimistic read + validate | n/a | TODO |
| **BrLock** (Big-Reader) | per-CPU reader rwlock | reader-heavy | TODO (Linux signal) |

## Variant contracts (TODO 高優先)

### MutexLock (futex-based sleeping mutex)

**Signature:**
```
type MutexLock struct {
    state atomic.Int32  // 0=free, 1=locked_no_waiters, 2=locked_with_waiters
}
```

**Contract:**
- Lock 路徑: spin K 次 retry → 失敗則 CAS 1→2 → 進 futex_wait (Linux) 或 `runtime_SemacquireMutex` (Go)
- Unlock 路徑: state == 2 才需要 wake;state == 1 直接 store 0 不需要 syscall
- 3-state 設計省 70%+ syscall

**Key insight:** 為什麼是 3 state 不是 2 state — 在 state==1 (no waiters) 時 unlock 不需要 syscall;只在 state==2 才 wake。Drepper paper 的核心 trick。

**Reference:** Ulrich Drepper "Futexes Are Tricky"。Linux glibc `pthread_mutex_t` 內部就是這個。

### CLH Lock

**Signature:**
```
type CLHLock struct {
    tail atomic.Pointer[clhNode]
}
type clhNode struct {
    locked atomic.Bool  // spin 自己的 pred.locked
}
```

**Contract:**
- Lock: 自己造 node, `pred = tail.Swap(myNode)`, spin `pred.locked`
- Unlock: `myNode.locked.Store(false)` (我已釋放)
- 跟 MCS 對比: spin 在 *predecessor* 的 flag 上 (不是自己的)

**Key insight:** 在 cache-coherent 系統上 CLH 跟 MCS 等價。在 NCC-NUMA 上 MCS 較好 (spin 在 local flag,CLH 在 predecessor 的 cache line 可能在遠端 socket)。Java AQS (AbstractQueuedSynchronizer) 用 CLH 變體。

**Reference:** Craig 1993, Magnussen-Landin-Hagersten 1994。

### Seqlock

**Signature:**
```
type Seqlock struct {
    seq atomic.Uint64  // even = unlocked, odd = locked
    // protected data 在外面
}
func (s *Seqlock) ReadBegin() uint64               // load even seq
func (s *Seqlock) ReadValidate(start uint64) bool  // re-load, check equal & even
func (s *Seqlock) WriteLock()                      // spin CAS even → odd
func (s *Seqlock) WriteUnlock()                    // seq++ (odd → next even)
```

**Contract:**
- Reader:
  ```
  for {
    s = ReadBegin()
    if s & 1 != 0 { continue }  // writer in progress
    // read data
    if ReadValidate(s) { break }
  }
  ```
- Writer 之間互斥 (用另一個 mutex 或 spinlock);seq 自己作 happens-before
- Reader 不 block writer;writer 不 block reader (reader 可能重讀)

**Key insight:** Reader 失敗就重讀,而不是 block。對 "writer rare, reader hot, data small" 完美 — Linux gettimeofday 就用這個。Reader 沒有 cache invalidation overhead (除非真的 writer 寫)。

**Reference:** Linux kernel `<linux/seqlock.h>`。Mellor-Crummey & Scott 提出。

### StampedLock (Java 8)

**Signature:**
```
type StampedLock struct {
    state atomic.Uint64  // 64-bit packed (mode, version)
}
type Stamp uint64

func (l *StampedLock) TryOptimisticRead() Stamp
func (l *StampedLock) Validate(s Stamp) bool
func (l *StampedLock) ReadLock() Stamp          // fallback to pessimistic read
func (l *StampedLock) WriteLock() Stamp
func (l *StampedLock) Unlock(s Stamp)
```

**Contract:**
- Optimistic read: load stamp → 讀 data → validate stamp。Fail 則 fallback to ReadLock
- Pessimistic read: 跟一般 RWLock 一樣 lock-based read
- Write: 互斥,bump version

**Key insight:** 把 seqlock 的「fail 就重讀」改成「fail 就 fallback to read lock」 — 對寫頻繁的場景更穩。Optimistic read 成本 ≈ seqlock,fallback 成本 ≈ ReadLock。

**Reference:** Doug Lea, Java 8 `java.util.concurrent.locks.StampedLock`。

### BrLock (Big-Reader)

**Signature:**
```
type BrLock struct {
    perCPU []rwlock  // 一個 CPU 一個 rwlock
}
func (b *BrLock) RLock()    // lock own CPU's rwlock (read mode)
func (b *BrLock) RUnlock()
func (b *BrLock) WLock()    // lock all CPUs' rwlocks (write mode)
func (b *BrLock) WUnlock()
```

**Contract:**
- Reader: 鎖自己 CPU 的 rwlock → 沒有 cross-CPU coherence 流量
- Writer: 鎖所有 N 個 rwlock → 很慢
- 給 *reader 極度 hot, writer 極稀有* 用

**Key insight:** 用空間換時間 — N 倍 rwlock,但 reader 不再競爭。Linux 過去用在 vfsmount,後被 lglock / percpu rwsem 替換。

**Reference:** Linux kernel `brlock` (已 retired)。

### Adaptive Mutex

**Contract:**
- Lock: 先 spin K 次 (K 動態調整);K 次內 holder 還沒釋放 → futex_wait
- K 隨歷史競爭情況調整:競爭多就 spin 少 (直接睡),holder 通常快放就 spin 多

**Reference:** Solaris adaptive mutex, Linux PI futex variant, FreeBSD adaptive。

### Recursive / Reentrant Mutex

**Signature:**
```
type ReentrantMutex struct {
    owner atomic.Int64  // goid or thread id
    count int32         // 同 owner 重入次數
    inner MutexLock     // 真實 lock
}
```

**Contract:**
- Lock: if owner == self → count++; else 鎖 inner,設 owner = self, count = 1
- Unlock: count--;count == 0 才 release inner

**Note:** Go 沒有 stable goid API。可以用 LockOSThread + thread id,但破壞 goroutine 抽象。實用價值低於教學價值。

### Read/Write 偏好變體 (詳細)

#### Read-Preferring (簡單,易 starve writer)
- 有 reader 在,新 writer 等
- 有 writer 在,新 reader 也等
- Writer starvation 風險

#### Write-Preferring (Java ReentrantReadWriteLock 預設)
- Writer 標 "waiting" → 新 reader 在 waiter writer drain 之前不 enter
- 避免 writer starvation

#### Fair (FIFO)
- 用一個 queue 排所有 R/W
- 不會 starve,但 throughput 較低 (連續 reader 不能 batch enter)

## 跨語言對照

| Variant | C++ | Java | Rust | Go |
|---|---|---|---|---|
| Spin | `std::atomic_flag` | `AtomicBoolean.spin` | `parking_lot::Mutex` spin | `atomic.Bool` loop (你有) |
| Sleeping mutex | `std::mutex` | `ReentrantLock` | `std::sync::Mutex` | `sync.Mutex` |
| RW | `std::shared_mutex` | `ReentrantReadWriteLock` | `std::sync::RwLock` | `sync.RWMutex` |
| Seqlock | n/a (kernel 自刻) | n/a | `seqlock` crate | 自刻 |
| StampedLock | n/a | `StampedLock` (Java 8) | n/a | n/a |
| Reentrant | `std::recursive_mutex` | `ReentrantLock` | n/a (有意不提供) | n/a |
| Brlock | n/a | n/a | n/a | n/a (Linux 限定) |

## Career signal

- SpinLock + Ticket + MCS 已做 → HFT baseline 達成
- **補 futex MutexLock + CLH + Seqlock 後 = lock family 完整** → Linux kernel / HFT signal 強
- StampedLock 是 Java 面試題 (Doug Lea 設計)
- BrLock 是 Linux kernel 路線的 signal
- 寫 blog 比較 spin / ticket / MCS / futex 在不同 contention 跟 critical section 長度下的曲線 = HFT 經典題目

## Recommended order

1. **MutexLock (futex / semacquire-based)** — 填完 skeleton,lock family 才算完整
2. **CLH Lock** — MCS 的 cousin,Java AQS 對照
3. **Seqlock** — Linux kernel signal + 解鎖 RWLock 思路
4. **StampedLock** — Seqlock 的延伸 + Java 概念
5. **RWLock 三個偏好變體** — 自刻 (不要 wrap)
6. **Adaptive Mutex** — 把 spin + futex 接起來
7. (進階) HemLock, BrLock, Recursive

## Dependencies

- Sleeping variants → `park/` (futex / semacquire wrapper)
- 全部 → `memory/` (release/acquire ordering)
- RCULock 已搬到 `rcu/`
