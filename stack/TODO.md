# Stack Family TODO

> Category 抽象: LIFO transfer。Push/Pop 都動 head 指標。
> Underlying logic: 單一熱點 (head) → contention 從 single CAS spot 爆炸。三條優化路徑:elimination、flat combining、bounded array。

## 核心 invariant

- LIFO order: 在 concurrent 下 push/pop linearization 順序 = LIFO of *commit* order
- 所有 variant 的瓶頸都在 head 上的 CAS contention
- ABA 是 Treiber stack 的經典坑 — 需要 HP / tagged pointer 或 EBR

## Inventory

| Variant | Impl | Status |
|---|---|---|
| MutexSliceMPMC | slice + mutex | done |
| MutexLinkedMPMC | linked-list + mutex | done |
| LockFreeMPMC (Treiber) | CAS on head, linked-list | done (**Note:** 還沒接 reclamation,有 ABA + UAF 風險) |
| EliminationBackoffMPMC | Treiber + elimination array | done |
| **FlatCombining stack** | leader 代跑所有 op,其他 thread 提交 + spin own slot | TODO |
| **Bounded array-based** | array + atomic top | TODO |
| **TreiberStack + HazardPointer** | 把現有 Treiber 接上 reclamation | TODO (production-grade 升級) |

## Variant contracts

### Flat Combining

**Signature:**
```
type FlatCombiningStack[T] struct {
    head    atomic.Pointer[node[T]]
    lock    atomic.Bool
    pubList atomic.Pointer[publicationRecord[T]]  // thread-local 提交 slots
}
type publicationRecord[T] struct {
    op   atomic.Int32  // PUSH / POP / DONE
    val  T
    next *publicationRecord[T]
    lastUsed uint64  // for GC of inactive records
}
```

**Contract:**
- Caller 寫 own publication record (op + val),嘗試 CAS 成為 leader
- Leader 巡邏 publication list,sequential 執行所有 pending ops on local stack
- Non-leader 在 own record 上 spin 等 `op == DONE`
- Leader 解鎖前一次寫所有結果

**Key insight:** 在高 contention 下,*放棄並行* 反而比硬要 CAS 競爭快。把 N 個 CAS war → 1 個 sequential code path。Trade fairness for throughput。

**Reference:** Hendler, Incze, Shavit, Tzafrir 2010 "Flat Combining and the Synchronization-Parallelism Tradeoff", SPAA。Folly 有實作。

### Bounded Array-based

**Signature:**
```
type BoundedArrayStack[T] struct {
    buf  []T
    top  atomic.Int64  // -1 = empty
    cap  int64
}
```

**Contract:**
- Push: CAS top 從 t 到 t+1,寫 buf[t+1]
- Pop: CAS top 從 t 到 t-1,讀 buf[t]
- Push 滿了 fail (or block);Pop 空了 fail
- **Note:** publication safety — Push 寫 buf 必須在 CAS 之後完成 (release on buf write before CAS publish);Pop 對稱

**Key insight:** 固定容量,不需要 reclamation。適合 thread-pool 的 task buffer。但有 wraparound 限制。

### TreiberStack + HazardPointer

**Contract:**
- 修改 `lockfree_mpmc.go`:
  - Pop: protect head.next via HP slot,reload check,再 CAS
  - Push: 不需要 HP (CAS-only)
  - 任何 Pop 成功的舊節點 → retire

**Key insight:** 補上這層後,Treiber stack 才是 production-grade。沒這層 = 玩具實作。

## Career signal

- Stack 在 HFT 用得少 (queue 更常見);但 elimination + flat combining 是 "進階 lock-free 思維" 的 signal
- 寫一篇 blog 比較 3 條優化路徑 (Treiber → Elimination → Flat Combining) 在不同 contention 下的曲線 = 強 differentiator
- AI infra: task pool 偶爾用 stack-of-work

## Recommended order

1. **TreiberStack + HazardPointer** (production-grade 升級,優先!)
2. FlatCombining (新 variant + blog 素材)
3. Bounded array-based (簡單 + 跟 task buffer 對接)

## Dependencies

- Treiber → `reclamation/` (HP) for production
- Elimination 用現有的 backoff,不需要 reclamation
- 全部 lock-free → `memory/`
