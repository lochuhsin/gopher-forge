# scope — Cancellation & Structured Concurrency

> **訓練主題**:Goroutine 的「生命週期管理」。誰啟動誰負責、誰失敗誰拖人陪葬、誰先 timeout 就誰收手。
>
> **為什麼獨立 package**:
> - 概念跟 syncx 的 primitive 正交 — syncx 解決「**怎麼同步**」,scope 解決「**何時該停**」
> - 有自己的型別系統(token tree、scope handle、cancellation propagation),不是單一 primitive
> - 對應 Rust async / Kotlin coroutineScope / Trio Nursery,**自成一個 paradigm**

---

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **E(6mo·容器/並行)** | **2.80** | 7.0/10 | 0.8 tokio `JoinSet` | 2.0 週 | T1 |

> fintech/BNPL cancellation;FAANG context idiom。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心概念

```
parent scope
 ├── child task 1
 ├── child task 2  ←— 失敗
 └── child task 3  ←— 自動被取消(因為兄弟失敗)

parent scope.Wait() 必須等所有 child 結束才返回 (不會 leak goroutine)
```

跟 Go 的 `context.Context` 差別:
- `context` 是「取消信號的單向傳遞」,**啟動 goroutine 是另一回事**(go fn() 跟 context 沒綁定)
- **structured concurrency**:啟動 goroutine **必須**綁在 scope 上,scope 退出前所有 child 必須結束

---

## Inventory

| 名稱 | 一句話 | 難度 |
|------|--------|------|
| **CancellationToken** | 比 context 更通用、可任意 fan-out/fan-in 的取消信號 | ⭐⭐ |
| **TokenTree** | Token 的父子樹狀傳播 — parent cancel → 所有 child cancel | ⭐⭐⭐ |
| **Nursery / Scope** | Trio 風格:`scope.Go(fn)` 啟動 child;`scope.Wait()` 等全部 | ⭐⭐⭐ |
| **ErrGroup** | `golang.org/x/sync/errgroup` 自刻版 + scope 整合 | ⭐⭐ |
| **DeadlineScheduler** | Timing wheel + cancellation,大量 timer O(1) | ⭐⭐⭐⭐ |
| **CooperativeCancellation** | check-point 模式 vs preemptive,對比實驗 | ⭐⭐ |
| **CancellationCallback** | OnCancel 註冊回調,避免 polling | ⭐⭐⭐ |

---

## 設計重點(實作前的思考題)

1. **Cancellation 是 sticky 還是可恢復?** 一旦 cancel,能 reset 嗎?(Go context: 不行;Rust CancellationToken: 不行;.NET CTS: Reset 有但建議避免)
2. **Cancel 傳播是 push 還是 pull?** Push = parent 主動通知 children;pull = child 查 parent 狀態。前者快但需要登記表;後者慢但簡單。Go context 是 hybrid。
3. **失敗如何傳播?** Nursery 預設:**任一 child 失敗 → cancel 所有兄弟 + parent 失敗**。errgroup 也是。但有「ignore-failure mode」嗎?
4. **Cooperative 還是 preemptive?** Goroutine 沒有 preemptive cancel(不像 Java `Thread.interrupt`),所有點都得自己 check `<-ctx.Done()`。

---

## 建議實作順序

```
1. CancellationToken         ← 比 context 更乾淨的最小版
2. TokenTree                  ← parent-child 傳播
3. Nursery (基礎)             ← scope.Go + scope.Wait
4. ErrGroup                   ← 加上 first-error 邏輯
5. CancellationCallback       ← OnCancel,避免 polling
6. DeadlineScheduler          ← timing wheel,大量 deadline
7. Cooperative vs Preemptive  ← benchmark
```

---

## 跨語言對照

| 語言 | API |
|------|-----|
| Go | `context.Context`(部分)、`errgroup`(部分) |
| Rust | `tokio_util::sync::CancellationToken`、`tokio::select!`、drop = cancel |
| Kotlin | `coroutineScope { launch { ... } }`(完整 structured concurrency) |
| Python | Trio `nursery`、anyio,**這個概念的發源地** |
| C# | `CancellationTokenSource` + `Task` |
| Java | `StructuredTaskScope`(Java 21+, JEP 453) |

---

## Dependencies

- → `syncx/` 的 `Future` / `WaitGroup`(scope.Wait 內部)
- → `syncx/cond.go`(OnCancel callback 註冊)
- ← `actor/`、`pattern/Reactor` 都會用 scope 做 worker lifetime
- ← `parallel/pipeline`(stage cancel propagation)

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 1(Composite ★3.6)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★3/5 | Required (Two Sigma/DRW) | Two Sigma Rust structured concurrency; DRW: "network and concurrent programming involving low latency and high message rates" |
| **Crypto** | ★3/5 | Required | Coinbase/Kraken service design: context cancellation, graceful shutdown; scope = idiomatic in every Go/Rust crypto service |
| **AI Infra** | ★4/5 | Required | vLLM preempts running requests under memory pressure (requires scope/cancellation); distributed training node failure propagation |
| **FAANG** | ★5/5 | Required | Go `context.Context` = THE most tested "idiomatic Go" topic in senior interviews; every HTTP handler uses context |
| **Dubai** | ★3/5 | Required | BNPL idempotency / fraud detection (Tabby/Tamara); graceful shutdown patterns |
| **Composite** | **★3.6/5.0** | **Tier 1** | — |

### 必要(Required for senior infra interviews)

> 在 **≥2 個 vertical** 被列為 Required,或 composite ≥ 3.4。

- **CancellationToken** — 比 `context.Context` 更通用的取消信號;fan-out/fan-in 傳播
  - Evidence: [FAANG research](../docs/research/faang.md) — "`context.Context` cancellation and deadline propagation is the single most tested idiomatic Go topic"
- **Nursery / scope.Go + scope.Wait** — Trio nursery 風格;structured concurrency;goroutine lifecycle binding
  - Evidence: [Cloudflare/Uber Go senior interviews](../docs/research/faang.md) — "propagating cancellation from main through goroutine trees via ctx.Done() channel is a coding interview question"
- **ErrGroup** — first-error propagation + parallel goroutine fan-out;`golang.org/x/sync/errgroup` 自刻版
  - Evidence: FAANG ★★★★★ — "errgroup for parallel work with error propagation; knowing this vs rolling your own is a signal"

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator。

- **DeadlineScheduler (timing wheel)** — 大量 timer O(1);LLM API TTFT SLA enforcement
  - Best for: AI Infra (every LLM API request needs per-request timeout); FAANG (distributed tracing SLA)
- **CooperativeCancellation checkpoint** — go routine check point 模式 vs 空轉 polling;benchmark
  - Best for: AI Infra (preemption-on-memory-pressure; vLLM preempts decode requests)
- **CancellationCallback** — OnCancel 回調,避免 polling;Rust drop = cancel 對照
  - Best for: Crypto (Rust-heavy Jito/Helius shops) + AI Infra

### Recommended Order(本 package 內部)

1. CancellationToken(最小版)
2. TokenTree(父子傳播)
3. Nursery 基礎(scope.Go + scope.Wait)
4. ErrGroup(first-error 邏輯)
5. CancellationCallback(OnCancel)
6. DeadlineScheduler(timing wheel)
7. CooperativeCancellation benchmark

### 對應的 Blog 題材(若想寫)

- "Go context vs Trio nursery vs Rust CancellationToken:structured concurrency 三種方言"
- "DeadlineScheduler:用 timing wheel 管理 100K concurrent LLM API 請求 timeout"

---

## Career signal

- **Rust async runtime(tokio)整套靠 cancellation token** — 自刻能對應 Rust infra 入門
- **Java 21 `StructuredTaskScope`** 剛 stabilize,熟悉這個概念在 Java infra 變稀缺資產
- **Anthropic / 大模型 infra** 大量平行 job + 任一失敗回滾 — 是這個模型的天然 use case