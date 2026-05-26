# STM (Software Transactional Memory) Family TODO

> Category 抽象: 用 `atomic { ... }` 區塊包多個記憶體存取;系統保證原子 + 隔離。Concurrent code 由 sequential code 直接得到。
> **Note:** 你的 `stm.go` 目前是空的。
> Underlying logic: read-set / write-set tracking + version validation + retry on conflict。

## 核心 invariant

- Transaction 內所有 read 看到一致的 snapshot
- Transaction commit 是 atomic: 全部寫入或全部回滾
- Conflict = retry (no progress guarantee for individual tx; only system-wide progress)
- Composability: 多個 tx 可以巢狀 / 組合,系統自動處理

## Inventory

| Variant | 來源 | 觀念 | Status |
|---|---|---|---|
| **TL2 (Transactional Locking 2)** | Dice, Shalev, Shavit 2006 | word-based, lazy versioned | TODO |
| **NORec** | Dalessandro et al. 2010 | no ownership records, validate by clock | TODO |
| **SwissTM** | Dragojevic et al. 2009 | mixed eager (write-write) + lazy (read-write) | TODO |
| **Block-STM (Aptos)** | Aptos 2022 | multi-version + dependency tracking,專為 EVM/Move | TODO (Crypto L1 高優先) |
| **HTM (hardware TM)** | Intel TSX, IBM POWER8 | hardware-supported tx | n/a (硬體) |
| **STMv (versioned)** | Riegel et al. 2007 | LSA 變體 | TODO (進階) |

## Variant contracts (TODO 高優先)

### TL2 (Transactional Locking 2) — 經典入門

**Signature:**
```
type TL2 struct {
    globalClock atomic.Uint64
}
type TVar[T any] struct {
    lock atomic.Uint64  // version (62 bits) + lock bit + reserved
    val  T
}
type Tx struct {
    tl2      *TL2
    readSet  []*tvarMeta
    writeSet map[*tvarMeta]any
    rv       uint64  // read version (start clock snapshot)
}
type tvarMeta struct { ... }
func (tx *Tx) Read[T any](v *TVar[T]) T
func (tx *Tx) Write[T any](v *TVar[T], val T)
func (tx *Tx) Commit() bool
func (tl2 *TL2) Atomic(f func(*Tx) bool)  // retry-until-commit wrapper
```

**Contract:**

**Begin:**
```
rv = globalClock.Load()
```

**Read(v):**
```
preLock = v.lock.Load()
if locked or version > rv: abort
val = v.val
postLock = v.lock.Load()
if preLock != postLock: abort
readSet.append(v)
return val
```

**Write(v, val):**
```
writeSet[v] = val   // 還沒實際 commit
```

**Commit:**
```
// 1. Lock write-set
for v in writeSet:
    if !CAS(v.lock, unlocked, locked): abort_and_release_locks

// 2. Get write version
wv = globalClock.Add(1)

// 3. Validate read-set
for v in readSet:
    if v.lock.locked-by-other or v.version > rv: abort_and_release_locks

// 4. Commit writes
for (v, val) in writeSet:
    v.val = val
    v.lock = (wv, unlocked)

return ok
```

**Key insight:** Global clock 把 partial order 拉成 total order。每個 tx 都有 *看到的 snapshot version* (rv) 跟 *commit version* (wv)。validation 確保 read-set 在 [rv, wv) 期間沒被改 = isolation 保證。

Write-set 在 commit 之前不寫入,所以 abort 不需要 rollback。

**Reference:** Dice, Shalev, Shavit 2006 "Transactional Locking II", DISC。Java DeuceSTM 是 reference impl。

### Block-STM (Aptos) — Crypto L1 最高 ROI

**Signature:**
```
type BlockSTM struct {
    mvHashMap *MultiVersionHashMap   // (key, version) → value
    scheduler *Scheduler
}
type Scheduler struct {
    nextTxIdx       atomic.Uint64
    incarnation     []atomic.Uint64  // per-tx retry count
    txStatus        []atomic.Uint32  // ready / executing / executed / aborting
    txDependency    []txIdx          // 阻塞我的 tx
    validationIdx   atomic.Uint64
}
type Tx interface {
    Execute(view *MVView) (writeSet, error)
}
type MVView struct { ... }  // multi-version read view at (txIdx, incarnation)
```

**Contract (concurrent execution + sequential validation):**

1. **Execution phase (並行):**
   - 多個 worker thread pick up `nextTxIdx`
   - 跑 `tx.Execute(view)`,記錄 read-set (含每個 read 的版本)
   - 把 write-set 寫入 mvHashMap 的 (key, txIdx, incarnation) 槽位

2. **Validation phase (按 tx 順序):**
   - 對每個 tx 按 sequence number 順序 validate
   - 檢查 read-set 中讀到的版本是否仍是該 (key, txIdx 之前最大) 的最新版本
   - 若不是 → 該 tx aborted,incarnation++,schedule re-execute
   - 同時所有依賴此 tx 的下游也要 re-validate

3. **Dependency tracking:**
   - 若 tx_i 讀了 tx_j 寫的 (j < i),tx_i 依賴 tx_j
   - tx_j re-execute → tx_i 也要 re-validate

**Key insight:** 對 *transaction 要 sequential output* 的場景 (EVM/MoveVM):樂觀並行執行 + sequential validation 把吞吐拉到接近並行,同時保持結果跟順序執行一致。

關鍵不同 vs TL2:
- TL2 任意 commit 順序 (linearizable to some serial order)
- Block-STM 強制 commit 順序 = 預定的 block 內 tx 順序 (block 內 tx 順序由 leader 決定)

**Reference:** Aptos 2022 "Block-STM: Scaling Blockchain Execution by Turning Ordering Curse to a Performance Blessing"。Aptos / Sui (similar) / Monad 都用變體。

### NORec (No Ownership Records)

**Contract:**
- 沒有 per-TVar 版本/lock — 只有一個 global clock
- Read 時 snapshot global clock,write 進 write-set
- Commit:
  1. Lock global clock (CAS)
  2. Re-read 所有 read-set 確認沒變 (full validation)
  3. Apply write-set
  4. Unlock global clock,clock++
- 簡單,但 contention 高時 abort 率高

**Key insight:** 比 TL2 更簡單 (沒有 per-TVar metadata),代價是 commit 必須 *full read-set validation*。對 small read-set / low contention 反而快。

**Reference:** Dalessandro, Spear, Scott 2010 "NORec: Streamlining STM by Abolishing Ownership Records"。

### SwissTM

**Contract:**
- 對 write-write conflict 用 *eager* detection (寫時即發現)
- 對 read-write conflict 用 *lazy* detection (commit 時發現)
- 比 TL2 平均更快,但實作複雜

**Reference:** Dragojevic, Guerraoui, Kapalka 2009 "Stretching Transactional Memory"。

### HTM (硬體,觀念)

**特性:**
- Intel TSX: `XBEGIN` / `XEND` / `XABORT`
- 完全硬體支援,fast path 比 SW-STM 快 10×+
- 限制: read-set / write-set 容量受 L1 cache 限制 (~64KB);任何 page fault / syscall / IRQ 都 abort
- 通常 *hybrid* — HTM fast path + SW-STM fallback

**Use case:** GLIBC malloc 2.30+ 用 elision (HTM + lock fallback)。

## 跨語言對照

| 語言 / 系統 | STM 變體 |
|---|---|
| Haskell | `Control.Concurrent.STM` (語言原生,GHC 有 HTM support) |
| Clojure | refs + STM (Rich Hickey 設計) |
| Scala | `scala-stm` |
| Rust | 不熱 (preferring lock-free + ownership);`stm` crate 存在但 niche |
| Java | Multiverse, ScalaSTM port,DeuceSTM (AOT-instrumented) |
| C++ | TBoost.STM (experimental), TSX intrinsics |
| Crypto | Aptos (Block-STM), Sui (similar Move-VM optimistic), Monad |

## Use case

- Composable atomic — Hashkell STM 的招牌用法
- Crypto L1 並行執行 (Aptos / Sui / Monad)
- GHC garbage collector internals
- 不適合 IO-heavy tx (沒法 rollback IO)
- 不適合 long tx (high abort rate)

## Career signal

- **Crypto L1 (Aptos / Sui / Monad / Fuel) 直接命中** — Block-STM 是這幾家的 differentiator
- 你 purpose doc 標 ★★★★★ for Crypto L1
- HFT / AI infra ROI 較低 (但 HTM 在 HFT 偶爾用,Intel TSX 撤銷後熱度下降)
- 寫 blog "從 TL2 到 Block-STM" → Crypto L1 招募團隊強訊號

## 推薦做法 (現實考量)

不刻完整 STM (TL2 已是博士級工作)。建議:

1. **Toy TL2** — 5-10 TVar, single-threaded validation,證明懂 read-set/write-set + global clock
2. **Block-STM 核心** — MultiVersionHashMap + simple scheduler,證明懂 Aptos paper
3. 寫 blog 對比 (這對 Crypto L1 招募團隊比 code 還重要)

## Recommended order

1. **Toy TL2** (基本 STM 概念,先有東西能跑)
2. **NORec** (TL2 的簡化版,validation 路徑教學)
3. **Block-STM core** (Crypto L1 直擊)
4. (選做) SwissTM, HTM intrinsics demo

## Dependencies

- → `memory/` (版本號 ordering, release-acquire on TVar lock)
- → 可選 `reclamation/` (multi-version data 要 GC)
- → 跟 `map/` skip list 同樣 ordered DS 知識
