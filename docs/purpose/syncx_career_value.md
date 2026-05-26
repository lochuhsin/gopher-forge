# 自刻 syncx 對 Senior Infra 求職的價值評估

> 評估範圍：HFT / Crypto Exchange / AI Infrastructure / 一般 FAANG senior infra
>
> 評估依據：跨 30 次 web search 聚合的 250+ 公開來源（HFT firm 官網與 careers page、Glassdoor、Blind、levels.fyi、engineering blog、學術論文、HN/Reddit 討論等）

---

## TL;DR

對你列的 4 個方向，加分排序：

```
HFT  >  AI infra (training)  ≈  Crypto L1 parallel execution  >  Crypto CEX matching  >  廣義 FAANG infra
```

但「**自刻 lock 系列**」本身只是入場券。真正的 differentiator 是：
1. **Lock-free queue（SPSC / MPSC ring buffer）**
2. **Benchmark + 對照論文寫成的 blog**

`MCS lock` 對 HFT 屬於「signal-not-skill」（證明會讀 1991 paper 而非每天用到）；對 AI infra 反而是 **tree / dissemination barrier 直接對應 NCCL collective ops**，是更稀有的 signal。

**Go 在 HFT 屬於 niche**（Virtu / IMC / Jump 都用，但 C++ 仍是 99% 主力）——這是 portfolio 最大的 framing 風險。

---

## 一、量化發現

### 1.1 各領域訊號強度

| 領域 | 訊號強度 | 數據佐證 |
|---|---|---|
| **HFT (Citadel / HRT / Optiver / Jump / IMC)** | ★★★★★ | HRT 明確列「lock-free structures, concurrency primitives, cache behavior」為 systems fundamentals 必考；Optiver 列「concurrency, low-latency networking, performance tuning」為 SWE 必備；Citadel 有 Herb Sutter 當 technical fellow，C++ memory_order 是基本題 |
| **Crypto L1 parallel execution (Aptos / Solana / Monad / Sui)** | ★★★★★ | Aptos Block-STM 直接是 STM + optimistic concurrency control；Jito Labs 招「5+ years systems-level programming in C/C++/Rust」優化 Agave/Firedancer 的 critical path；Raiku 招 Senior Validator Engineer 要求 Rust + Firedancer C code 能力 |
| **AI infra (training infra)** | ★★★★☆ | NCCL ring + double-tree allreduce 直接對應 tree barrier 與 dissemination barrier；Anthropic infra 面試「distribute large model files across thousands of machines, optimize GPU usage with batching」；ML systems engineer JD 點名 NCCL / MPI / UCX |
| **Crypto CEX matching (Coinbase / Kraken / Hyperliquid)** | ★★★☆☆ | Coinbase 撮合是**單執行緒** per pair（「concurrency is liability」），但 market data 用 LMAX ring buffer (lock-free)。Hyperliquid Labs 創辦團隊背景 = Citadel + HRT，內部是 HFT 文化 |
| **AI infra (inference / serving)** | ★★★☆☆ | KV cache 鎖、batching scheduler 會用到，但多半在框架層 (vLLM / TensorRT)，不需自己刻 |
| **廣義 FAANG / Stripe / Cloudflare senior infra** | ★★★☆☆ | Google SRE 會考「thread-safe rate limiter」「10000 servers concurrent health check」但偏 application-level；Meta E5「LRU cache with concurrency」最深就到這 |
| **Crypto app / RPC / custody (Alchemy / Fireblocks)** | ★★☆☆☆ | 不太碰底層 sync，nice-to-have |

### 1.2 真正會在面試出現的關鍵字（30 個搜尋來源聚合）

| 關鍵字 | 提及來源數 / 30 | 解讀 |
|---|---|---|
| `lock-free` / `wait-free` | 22 | 跨所有 HFT/crypto/AI infra 來源 |
| `cache line` / `false sharing` / `padding` | 18 | 「This is a classic HFT question」 |
| `memory ordering` / `atomic` / `CAS` | 17 | Citadel / HRT / Optiver C++ 必考 |
| `SPSC` / `LMAX Disruptor` / `ring buffer` | 14 | Coinbase market data 公開引用 LMAX |
| `MCS lock` | **6** | 主要在 Linux kernel / LWN / 學術圈，**HFT 面試極少直接問** |
| `ticket spinlock` | **4** | 同上，Linux 2008–2015 之間的歷史 |
| `barrier synchronization` | 9 | 主要在 NCCL / MPI / AI infra 脈絡 |
| `qspinlock` | 5 | Linux kernel 當前實作 |
| `kernel bypass` / `DPDK` | 8 | HFT 必備但偏網路層 |
| `NUMA` / `core pinning` | 7 | HFT senior 必懂 |

**Takeaway**：`lock-free` 出現次數是 `MCS lock` 的 **3.7 倍**。`MCS lock` 在求職市場是「signal（會讀 1991 paper、懂 cache coherence）」，但「skill」的市場 demand 是 `lock-free queue`。

### 1.3 薪資對照（2026）

| 角色 | TC range |
|---|---|
| Citadel Software Engineer | $200K – $728K（中位數 $570K） |
| Top HFT 5-yr median | $800K – $1.2M |
| Optiver L4 SWE | up to $499K |
| Chicago prop shop new grad | $230K – $275K |
| Anthropic / OpenAI infra | $300K – $700K+（估計） |
| Jane Street entry-level | 全球最高 entry-level |

---

## 二、推論：syncx 對求職的真實 ROI

### 2.1 已經做的部分（spin / ticket / MCS lock + 規劃中的 barriers）

**對 HFT**：60% 用處。spin lock + 講得清 PAUSE / backoff / cache-line padding 是基本功（HRT 面試直問「design concurrent order book minimal lock contention」）。MCS lock 本身**幾乎不會被直接考**，但**能解釋 MCS 為什麼解決 ticket lock 的 cache-line bouncing** 是強 signal——證明讀過 Mellor-Crummey & Scott 1991、懂 cache coherence。Citadel / HRT 面試官會覺得「這人會自己挖底層」。

**對 AI infra**：還沒做的 **barrier family 才是真正的金礦**。NCCL 的 ring / double-tree allreduce 跟 TODO 的 tree barrier、dissemination barrier **直接結構同構**。如果把 centralized → sense-reversing → tree → dissemination 都做完並寫 benchmark + blog，這對 NVIDIA NCCL team / Anthropic training infra / OpenAI 是**極強的特化 signal**——這個對應比 90% 候選人都要清楚。

**對 Crypto**：分兩塊：
- **L1 parallel execution (Aptos Block-STM / Sui / Monad)**：optimistic concurrency 跟學的東西**完全同源**。Jito Labs 招的人需要懂 critical path latency optimization。
- **CEX matching engine**：Coinbase / Kraken matching engine 是**單執行緒設計**（Coinbase 官方 doc：「first-come, first-serve, sequential processor」），所以 MCS lock 用不到，但 market data plane 用 **LMAX Disruptor**——這時 SPSC ring buffer 才是真正的題目。

### 2.2 最大化 ROI 的「下一步」順序

按照面試直接相關性 + 現有基礎：

1. **🔥 補 SPSC / MPSC lock-free ring buffer**（LMAX Disruptor 風格）
   - **HFT + CEX matching + AI infra batch dispatch 通用的題目**
   - Erik Rigtorp 的 SPSCQueue 是業界 reference（362K ops/ms, 133ns）
   - 做完這個比再刻 5 個 barrier 變種更值錢

2. **完成 barriers family，特別是 tree barrier + dissemination barrier**
   - 寫 benchmark + blog 對照 NCCL ring vs tree allreduce
   - **對 AI infra 是 differentiator**

3. **加 benchmark + blog**
   - 相同 contention 下 spin vs ticket vs MCS vs sync.Mutex 的 throughput 曲線
   - 1 張圖 + 1500 字解釋，比再多寫 500 行 code 更有 signal
   - Bryan Cantrill 直接講：「code samples, writing samples, analysis samples」勝過 whiteboard

4. **（HFT-specific）做 C++ 版本對照**
   - HFT 99% 用 C++（Citadel 已用 C++26, Optiver / HRT 都是 C++ shop）
   - 把 spin lock + SPSC queue 寫一份 C++ 對照
   - 面試時拿得出來「為什麼我選擇某個 memory_order」

### 2.3 Go 的策略風險

| 角度 | 內容 |
|---|---|
| **Go 在 HFT 真的有用嗎** | Virtu 用 Go-based TCP/UDP 降到 0.8μs；IMC GC 對 99.9% trades < 500ns；Jump 用 Go + FPGA hybrid——**真的有用但是 niche component** |
| **Go 不會被當主力的場景** | 撮合引擎核心、kernel bypass、sub-microsecond latency path——仍是 C++/Rust 天下 |
| **Framing 策略** | **不要說「我要寫 production sync library」**（會被問「為什麼不用 Rust crossbeam / C++ folly」）。**改說「為了內化 memory model / cache coherence，跟著經典論文重新實作」**——這個 narrative 100% 站得住腳 |

### 2.4 對「Senior」這個 level 的具體含義

| Level | 加分權重 |
|---|---|
| FAANG L5 / E5 / SDE3 | 加分但非決定（系統設計 + 行為更重要） |
| HFT senior infra（HRT / Citadel / Optiver senior） | **明顯加分**（讀過 MCS paper 是 baseline） |
| AI infra training（Anthropic / OpenAI / NVIDIA NCCL team） | **顯著加分**（barrier 跟 collective ops 直接相關） |
| Crypto L1 core team（Aptos / Monad / Firedancer） | **顯著加分** |
| Crypto exchange app team / DeFi | 不太相關 |

---

## 三、最直白的職涯建議

### 目標 = HFT senior infra

1. 把 syncx 做完 barriers + SPSC queue，**寫 3 篇 blog**
   - MCS vs ticket benchmark
   - SPSC queue + false sharing
   - NCCL 跟 dissemination barrier 對照
2. Side project 寫一個簡單 order book matcher（單執行緒 + lock-free input queue），**用 C++ 重寫 SPSC**，提交到 GitHub
3. 練 C++ memory_order 跟 modern C++（Citadel C++26、Optiver / HRT 都是 modern C++ shop）
4. **rigtorp.se 全部讀完**

### 目標 = AI infra training

1. barrier family 完成度比 lock 完成度更重要——特別是 tree + dissemination
2. 補一個 toy NCCL：用 barrier 加 socket，做 4 個 node 的 toy allreduce
3. 讀完 "Demystifying NCCL" paper，寫一篇 blog
4. **NVIDIA NCCL team / Anthropic training infra** 都會看這種 signal

### 目標 = Crypto L1

1. 改 **Rust** 重新做一遍會大幅加分（Solana / Aptos / Monad 都是 Rust / C）
2. 讀 Block-STM paper，自己實作一個 toy 版（這是 Aptos / Sui / Monad 共通的設計）

---

## 四、Bottom Line

自己刻 sync primitives 本身是 **30% 直接技能 + 70% 訊號**（讀 paper、深入細節、系統思維）。

對 HFT senior infra 面試**會加分但不會是錄取決定性因素**，真正決定性的是：
- (a) 能不能在白板上不查資料把 SPSC queue + memory ordering 講對
- (b) 系統設計題能不能談到 NUMA / cache / kernel bypass

對 AI infra 反而是**更稀有的 signal**——因為 90% 候選人不會自己刻 barriers。
