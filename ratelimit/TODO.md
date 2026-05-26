# ratelimit — Flow Control & Resilience

> **訓練主題**:**「生產者比消費者快怎麼辦?」** 不擋會 OOM,擋了會卡 — 中間的藝術。
>
> **為什麼獨立 package**:
> - 跟 syncx primitive 正交 — ratelimit 是 **policy** 層,不是 primitive
> - 多個演算法解同一問題,trade-off 不同,適合對比
> - 工業界面試常見題(分散式 rate limit、circuit breaker design)

---

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **C(3mo·杜拜 anchor)** | **2.98** | 8.5/10 | 0.7 `governor`(policy層) | 2.0 週 | T0 |

> V 最高(Binance 確認面試題+VARA)但 R_corr 低 → 杜拜硬門檻,不展示 Rust 系統深度。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心概念

```
場景:每秒最多 N 個 request

Token Bucket:      容量 C 的桶,以 r/s 速率補 token,每 req 拿 1 個 → 允許 burst
Leaky Bucket:      固定速率 r/s 漏出 → 輸出穩定,丟超出的
Fixed Window:      每整秒重置 counter → 邊界毛刺(2N in 2 秒)
Sliding Window:    時間 weighted counter → 平滑
GCRA:              Cell rate algorithm,Token bucket 的常數空間版
```

---

## Inventory

### A. Rate Limiter 家族

| 名稱 | 行為 | 空間 | 一句話 |
|------|------|------|--------|
| **Token Bucket** | 允許 burst | O(1) | 最常用,Stripe / AWS API 都用 |
| **Leaky Bucket (queue)** | 平滑輸出 | O(N) | 像水管漏水,fix-rate output |
| **Fixed Window Counter** | 簡單但有邊界 burst | O(1) | 不推薦,教學用 |
| **Sliding Window Log** | 精確 | O(N) | 記錄每個請求時間 |
| **Sliding Window Counter** | 近似 | O(1) | 兩個 fixed window 加權 |
| **GCRA** (Generic Cell Rate) | Token bucket 的等價但更省 | O(1) | Cloudflare 用 |
| **Distributed Rate Limit** | Redis-based 或 cluster-aware | depends | 跨 process |

### B. Circuit Breaker 家族

| 名稱 | 狀態機 | 一句話 |
|------|--------|--------|
| **Basic Circuit Breaker** | Closed → Open → Half-Open | 失敗率高就斷,過段時間試探 |
| **Hystrix-style** | + sliding window + bulkhead | Netflix 經典 |
| **Adaptive Breaker** | based on latency percentile | Google SRE / 平台級 |

### C. Bulkhead / 隔離

| 名稱 | 一句話 |
|------|--------|
| **Bulkhead** | 不同類型的 request 用不同 pool,一壞不影響另一邊 |
| **Semaphore Isolation** | `syncx/Semaphore` 的直接應用 |
| **Thread Isolation** | 各自 worker pool |

### D. Backpressure

| 名稱 | 一句話 |
|------|--------|
| **Credit-based Flow Control** | 消費者發 N 個 credit,生產者只能發 N 個 |
| **Pull-based** | 消費者主動拉,生產者不主動推 |
| **Drop / Replace / Block** | 三種溢出策略對比 |

---

## 建議實作順序

```
1. Token Bucket             ← 最常用,O(1) 版本
2. Leaky Bucket             ← 對比 burst 行為
3. Sliding Window Counter   ← 近似版,實用
4. Sliding Window Log       ← 精確版,benchmark 空間 vs 精度
5. GCRA                     ← Token bucket 的常數空間等價
6. Basic Circuit Breaker    ← 三態 state machine
7. Bulkhead                 ← 用 syncx/Semaphore 隔離
8. Hystrix-style            ← 上面組合 + sliding window 統計
9. Credit-based Backpressure  ← Reactive streams 的核心
```

---

## 設計重點

### Token Bucket 的並發實作
- **lazy refill**:不開 goroutine 補 token,呼叫時計算 `min(capacity, last_tokens + (now - last_time) * rate)`
- **CAS loop**:atomic 更新 `(tokens, last_time)` pair — 教學重點
- **gotcha**:high-rate 下 CAS contention,可分片(per-CPU bucket)

### Circuit Breaker state machine
```
Closed --(failure rate > threshold)--> Open
Open   --(timeout passed)-->           Half-Open
Half-Open --(test req succeed)-->      Closed
Half-Open --(test req failed)-->       Open
```
- 狀態轉換要 atomic
- 統計用 sliding window(接 A 的 sliding window 實作)

### Bulkhead
- 用 `syncx/Semaphore` 配額不同 pool
- 每個 caller 拿不同 semaphore,壞掉的 caller 不影響其他

---

## Dependencies

- → `syncx/Semaphore`(bulkhead)
- → `syncx/atomic`(token bucket、circuit breaker state)
- → `scope/`(timeout + cancellation)
- ← `pattern/Pipeline`(backpressure 機制)

---

## Career signal

- **Stripe / Cloudflare / AWS API gateway**:rate limit 是 day-1 題
- **Netflix / Uber backend**:circuit breaker(Hystrix / Resilience4j)文化
- 寫一篇「**為什麼分散式 rate limit 比單機難 10 倍**」+ Redis Lua script 實作 → 強 signal
- HFT:credit-based backpressure 在 market data path 用得很重

---

## 參考資料

- Stripe **"Scaling your API with rate limiters"** blog
- AWS **token bucket** 文件
- Netflix Hystrix wiki(雖然 deprecated 但設計仍是經典)
- Cloudflare **"How we built rate limiting capable of scaling to millions of domains"**

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 0(Composite ★4.4)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★4/5 | Required | HRT: "pre-trade risk checks that are fast, correct, and safe under partial failures"; circuit breaker-style controls cited at SIG |
| **Crypto** | ★4/5 | Required | "Write a thread-safe rate limiter" = explicitly cited Coinbase interview question; Chainlink OCR rate-limits report submissions |
| **AI Infra** | ★4/5 | Required | Token bucket for interactive LLM API; leaky bucket for batch inference admission control; Together AI/Perplexity production serving |
| **FAANG** | ★5/5 | Required | #1 most-cited system design concurrency topic across 40+ sources; Uber `go.uber.org/ratelimit` = Go production reference; Redis Lua atomic rate limit |
| **Dubai** | ★5/5 | Required | Binance confirmed: "rate limiting in distributed environment"; VARA compliance must have rate limits |
| **Composite** | **★4.4/5.0** | **Tier 0** | — |

### 必要(Required for senior infra interviews)

> 在 **≥2 個 vertical** 被列為 Required,或 composite ≥ 3.4。

- **Token Bucket (atomic CAS, lazy refill)** — 不開 goroutine 補 token;`(tokens, last_time)` CAS pair;burst-tolerant
  - Evidence: [Uber rate limiting blog](https://www.uber.com/blog/ubers-rate-limiting-system/) — `go.uber.org/ratelimit` leaky bucket style; [FAANG research](../docs/research/faang.md) — ★★★★★, #1 system design topic
- **Sliding Window (log + counter 兩版本)** — log = 精確 O(N);counter = 近似 O(1);對比是面試常見深度問題
  - Evidence: Coinbase interview question: "thread-safe rate limiter"; [hellointerview distributed rate limiter](https://www.hellointerview.com/learn/system-design/problem-breakdowns/distributed-rate-limiter)
- **Leaky Bucket** — 固定速率;batch inference admission control;`go.uber.org/ratelimit` 底層是 leaky bucket
  - Evidence: AI Infra research — "leaky bucket for steady-state batch inference admission"
- **Circuit Breaker** — Closed→Open→Half-Open state machine;HFT pre-trade kill-switch 等價
  - Evidence: [questdb HFT circuit breakers](https://questdb.com/glossary/high-frequency-trading-circuit-breakers/) — "automated risk control at microsecond intervals"

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator。

- **Redis Lua script distributed rate limiter** — 原子性 across processes;VARA 合規 + Binance 面試命中
  - Best for: Dubai (Binance JD confirmed); FAANG (senior system design component)
  - Evidence: [Uber intelligent load management 2026](https://www.uber.com/en-IN/blog/from-static-rate-limiting-to-intelligent-load-management/) — production Redis-based
- **Bulkhead (Semaphore isolation)** — 不同 caller 用不同 Semaphore pool;壞掉不影響其他
  - Best for: AI Infra (VIP vs standard GPU memory budget isolation)
- **GCRA (Generic Cell Rate Algorithm)** — Token bucket 的常數空間等價;Cloudflare 生產使用
  - Best for: FAANG (Cloudflare `blog.cloudflare.com/how-we-built-rate-limiting`)

### Recommended Order(本 package 內部)

1. Token Bucket(最常用,O(1))
2. Leaky Bucket(對比 burst)
3. Sliding Window Counter(近似版,實用)
4. Sliding Window Log(精確版)
5. GCRA(Token bucket 等價)
6. Basic Circuit Breaker(三態)
7. Bulkhead(用 syncx/Semaphore)
8. Redis Lua distributed rate limit(進階)

### 對應的 Blog 題材(若想寫)

- "Go rate limiter 完全指南:token bucket CAS 陷阱、sliding window 精度對比、Redis 分布式版本"
- "pre-trade risk control = HFT 的 rate limiter:為什麼速度比準確更重要"
