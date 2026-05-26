# actor — Actor Model

> **訓練主題**:**「每個物件有自己的 inbox + state,只透過訊息互動」** — 跟 channel-based / shared-memory 完全不同的世界觀。
>
> **為什麼獨立 package**:
> - 是一整套 paradigm,不是單一 primitive(對應 Erlang OTP / Akka / Pony)
> - 有自己的型別需求(mailbox、scheduler、supervisor tree)
> - 跟 channel-based 並列為兩大「不共享 state」流派,值得自成一檔

---

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **later(6mo+)** | **2.04** | 6.0/10 | 0.85 `actix`/tokio | 2.5 週 | T2 |

> event-driven services;Ray 生態(AI infra)。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心概念

```
Channel-based (Go):                Actor (Erlang/Akka):
─────────                          ─────────
goroutine 共享 channel             actor 各自擁有 mailbox
channel 是顯式 first-class         mailbox 是隱式(actor 內建)
select 多 channel                  actor 一次只處理一個訊息
flat                               supervisor tree(階層)
無 identity                         actor 有 unique ID + 可遠端引用
```

---

## Inventory

### A. 核心 primitive

| 名稱 | 一句話 |
|------|--------|
| **Mailbox (SPSC inbox)** | 每個 actor 一個 inbox,**多 sender 一個 receiver** |
| **Actor Trait/Interface** | `Receive(msg)` + state |
| **Actor Reference (Ref)** | 可序列化的 actor 識別子 |
| **Scheduler** | dispatcher,把 mailbox 有訊息的 actor 排上 worker thread |
| **Supervisor** | 父 actor 監控子 actor,異常時重啟 / 停止 / escalate |

### B. 進階 feature

| 名稱 | 一句話 |
|------|--------|
| **Selective Receive (Erlang)** | mailbox 不一定 FIFO 處理,可 pattern-match 跳過 |
| **Ask Pattern** | request-reply,接 `syncx/Future` |
| **Become / Behavior Switch** | actor 動態改變 receive function(state machine) |
| **Backpressure Mailbox** | 滿了 block sender / drop / dead letter |
| **Persistent Actor (Event Sourcing)** | 訊息序列存檔,重啟可 replay |

### C. 拓樸 / 監督

| 名稱 | 一句話 |
|------|--------|
| **One-for-One** | 子 actor 掛了只重啟它自己 |
| **One-for-All** | 一個掛了所有兄弟一起重啟 |
| **Rest-for-One** | 後啟動的兄弟一起重啟 |

---

## 建議實作順序

```
1. Mailbox (per-actor SPSC)   ← 用 queue/ 的 SPSC ring 或 channel
2. Actor + Scheduler 基礎     ← Receive(msg) + dispatcher
3. Ask Pattern                ← 整合 syncx/Future
4. Become / Behavior Switch   ← state machine
5. Supervisor (One-for-One)   ← restart 邏輯
6. Selective Receive          ← 進階 mailbox(Erlang 風格)
7. Persistent Actor           ← 結合 event sourcing(可選)
8. Distributed Actor (RPC)    ← 跨 process / machine(進階)
```

---

## 設計重點

### Mailbox 是 SPSC 還是 MPSC?
- **答**:**MPSC** — 多個其他 actor / 外部 goroutine 都可能 send,只有自己 receive
- 用 `queue/LockFreeMPSC` 是天然 fit
- **gotcha**:high-throughput actor 的 mailbox 是 hot spot,padding / per-priority queue

### Actor 怎麼上 worker?
- 兩種 model:
  - **Thread-per-actor**(老 Erlang)— actor 跟 OS thread 1:1
  - **M:N scheduling**(Akka / 現代)— actor 池 + worker pool,actor 有訊息才上 CPU
- 後者效率高但複雜,需要「**fairness**」(避免一個 actor 霸佔 worker)

### Supervisor
- 子 actor panic → 父 actor 收到 `Terminated(child_id, reason)`
- 父 actor 決定:重啟 / 停止 / 上報祖父
- 「**讓它崩潰(Let it crash)**」哲學 — 不防禦性 catch,讓 supervisor 管

---

## Dependencies

- → `queue/LockFreeMPSC`(mailbox)
- → `syncx/Future`(ask pattern)
- → `scope/`(actor lifetime, supervision)
- → `pattern/Active Object`(概念非常近,可共享底層)
- ← 可作為 `parallel/Pipeline` 的執行 model

---

## 跨語言 / 框架對照

| 框架 | 語言 | 特色 |
|------|------|------|
| **Erlang / OTP** | Erlang | 開山祖,supervisor tree + hot code reload |
| **Akka** | Scala / Java | JVM 上 actor 標準 |
| **Pony** | Pony | 編譯期保證 actor 沒有 data race |
| **Orleans** | C# | 「Virtual actor」— actor 自動 spawn/dispose |
| **Pekko** | Scala | Akka 的 OSS fork |
| **Ergo** | Go | Erlang 風格的 Go 框架(可參考實作) |

---

## Career signal

- **遊戲 backend**(Riot / Discord)大量用 actor 處理玩家狀態
- **電信 / 金融**:Erlang 文化的延伸
- **AI infra**:Ray 是分散式 actor 框架
- 自刻 mailbox + 寫一篇「actor vs Go channel 的 benchmark」blog 是強 signal
- WhatsApp 早期 2 億用戶用 Erlang 跑(2 個工程師)— actor 哲學的代表案例

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 2(Composite ★3.0)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★2/5 | Not Required | No HFT interview source asked about actor model; post-trade/risk systems occasionally use Akka |
| **Crypto** | ★3/5 | Advanced | Narwhal network layer uses actor-like message passing between validators; Coinbase sometimes structured as actor systems |
| **AI Infra** | ★4/5 | Advanced | Ray's entire distributed execution model = actors; vLLM MultiProcExecutor = actor-like broadcast message queue |
| **FAANG** | ★3/5 | Advanced | Amazon SDE3 lists "actor model" as tested topic; Discord uses Elixir BEAM actor model |
| **Dubai** | ★3/5 | Advanced | Akka-based crypto exchange architectures; G42/Core42 AI ecosystem |
| **Composite** | **★3.0/5.0** | **Tier 2** | — |

### 必要(Required for senior infra interviews)

> 本 package 的跨 vertical 共識必要項集中在 AI Infra Ray 生態系統:

- **Mailbox (MPSC queue-based)** — 用 queue/LockFreeMPSC;actor inbox 是 hot path
  - Evidence: [AI Infra research](../docs/research/ai_infra.md) — Ray actor remote method calls; vLLM MultiProcExecutor broadcast message queue
- **Basic Actor + Scheduler** — M:N scheduling;actor 有訊息才上 CPU;fairness 避免霸佔 worker
  - Evidence: [FAANG research](../docs/research/faang.md) — "Amazon SDE3 lists actor model as a tested topic"

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator — 特別是 AI Infra Ray 路線。

- **Supervision tree** — One-for-One / One-for-All / Rest-for-One;Ray worker failure detection and restart
  - Best for: AI Infra (Ray training cluster worker failure; vLLM worker crash recovery)
- **Selective receive** — Erlang-style pattern-match skip;stale message handling
  - Best for: Crypto (Narwhal validator message ordering; selective block proposal processing)
- **Ask pattern** — request-reply + Future;actor 回傳 Future 給呼叫方
  - Best for: AI Infra (Ray placement groups, multi-node actor scheduling)

### Recommended Order(本 package 內部)

1. Mailbox MPSC(用 queue/LockFreeMPSC)
2. Actor + Scheduler 基礎
3. Ask Pattern(接 syncx/Future)
4. Become/Behavior Switch(state machine)
5. Supervisor One-for-One
6. Selective Receive(Erlang 進階)

### 對應的 Blog 題材(若想寫)

- "Go Actor 框架自刻:mailbox + scheduler + supervisor = Ray actor model 的 Go 等價"
