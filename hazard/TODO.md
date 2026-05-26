# Hazard Pointer (HP)

> Category 抽象: Hazard Pointer 是 [reclamation](../reclamation/TODO.md) family 的一個 variant。
> 本 package 專門實作 HP scheme。

## 核心 invariant

- Reader 在 deref 前先 `protect(ptr)` — 把指標寫入 own HP slot,加 fence
- Writer unlink 後 `retire(ptr)` — 加進 own retire list
- 週期性 scan: 收集所有 thread 的 HP slot,只 free 不在任何 HP 裡的 retired 節點

**核心 protocol — Read fence semantics:**

```
again:
  p = atomic.Load(slot)
  hp[me] = p            // protect
  fence
  p2 = atomic.Load(slot) // reload
  if p != p2: goto again
  // now safe to deref p
```

Protect 之後必須 **reload** 確認沒變;沒變才能安全 deref。這個 double-check 是 HP 的精髓。

## 公開 API (signature 草案)

```go
type Domain struct {
    head    atomic.Pointer[HPRec]   // 所有 thread 的 HP record linked list
    retired *retireList             // per-domain retired pool
    threshold int                   // scan trigger
}

type Holder struct {
    domain *Domain
    slots  []atomic.Pointer[byte]   // K 個 HP slots (e.g. K=2)
    retired []retiredItem
}

// Protect a pointer; returns the safely-pinned pointer (after reload check)
func (h *Holder) Protect[T any](slot int, src *atomic.Pointer[T]) *T

// Drop protection on a slot
func (h *Holder) Reset(slot int)

// Mark a pointer for deferred deletion
func (h *Holder) Retire[T any](p *T, deleter func(*T))

// Scan all hazard slots and free retired pointers that are not pinned
func (h *Holder) Reclaim()
```

**Contract:**
- 每個 goroutine 一個 Holder,持有 K 個 HP slot (K 通常 = 2 for queue / stack)
- Retire list size 超過閾值 (e.g. `2 * num_threads * K`) 觸發 Reclaim
- Domain 可分 (per-DS 一個,降低 scan 範圍) 或 global

## Key insight

HP 的成本不對稱:

| 路徑 | 成本 |
|---|---|
| Reader fast path | 1 store + 1 fence + 1 reload check |
| Writer retire | amortized O(1) |
| Reclaim | O(N_threads × K + retire_list_size) |

對 read-mostly DS 不利 (reader 仍要 fence)。read-mostly 應該用 RCU / EBR / QSBR (見 [reclamation/TODO.md](../reclamation/TODO.md))。

## 跨語言對照

- **C++26:** `std::hazard_pointer` 進標準了 (P1121)
- **Folly:** `folly::hazptr_holder` / `folly::hazptr_obj_base`
- **Rust:** `haphazard` crate
- **Linux kernel:** 沒有 (kernel 用 RCU)
- **Java:** 沒有原生 (用 AtomicReference + version stamp 或 GC)

## Reference

- Michael 2004 "Hazard Pointers: Safe Memory Reclamation for Lock-Free Objects", TPDS
- Maged Michael 多篇 follow-up
- C++ proposal P1121 (C++26 入標準)
- Folly source `folly/synchronization/Hazptr.h`

## 對 gopher-forge 的整合 (TODO)

| 消費者 | 用途 |
|---|---|
| `queue/lockfree_mpmc.go` | M&S unbounded 版的 Enqueue/Dequeue 用 HP 保護節點讀取 |
| `stack/lockfree_mpmc.go` | Treiber Pop 用 HP 保護 head.next 讀取 |
| `rcu/` | 可選:RCU 內部用 HP 偵測 grace period |

## Recommended order

1. **Phase 1:** 單一 global domain + 固定 thread 數的 HP slot 表 + Reset + Retire + naive scan
2. **Phase 2:** 動態 thread registration (per-goroutine TLS via runtime.LockOSThread or sync.Pool)
3. **Phase 3:** 接到 Treiber stack 上跑 stress test
4. **Phase 4:** 加 amortized reclaim 閾值

## Career signal

- C++26 把 HP 提到標準 → signal 強
- Folly HP 是面試常考的 reference impl
- 自己刻過 HP 並用在 Treiber stack = production-grade lock-free 的證明
- Citadel / Jane Street / HRT 面試會問 "your lock-free stack does X under workload Y, what fails?" — 答案就是 ABA + UAF,解法是 HP

## Dependencies

- → `memory/` (fence patterns)
- 被 `queue/`, `stack/`, 可選地 `rcu/` 消費
