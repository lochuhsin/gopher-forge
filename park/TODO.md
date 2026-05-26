# Park / Unpark Family TODO

> **Building-block category** — 所有 sleeping sync primitive 的底層。Mutex (sleeping), Cond, Semaphore (waiter queue), CountDownLatch 都靠這個。

## 核心 invariant

- 「Atomic check + sleep」是 racy 的關鍵 — 必須是 *單一 syscall* 同時做這兩件事,不然喚醒會丟
- Permit semantic (Java / Linux futex): unpark 預發 permit 不會 accumulate (Java) 或會 count (futex counting)
- Park 必須能被 spuriously wake (interrupt, signal),caller 必須 loop on predicate

## Inventory

| Variant | 來源 | API | Status |
|---|---|---|---|
| **futex** (Linux) | kernel syscall | `FUTEX_WAIT(addr, expected)`, `FUTEX_WAKE(addr, n)` | TODO |
| **WaitOnAddress** (Windows) | kernel | 對應 futex | TODO (cross-platform optional) |
| **ulock** (Darwin) | kernel | macOS 對應 | TODO (cross-platform optional) |
| **LockSupport-style permit** (Java) | jvm | `park()` / `unpark(thread)`, permit-style | TODO |
| **runtime semacquire** (Go) | runtime | `runtime_Semacquire/Semrelease` via linkname | done in `syncx/lock.go` |
| **Parker wrapper** | this repo | 統一抽象,unify futex / semacquire / etc. | TODO |

## Go 特殊情況

Go 的 sleeping primitive 不是 OS thread park,是 *goroutine* park (runtime gopark / goready):

- `gopark(unlockf, lock, reason, traceEv, traceskip)` — 把當前 g 標 waiting + unlock 後讓出
- `goready(gp, traceskip)` — 把 g 加到 P 的 runq

公開 API 只能透過:
- `sync/atomic` + `sync.Cond`
- `//go:linkname` runtime.semacquire / semrelease (你已用)
- channel (本身就是 park-on-empty / wake-on-send 的封裝)

## Variant contracts (TODO)

### 1. Futex-based Sleeping Mutex (Linux only, build tag)

**Signature:**
```
type FutexMutex struct {
    state atomic.Uint32  // 0=unlocked, 1=locked_no_waiters, 2=locked_with_waiters
}
func (m *FutexMutex) Lock()
func (m *FutexMutex) Unlock()
```

**Contract:**
- Lock 路徑:
  1. CAS 0 → 1。Success 直接返回
  2. Spin K 次 retry
  3. CAS 1 → 2 (告訴未來 unlocker 有 waiter)。然後 `futex(FUTEX_WAIT, &state, 2)` 等
  4. 醒來 goto step 1
- Unlock 路徑:
  1. Atomic dec state
  2. 如果舊值是 2 (有 waiter): `futex(FUTEX_WAKE, &state, 1)`

**Key insight:** 為什麼是 3 state 不是 2 state — 因為要在「Unlock 該不該 syscall wake」上節省 syscall。State==1 表示沒有 waiter → unlock 不需要 syscall。這個 optimization 省 70%+ syscalls (Drepper 的數據)。

**Reference:** Ulrich Drepper "Futexes Are Tricky" (2011 revision)。Linux glibc `pthread_mutex_t` 內部就是這個設計。

### 2. Go go:linkname Park/Unpark wrapper

**Signature:**
```
//go:linkname semacquire sync.runtime_Semacquire
func semacquire(s *uint32)
//go:linkname semrelease sync.runtime_Semrelease
func semrelease(s *uint32, handoff bool, skipframes int)
//go:linkname semacquireMutex sync.runtime_SemacquireMutex
func semacquireMutex(s *uint32, lifo bool, skipframes int)

// 包裝成乾淨 API
type Parker struct { sema uint32 }
func (p *Parker) Park()
func (p *Parker) Unpark()
func (p *Parker) UnparkAll()  // 或補一個 broadcast 版本
```

**Contract:**
- 包裝成乾淨 API 給其他 syncx primitive 用
- 補上 FIFO option (`lifo=false` for FIFO)

### 3. Permit-style Parker (Java LockSupport 對照)

**Signature:**
```
type PermitParker struct {
    permit atomic.Uint32  // 0 or 1
    sema   uint32          // 內部 OS-level park
}
func (p *PermitParker) Park()
func (p *PermitParker) Unpark()
```

**Contract:**
- Park: CAS permit 1→0 success 直接返回;0→0 success 則 park on sema
- Unpark: store permit=1; if was 0, semrelease(&sema)
- **Park 之前 unpark 不會丟 permit** (跟 cond var 不同)

**Key insight:** Permit-style 的核心區別:unpark 對沒在 park 的 thread 留 permit (counter 不會超過 1)。Java AQS 用這個避免 wake-before-park race。

**Reference:** Java LockSupport, AQS (AbstractQueuedSynchronizer)。

## 跨語言對照

| 系統 | API | Permit semantics |
|---|---|---|
| Linux | `syscall(SYS_futex, ...)` | counting (FUTEX_WAKE_OP) |
| Windows | `WaitOnAddress` / `WakeByAddressSingle` | counting |
| macOS | `__ulock_wait` / `__ulock_wake` | counting |
| Java | `LockSupport.park / unpark` | 0 or 1 permit |
| Go (internal) | `runtime.gopark` / `goready` | direct g handoff |
| Go (public) | `runtime.Semacquire / Semrelease` (via linkname) | counting |
| Rust | `parking_lot::park` / `unpark`, `std::thread::park` | 0 or 1 permit |
| C++20 | `std::atomic<T>::wait / notify_one / notify_all` | counting on value change |

## Career signal

- 自己用 futex 寫一個 sleeping mutex = Linux kernel / 系統工程強 signal
- 解釋為什麼 Go 用 gopark 而不是直接 futex = Go runtime internals 知識 (避免 syscall, M:N scheduling)
- AQS (Java AbstractQueuedSynchronizer) 內部就是 park/unpark + CLH queue — 補了這個 Java concurrency 整套打通
- C++20 `atomic::wait/notify` 也是 futex-based — HFT C++ 面試會問

## Recommended order

1. linkname wrapper Parker (基於現有 lock.go 的 `runtime_SemacquireMutex`)
2. 把 `syncx/lock.go` 的 `MutexLock` 用 Parker 實作 (補完 skeleton)
3. (Linux only) raw futex wrapper + FutexMutex demo
4. Permit-style Parker (AQS 對照)

## Dependencies

- 沒有上游 — 是 sync primitive 的最底層
- 被 `syncx/lock.go` (MutexLock), `syncx/cond.go`, `syncx/semaphore.go`, `syncx/latch.go`, `syncx/future.go` 消費
