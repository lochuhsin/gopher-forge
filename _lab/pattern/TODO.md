# pattern — Architectural Concurrency Patterns

> **訓練主題**:**「組合」而非「實作」**。用既有 primitive 拼出工業界常見的 concurrency 架構。
>
> **為什麼獨立 package**:
> - 跟 syncx primitive 正交 — pattern 是**設計範式**,每個 pattern 都組合多個原語
> - 對應 POSA2 / "Pattern-Oriented Software Architecture: Concurrent and Networked Objects"
> - 每個 pattern 都是工業界真實架構的精簡版(Reactor=Netty、Disruptor=LMAX、Active Object=COM)

---

## 🎯 Priority(Dubai-focused)

> **Lab — 教學用,不在 career 實作序列內**。完整排序見 [ROADMAP.md](../../ROADMAP.md)。
> 做完 Phase A-E 之後若想練,可用 此 package 反向驗證 queue/(Disruptor 升級 LockFreePaddedMPMC)。

---

## 核心概念

```
primitive 層        →   pattern 層        →   application 層
(syncx, queue)         (reactor, actor)        (你的服務)

primitive 是「動詞」    pattern 是「句型」     application 是「文章」
```

---

## Inventory

### A. 物件層級 pattern

| 名稱 | 一句話 | 訓練重點 |
|------|--------|---------|
| **Monitor Object** | Lock + Cond 包進物件,method 自動 serialize | Java `synchronized` 的本質 |
| **Active Object** | method call 變 message,後端 actor 處理 | 解耦 caller / executor |
| **Thread-Specific Storage** | per-thread 資料,避免 lock | Go: `runtime.LockOSThread` + per-P |

### B. 事件層級 pattern

| 名稱 | 一句話 | 訓練重點 |
|------|--------|---------|
| **Reactor** | 單 thread event loop,demultiplex IO 事件 | epoll / kqueue 抽象 |
| **Proactor** | 完成通知(complete-then-notify),而非 ready-then-do | io_uring / IOCP 抽象 |
| **Acceptor-Connector** | 連線建立的責任分離 | Network server 通用骨架 |

### C. 執行緒模型 pattern

| 名稱 | 一句話 | 訓練重點 |
|------|--------|---------|
| **Half-Sync / Half-Async** | IO thread 收事件 + worker thread 處理,中間 queue | 兩層 thread pool 設計 |
| **Leader-Followers** | 一個 leader 等事件,事件到後**升一個 follower 成 leader**,自己處理 | 減 context switch |
| **Thread-Per-Connection** | 每連線一 thread(古典,但好懂) | Go goroutine 哲學的對照組 |

### D. 高效能特化 pattern

| 名稱 | 一句話 | 訓練重點 |
|------|--------|---------|
| **Disruptor (LMAX)** | Single-writer ring buffer + dependency graph + wait strategy | 無 lock 的 pipeline,你已有 padded MPMC 可升級 |
| **Pipeline + Backpressure** | Bounded stage,生產過快會被擋 | Reactive streams 的核心 |
| **Pipes & Filters** | 純資料流組合 | Unix pipe / Spark RDD |

---

## 建議實作順序

```
1. Monitor Object         ← 包 lock + cond,你已經有兩者
2. Active Object          ← Monitor + queue(method 變 message)
3. Reactor                ← 單 thread event loop,epoll-style
4. Half-Sync/Half-Async   ← Reactor 加 worker pool
5. Leader-Followers       ← 比 H-S/H-A 更精細,benchmark 對比
6. Disruptor              ← 把現有 LockFreePaddedMPMC 包成完整 LMAX
7. Pipeline + Backpressure  ← 接 syncx/Semaphore 做 credit
8. Acceptor-Connector     ← 配合 net 套件做 server 骨架
```

---

## 每個 pattern 的「最少組件清單」

| Pattern | 需要的 primitive |
|---------|------------------|
| Monitor | Lock + Cond |
| Active Object | Monitor + Queue + Future(method 回傳值) |
| Reactor | Channel(events) + Handler map + Loop |
| Half-Sync/Half-Async | Reactor + Worker Pool + Queue |
| Leader-Followers | Cond + Worker Pool + role token |
| Disruptor | Ring buffer + Sequence + Wait strategy + Barrier |
| Pipeline | Bounded queue + Semaphore + Cancellation(scope/) |

---

## Dependencies

- → `syncx/` 全家(基底)
- → `queue/`、`stack/`、`deque/`(資料結構)
- → `scope/`(pattern 內部 worker 的 lifetime)
- → `actor/`(Active Object 接近 actor)
- ← `parallel/pipeline` 用本 package 的 Pipeline pattern

---

## 學習資源

- **POSA2**(Schmidt et al. 2000)— Reactor / Proactor / Active Object / Half-Sync 原文
- **LMAX Disruptor paper**(Thompson 2011)
- **Netty 原始碼** — Reactor 工業標準實作
- **Akka 原始碼** — Active Object / Actor 工業實作

---

## Career signal

- **HFT** 的 market data path 用 Disruptor / Single-writer ring → padded MPMC 升級成 Disruptor 後可寫 blog
- **Netty / Vert.x / Node.js** 都是 Reactor 變體,做完看原始碼有歸屬感
- **POSA2 是 senior infra 面試官的共同語言** — 講「我這裡用 Half-Sync/Half-Async」比說「我用 channel + goroutine」 signal 強 10 倍

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../../ROADMAP.md) **Tier 1(Composite ★3.4)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★4/5 | Required | LMAX Disruptor cited 8+ sources as HFT production pattern; MyntBit: "Disruptor 3 orders of magnitude lower latency than queue-based approaches" confirmed as Jane Street/Citadel/Two Sigma hard question; Single-writer ring = market data path standard |
| **Crypto** | ★3/5 | Advanced | Firedancer tile pipeline = Pipeline + Backpressure pattern; Jito validator uses Reactor-style event loop for block building |
| **AI Infra** | ★3/5 | Advanced | vLLM request dispatcher = Half-Sync/Half-Async (IO thread receives HTTP, worker pool runs inference); Ray uses Actor pattern extensively |
| **FAANG** | ★3/5 | Advanced | Netty/Vert.x Reactor pattern = common backend architecture discussion; Half-Sync/Half-Async = standard thread-pool interview topic |
| **Dubai** | ★2/5 | Niche | HFT-adjacent Disruptor knowledge; Binance order matching uses Reactor-style event demux |
| **Composite** | **★3.4/5.0** | **Tier 1** | — |

### 必要(Required for senior infra interviews)

> 在 **≥2 個 vertical** 被列為 Required,或 composite ≥ 3.4。

- **Reactor (single-thread event loop)** — epoll/kqueue 抽象;Netty 的骨架;所有 non-blocking IO server 的基礎
  - Evidence: [FAANG research](../../docs/research/faang.md) — Netty Reactor = senior backend architecture discussion; Node.js/Vert.x variants
- **Active Object** — method call → message queue;解耦 caller / executor;接 actor/
  - Evidence: [AI Infra research](../../docs/research/ai_infra.md) — Ray remote() = Active Object pattern; async task dispatch in training pipeline

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator — 特別是 HFT。

- **LMAX Disruptor** — Single-writer ring buffer + sequence barrier + wait strategy;升級現有 LockFreePaddedMPMC
  - Best for: HFT — [myntbit.com Disruptor](https://myntbit.com/blog/disruptor-pattern) — "3 orders of magnitude lower latency"; MyntBit confirmed as hard quant question at Jane Street/Citadel/Two Sigma
- **Leader-Followers** — 減 context switch;一個 leader 等事件,到後升 follower
  - Best for: HFT (minimize latency on event arrival path); FAANG (C10K server design discussion)
- **Half-Sync / Half-Async** — IO thread + worker pool + bounded queue;最常見的生產線程模型
  - Best for: AI Infra (vLLM 兩層架構的模型;request scheduler + execution worker)

### Recommended Order(本 package 內部)

1. Monitor Object(包 lock + cond)
2. Active Object(Monitor + Queue + Future)
3. Reactor(單 thread event loop)
4. Half-Sync / Half-Async(Reactor + worker pool)
5. Leader-Followers(對比 H-S/H-A benchmark)
6. Disruptor(升級 LockFreePaddedMPMC)
7. Pipeline + Backpressure(接 scope/)

### 對應的 Blog 題材(若想寫)

- "LMAX Disruptor in Go:從 padded MPMC 到 sequence barrier + wait strategy"
- "HFT market data pipeline 的設計:Single-writer ring + Reactor + Leader-Followers 組合"
