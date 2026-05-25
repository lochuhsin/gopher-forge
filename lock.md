# Lock 的種類

Lock 的分類可以從「等待策略」和「並發語意」兩個維度看。以下整理九種重要的 lock，從 spinlock 一路到 RCU、recursive lock。

---

## 1. Spinlock

CAS 迴圈，不睡，燒 CPU。最底層、最簡單的 lock，所有 mutex 實作的基礎。

```
for !CAS(&state, 0, 1) {
    // busy wait
}
```

適合超短 critical section、不能睡的情境（kernel、interrupt context）。Linux kernel 的 `spinlock_t` 本質就是這個。缺點是 critical section 一長，整顆 core 都被你佔住做無用功，且 cache line 會在所有 contending core 之間不停 invalidate。

---

## 2. Blocking mutex（一般 mutex）

搶不到就把 thread 交給 OS scheduler `park` 掉，等 unlock 時 wake。

- Linux：`futex`
- Windows：`WaitOnAddress`
- macOS：`__ulock`

優點是不浪費 CPU，缺點是 park / wake 有 syscall 成本（幾微秒級），critical section 很短時反而比 spinlock 慢。

---

## 3. Adaptive mutex（spin-then-park）

現代主流實作。先 spin 幾輪試試（賭對方很快會 unlock），失敗才 park。拿到 spinlock 的低延遲 + mutex 的低 CPU 浪費。

代表實作：

- `pthread_mutex`
- Go 的 `sync.Mutex`
- Java 的 `synchronized`

Go 的 `sync.Mutex` 還會在偵測到 starvation（某個 goroutine 等 >1ms）時切到 strict FIFO 模式避免餓死。

---

## 4. Ticket lock

像麵包店抽號碼牌：每個 thread 拿一個遞增的 ticket，輪到自己的號碼才進去。**FIFO，公平**。

問題：所有 waiter 都在 spin 同一個變數，cache line 一直 bounce，scale 到很多 core 時效能差。

---

## 5. MCS lock / CLH lock

解決 ticket lock 的 cache bouncing：每個 waiter 把自己放進一個 linked list，**只在自己的 local node 上 spin**，前一個 waiter unlock 時去把後一個的 node flag 翻過來。Cache line 不會打架，scale 到幾十、幾百 core 還是好的。

實際應用：

- Linux kernel 的 `qspinlock`（queued spinlock）本質就是 MCS 的變體
- Java `AbstractQueuedSynchronizer` 用的是 CLH 變體

---

## 6. Reader-writer lock

Readers 共享 / writer 獨佔。三種策略：

- **Reader-biased**：新 reader 即使有 writer 等也能直接加入。Read throughput 最高，但有 writer starvation 風險。
- **Writer-biased**：有 writer 等時新 reader 必須 block。避免 starvation，read throughput 較低。Go 的 `sync.RWMutex` 屬於這類。
- **Fair / FIFO**：純按到達順序，兩邊吞吐量都不是最佳。

實務上 reader-biased 因為 starvation 問題被視為壞預設，大多數語言只給 writer-biased 或 fair。

---

## 7. Seqlock（sequence lock）

Reader 完全不 block writer，writer 也不 block reader——靠一個版本號（seq counter）。

- Writer 進入時把 seq +1（變奇數），離開時再 +1（變偶數）
- Reader 讀之前記下 seq、讀完再檢查 seq 有沒有變/是不是偶數，**變了就重讀**

Read 路徑完全無等待，超快，但 reader 必須能接受 retry，且讀的資料要能容忍中途看到不一致（通常配 memcpy 整塊讀）。

Linux kernel 大量用在 jiffies、時間戳這類「writer 少、reader 極多」的場景。

---

## 8. RCU（Read-Copy-Update）

更極端：**reader 完全無鎖、無 atomic、無 retry，只是普通 load**。

Writer 要修改時不直接改，而是 copy 一份新的、修改完用 atomic pointer swap 換上去，舊版本等到「沒有任何 reader 還可能看到它」之後才釋放（grace period）。

這是 Linux kernel 對付極端 read-heavy 結構（路由表、dentry cache）的武器。userspace 也有 `liburcu`。

代價：writer 變慢、API 複雜、要處理 grace period。

---

## 9. Recursive / Reentrant lock

同一個 thread 可以重複 lock 同一把鎖不死鎖，內部用 owner thread ID + counter 計數。

代表實作：

- Java `ReentrantLock`
- `pthread_mutex` 設成 `RECURSIVE` attr
- C# `lock`
- Python `threading.RLock`

**Go 的 `sync.Mutex` 故意不支援 recursive**，Russ Cox 的理由是「如果你需要 recursive lock，通常是你的設計有問題」。

---

## 速查表

| Lock | 等待方式 | 公平性 | 典型場景 |
|------|---------|--------|---------|
| Spinlock | Busy wait | 無 | Kernel、極短 critical section |
| Blocking mutex | Park / sleep | 視實作 | 一般 user code |
| Adaptive mutex | Spin → park | 視實作 | 現代主流 mutex |
| Ticket lock | Busy wait | FIFO | 需要公平性的 spin 場景 |
| MCS / CLH lock | Local spin | FIFO | 高 core 數 kernel / runtime |
| RWMutex | 視 bias | 視 bias | Read-heavy + 偶爾 write |
| Seqlock | Reader 無等 / retry | N/A | Writer 少、reader 極多 |
| RCU | Reader 完全無鎖 | N/A | 極端 read-heavy（路由表等） |
| Recursive lock | 同 mutex | 同 mutex | 同 thread 需重入 |
