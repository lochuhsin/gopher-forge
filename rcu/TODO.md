# RCU (Read-Copy-Update) Family TODO

> Category 抽象: Reader 零成本 (沒有 atomic, 沒有 fence on x86),Writer 走 copy-on-write + 等所有 in-flight reader 結束 (grace period) 才 free 舊版本。
> Underlying logic: 把同步成本從 reader 轉嫁到 writer + reclamation 路徑。RCU **不是 lock**,是 publication protocol。

## 核心 invariant

- Reader: `rcu_read_lock()` ... `rcu_dereference(ptr)` ... `rcu_read_unlock()` — **沒有任何 atomic on x86**
- Writer: `new = copy(old); modify(new); rcu_assign_pointer(slot, new); synchronize_rcu(); free(old)`
- `synchronize_rcu()` 等所有當前 in-flight reader 完成 — *不等新 reader*

`rcu_assign_pointer` = release store;`rcu_dereference` = consume/acquire load。整個 RCU 是 release-acquire memory model 的最純粹示範。

## Inventory

| Variant | Grace period 偵測 | Reader cost | Status |
|---|---|---|---|
| **Classic RCU (Linux)** | preempt-disable 期間 + scheduler tick | 0 (just preempt_disable) | TODO (Go 沒對應,跳過或 toy) |
| **Sleepable RCU (SRCU)** | per-domain epoch counter | 1 atomic inc/dec | TODO |
| **Userspace RCU (URCU) - general purpose** | thread-local epoch counter | acquire load + RMW | TODO (Go 主推) |
| **Tasks RCU** | voluntary context switch | 0 | TODO (Go 沒對應) |
| **QSBR-RCU** | reader 主動 periodic quiescence | 0 in critical | TODO (與 `reclamation/qsbr.go` 共生) |

## Variant contracts

### Userspace RCU (URCU) — Go port 首選

**Signature:**
```
type RCU struct {
    globalCounter atomic.Uint64
    threads       sync.Map  // goid -> *threadReg
}
type threadReg struct {
    counter atomic.Uint64  // even = quiescent; odd = in critical section
}

func (r *RCU) Register() *Reader
func (r *RCU) Unregister(reader *Reader)
type Reader struct { reg *threadReg }
func (rd *Reader) ReadLock()
func (rd *Reader) ReadUnlock()

func AssignPointer[T any](slot *atomic.Pointer[T], new *T)  // release store
func Dereference[T any](slot *atomic.Pointer[T]) *T          // acquire load

func (r *RCU) Synchronize()  // 等所有 reader 至少 quiescent 過一輪
```

**Contract:**
- ReadLock: `counter.Add(1)` (從 even → odd)
- ReadUnlock: `counter.Add(1)` (從 odd → even)
- AssignPointer: release-store 新指標
- Synchronize:
  1. snapshot 所有 thread 的 counter
  2. wait until 每個 thread 的 counter 變動過 (表示離開過 odd state) 或本來就 even (沒在 critical)
  3. 此時 retire 過的東西可以 free

**Key insight:** Reader fast path on x86 = 一次 atomic add (counter++)。沒有 lock contention,沒有 cross-core invalidation (counter 是 thread-local cache line)。Writer 慢但 reader 飛快 — 對 read-mostly 完美。

**Reference:** Mathieu Desnoyers, Paul McKenney 2009 "User-Level Implementations of Read-Copy Update", TPDS。userspace-rcu library (liburcu)。

### Sleepable RCU (SRCU)

**Contract:**
- 像 URCU 但允許 reader 在 critical section 內 *sleep* (阻塞 syscall, lock acquire)
- 每個 srcu_struct 有自己的 epoch counter (不共享 global)
- Synchronize 範圍只在這個 srcu_struct
- 代價:reader 要 atomic inc/dec (不是 0 cost)

**Reference:** Paul McKenney 2006 "Sleepable Read-Copy Update"。

### Classic Linux RCU (Go 無對應,僅作教學)

**Contract:**
- Reader: `preempt_disable() / preempt_enable()` — kernel-only
- Grace period: 等每個 CPU 至少 context-switch 過一次 (證明 preempt-enabled 期間都過了)
- Reader 完全 0 cost,但只能在 non-preemptible context

**Note:** Go 是 user space + cooperative goroutine preemption,沒有對應。教學用提及即可。

## Go 特殊性

Go runtime 對 goroutine preemption 是 cooperative (function preamble check + async preempt since 1.14),不是 OS preempt。所以 Linux kernel 風格的「preempt_disable」沒有對應。Go 必須用 **URCU 風格 — explicit ReadLock/Unlock**。

實作上 `goid` 不是 stable identifier (Go runtime 沒公開 goid API),要用 `runtime.LockOSThread` 綁 OS thread 或 `sync.Pool` per-P 槽位來找 thread-local 計數器。

## 跨語言對照

| 語言 / 系統 | RCU 變體 |
|---|---|
| Linux kernel | Classic, SRCU, Tasks RCU 並存 |
| C++26 | `std::rcu_*` (進標準了) |
| Rust | `crossbeam-epoch` (EBR,概念相近但不完全相同) |
| Java | 沒有,GC 直接替代 reclamation,publication 用 `AtomicReference` + `volatile` |
| Go | 沒有原生,要 wrap atomic.Pointer + grace 機制 |
| DPDK | URCU-based 路由表 |

## 對應典型 use case

| Use case | 為什麼用 RCU |
|---|---|
| Linux routing table | 99.99% read, rare update |
| dentry cache | read-heavy + 大量更新但都是少數 entry |
| Crypto L1 (Solana gossip table) | validator 共識資訊讀 >> 寫 |
| Service discovery snapshot | 服務列表偶爾變,大量 read |
| Config reload | 啟動時讀一次 + 偶爾 hot reload |

## Career signal

- Linux kernel 直接命題:routing table / dentry cache / namespace 都用 RCU
- Crypto L1 (Solana validator gossip, Aptos state) 大量 read-mostly + 偶爾 update — RCU pattern 直接適用
- AI infra: routing table for tensor placement / weight sharding (read-heavy)
- 自刻 URCU + 跟 GC-based publication (e.g. atomic.Pointer + GC) 對比 → 寫 blog 證明懂 publication 跟 reclamation 的分離

## Recommended order

1. URCU (Go port) — 主要實作
2. SRCU — URCU 之上加 sleepable
3. QSBR variant — 與 `reclamation/qsbr.go` 共用實作
4. Classic Linux RCU — 文件解釋 + 為什麼 Go 沒對應

## Dependencies

- → `memory/` (release/acquire 配對是 RCU 核心)
- 可選 → `reclamation/qsbr.go` (RCU 內部也可以用 EBR 來實作 grace period 偵測)
- 取代 `syncx/lock.go` 的 RCULock skeleton
