# gopher-forge — Roadmap

> **目標**:杜拜/UAE 地區 · Senior SWE · **Rust/Go 職缺** · 極限場景(非 CRUD,Fintech 可)。
> Repo 用 Go 實作,概念 language-agnostic,**Rust 相關性越高越好**。
> 時間:Optimal **3 個月**,極限 **6 個月**。
>
> **方法論**:5 個平行調查 agent、聚合 **250+ 網站**(HFT / Crypto / AI Infra / FAANG / Dubai)。詳細證據在 [docs/research/](docs/research/)。
> 本檔有兩個 lens:**Part I**(行動計畫:Dubai 地區 + Rust 相關性 ROI 最佳化)、**Part II**(參考:全球 cross-vertical signal,若你之後 pivot)。

---

# Part I — 行動計畫(Dubai + Rust,實際要照做的)

## 1. 為什麼地區版 ≠ 全球產業版

杜拜真實雇主結構跟「全球該產業」不同 — deep HFT systems(MCS、dissemination barrier)在杜拜**還沒落地**:

| 杜拜雇主 | 狀態 | 語言 | 極限場景? |
|---------|------|------|-----------|
| **Crypto 交易所**(Bybit/Binance/OKX/Bitget) | **主力,大量招** | **Go 主 + Rust(Bybit 有 Rust 職缺)** | ✅ 撮合、市場資料 lock-free |
| **Fintech/BNPL**(Tabby/Tamara) | 活躍 | Go/Python | ✅ 高吞吐 payment、idempotency |
| **AI infra**(G42/Core42,Abu Dhabi) | 成長 | **Python 主** | ✅ 但 Go 只在 control plane |
| **HFT**(Citadel DIFC) | **nascent**(2026+ 談判) | C++/Rust | ✅ 但**還沒落地**(未來賭注) |
| **Rust 純職缺**(Syndica/Bybit Rust) | niche 但 premium | **Rust** | ✅ Solana infra、撮合 |

**甜蜜點 = crypto 交易所系統層(撮合/市場資料/lock-free)+ Rust 系統職缺**。HFT 是未來賭注,AI infra 受 Go-only 限制(深度需 Python)。

---

## 2. ROI 模型

```
ROI = ( V_dubai × R_corr ) / E_remaining

V_dubai     杜拜市場價值(0-10):crypto CEX 40% + fintech 20% + AI 20%
            + HFT-future 10% + 通用 backend 10%,對「極限場景」加成
R_corr      Rust 相關性(0.6-1.0):這個 Go package 教的概念,在 Rust
            系統職位被看重的程度(crossbeam / parking_lot / tokio / rayon)
E_remaining 剩餘工時(週)— 已實作的不計
```

時間換算(part-time):3 個月 ≈ **11-12 工時週**;6 個月 ≈ **23-24 工時週**。

---

## 3. Rust 相關性對照表(要 maximize 的軸)

| Package | R_corr | 對應 Rust 生態 |
|---------|--------|---------------|
| **queue/** | **1.0** | `crossbeam-queue`, `crossbeam-channel` |
| **deque/** | **1.0** | `crossbeam-deque`(Chase-Lev **就是**這 crate) |
| **hazard/ + reclamation/** | **1.0** | `crossbeam-epoch`(無 GC lock-free 回收的標誌主題) |
| **memory/** | **1.0** | `std::sync::atomic`, `Ordering::{Acquire,Release,SeqCst}` |
| **syncx/Lock**(Spin/MCS/Seqlock) | **1.0** | `parking_lot`, `spin`, `seqlock` |
| **syncx/Future** | **1.0** | `std::future::Future` + `tokio`(**Rust async 核心**) |
| **park/** | **1.0** | `parking_lot_core`, tokio park/unpark |
| **parallel/** | **0.95** | `rayon`(Rust 旗艦 data-parallel) |
| **STM** | **0.95** | Block-STM(Aptos,**Rust 寫的**) |
| **rcu/** | **0.9** | `arc-swap`, `left-right` |
| **stack/** | **0.9** | `crossbeam`(Treiber) |
| **arena/** | **0.9** | `bumpalo`, `typed-arena` |
| **map/** | **0.85** | `dashmap`, `evmap`(Block-STM 用 dashmap) |
| **actor/** | **0.85** | `actix`, `ractor`, tokio actors |
| **syncx/Semaphore** | **0.85** | `tokio::sync::Semaphore` |
| **syncx/Once** | **0.8** | `std::sync::Once`, `OnceCell` |
| **scope/** | **0.8** | tokio structured concurrency / `JoinSet` |
| **clock/** | **0.75** | 雜 crate,較邊緣 |
| **crdt/** | **0.75** | `rust-crdt`, `automerge-rs` |
| **syncx/Barrier** | **0.7** | `std::sync::Barrier`(但 NCCL=C++/CUDA) |
| **ratelimit/** | **0.7** | `governor`(policy 層) |
| **syncx/Latch/WaitGroup** | **0.7** | tokio,偏 Go idiom |
| **syncx/Cond** | **0.7** | `std::sync::Condvar` |
| **syncx/Channel patterns** | **0.6** | Go-specific(Rust mpsc 不那麼 idiomatic)— **最低** |

> **核心洞察**:R_corr=1.0 那一圈(crossbeam + parking_lot + tokio Future)**同時是** Rust 系統面試核心 **又是** 杜拜 crypto 撮合/市場資料的極限場景。兩目標完美對齊。Channel patterns 反而最低(Go 特有)。

---

## 4. 完整評分表(按 ROI 排序)

> `✅`=已實作可加 bench;`◐`=部分(補缺);`○`=從零。E=**剩餘**工時。Phase 見第 5-7 節。

| Package / Item | V | R_corr | E | **ROI** | 狀態 | Phase | Rust |
|---|---|---|---|---|---|---|---|
| stack/ polish+bench | 5.5 | 0.9 | 0.5 | **9.90** | ◐ | A | crossbeam |
| syncx/Lock +CLH+Seqlock | 8.0 | 1.0 | 1.0 | **8.00** | ◐ | **A** | parking_lot |
| syncx/Latch +CountDownLatch | 5.5 | 0.7 | 0.5 | **7.70** | ◐ | A | — |
| syncx/Once | 4.5 | 0.8 | 0.5 | **7.20** | ○ | A | OnceCell |
| syncx/Barrier 基礎(Cyclic) | 5.0 | 0.7 | 0.5 | **7.00** | ◐ | A | std::Barrier |
| syncx/Semaphore +weighted/timeout | 6.5 | 0.85 | 1.0 | **5.53** | ◐ | **A** | tokio Sem |
| park/ | 6.5 | 1.0 | 1.5 | **4.33** | ○ | **D** | parking_lot_core |
| **memory/** | 8.0 | 1.0 | 2.0 | **4.00** | ○ | **B** | std::atomic |
| **queue/** 剩餘(SPSC+M-S+bench) | 9.5 | 1.0 | 2.5 | **3.80** | ◐ | **B** | crossbeam-queue |
| **syncx/Future + Promise** | 7.5 | 1.0 | 2.0 | **3.75** | ○ | **B** | Future trait |
| **deque/** Chase-Lev | 7.0 | 1.0 | 2.0 | **3.50** | ○ | **D** | crossbeam-deque |
| map/ (sharded + sync.Map) | 8.0 | 0.85 | 2.0 | **3.40** | ○ | **E** | dashmap |
| **ratelimit/** | 8.5 | 0.7 | 2.0 | **2.98** | ○ | **C** | governor |
| rcu/ 完整 | 6.5 | 0.9 | 2.0 | **2.93** | ◐ | later | arc-swap |
| scope/ | 7.0 | 0.8 | 2.0 | **2.80** | ○ | E | tokio JoinSet |
| syncx/Barrier 進階(NCCL) | 6.0 | 0.7 | 1.5 | **2.80** | ◐stub | later(AI) | — |
| arena/ | 6.0 | 0.9 | 2.0 | **2.70** | ○ | later | bumpalo |
| syncx/Channel +pipeline | 6.5 | 0.6 | 1.5 | **2.60** | ◐ | later | — |
| **hazard/ + reclamation/** | 7.5 | 1.0 | 3.0 | **2.50** | ○ | **D** | crossbeam-epoch |
| clock/ | 6.5 | 0.75 | 2.0 | **2.44** | ○ | later | — |
| parallel/ core | 7.5 | 0.95 | 3.0 | **2.38** | ○ | **E** | rayon |
| actor/ | 6.0 | 0.85 | 2.5 | **2.04** | ○ | later | actix |
| STM (Block-STM) | 7.5 | 0.95 | 4.0 | **1.78** | ○ | later(Rust!) | Block-STM |
| crdt/ | 5.5 | 0.75 | 3.0 | **1.38** | ○ | later | rust-crdt |

---

## 5. 3 個月 Optimal 序列(~11.5 週)

**策略**:syncx Rust-parity 廉價完成 + 3 個 deep Rust-correlated showcase + 1 個杜拜職位 anchor。

### Phase A — Rust-parity 廉價完成(3.0 週)
- [ ] syncx/Lock +CLH+Seqlock(1.0)— `parking_lot`/`seqlock`;Seqlock = 市場資料極限場景
- [ ] syncx/Semaphore +weighted/timeout(1.0)— `tokio::sync::Semaphore`
- [ ] syncx/Once(0.5)— `OnceCell`
- [ ] syncx/Latch +CountDownLatch(0.5)

### Phase B — Deep Rust-correlated showcases(6.5 週)← **portfolio 主力**
- [ ] **queue/** 剩餘:純 SPSC(rigtorp 風)+ Michael-Scott + benchmark(2.5)— `crossbeam-queue`;Bybit 撮合 + Coinbase LMAX
- [ ] **memory/** ordering + atomics + OnceCell(2.0)— `std::atomic Ordering`;Bybit infra 極限
- [ ] **syncx/Future + Promise**(2.0)— **Rust `Future` trait**(Rust async 入門信號最強)

### Phase C — 杜拜職位 anchor(2.0 週)
- [ ] **ratelimit/**(2.0)— Binance 確認面試題、VARA 合規、Tabby fraud(R_corr 低但 V 高,不能跳過)

**總計 11.5 週 ≈ 3 個月** ✓ → 「高並發 Go backend + lock-free 系統」portfolio,直接對應 Bybit/Binance/OKX/Tabby JD。

### 3 篇 blog(明確畫 Go→Rust 對應)
1. 「Go lock-free queue ≈ crossbeam:SPSC benchmark + cache-line 分析」
2. 「Memory ordering 實戰:Go atomic vs Rust `Ordering::Acquire/Release`」
3. 「從零實作 Future:Go channel 版 ≈ Rust `Future` trait + waker」

---

## 6. 6 個月極限延伸(+~11.5 週)

3 個月核心 + **crossbeam 全家桶**(Rust 系統相關性最高的 cluster):

### Phase D — crossbeam cluster(深 lock-free,7.0 週)
- [ ] **hazard/ + reclamation/**(3.0)— `crossbeam-epoch`;無 GC lock-free 回收 = Rust 最硬 signal
- [ ] **deque/** Chase-Lev(2.0)— **就是** `crossbeam-deque`
- [ ] **park/**(1.5)— `parking_lot_core` / tokio;runtime 地基
- [ ] (順便)queue/ 接 hazard pointer 做 Michael-Scott 安全回收

### Phase E — 並發容器 + 並行(4.5 週)
- [ ] **map/** sharded + sync.Map(2.0)— `dashmap`;撮合狀態、帳戶 map
- [ ] **parallel/** core + rayon-style join/scope(2.5)— `rayon`;G42 AI 橋接 + crypto parallel
- [ ] (選)scope/(2.0)— fintech/BNPL cancellation

**6 個月總計 ~23 週** ✓

### 額外 blog
4. 「crossbeam-epoch 在 Go:hazard pointer vs EBR」
5. 「Chase-Lev work-stealing deque:Go runtime ≈ crossbeam-deque」

---

## 7. 6 個月以後(若繼續)

按 ROI 遞減,且這些**最值得直接用 Rust 寫**(相關性最大化):
- **STM / Block-STM** — Aptos 是 Rust;認真打 crypto L1 就**直接用 Rust 寫**,signal 爆表
- **rcu/**(`arc-swap`)、**arena/**(`bumpalo`)、**actor/**(`actix`)
- **clock/ + crdt/** — crypto 分散式,R_corr 中等
- **syncx/Channel +pipeline** — 實用但 Rust 相關性最低
- **syncx/Barrier 進階(NCCL)** — 只有走 AI infra 才做,需配 Python

---

## 8. 累積 Signal 曲線

```
累積
Signal
  ▲                                    ___________  ← 邊際遞減(週 23+)
  │                          _________/
  │                  _______/  ← Phase D-E(crossbeam,Rust 頂峰)
  │           ______/
  │      ____/  ← Phase B-C 最陡(deep showcase + 杜拜 anchor)★ 投職位門檻
  │  ___/
  │_/  ← Phase A(廉價完成)
  └──────┬──────────┬──────────────────┬─────────▶ 週
        3          11(3個月)          23(6個月)
                "job-ready knee"    "Rust-systems 完整"
```

**數學驗證你的直覺**:週 3-11 斜率最陡(可投杜拜 crypto 職位);週 23 後遞減 → 3 個月 optimal、6 個月極限。

---

## 9. 誠實警示

1. **Go-only 對 AI infra 有天花板**:G42 深度 AI 是 Python。Go 版 `parallel/AllReduce` 是「概念 signal」不是 production match。真打 AI infra 要補 Python。
2. **HFT 是未來賭注**:`memory/`/`arena/`/`hazard/` 的最強買家(Citadel DIFC)**還沒落地**,押 2026+。但這些同時是 Rust crossbeam signal,不浪費。
3. **R_corr=1.0 ≠ 用 Go 寫就能拿 Rust 職位**:代表「概念高度 transfer」,但 Rust 職缺要看**真的 Rust code**。**強烈建議** 6 個月後把 1-2 個 showcase(queue 或 STM)**真的用 Rust 重寫** → 從 correlated 升級成 proven。
4. **ratelimit 矛盾**:V 最高(杜拜 #1 面試題)但 R_corr 最低之一(policy 層)。仍要做(硬門檻),但別期待它展示 Rust 系統深度。

---

# Part II — 參考:全球 Cross-Vertical Context

> 若你之後 pivot 出杜拜(SF / London / Singapore / 遠端),用這部分。資料同樣來自 250+ sources。

## 10. 跨 Vertical Signal 矩陣

> ★ = 在該 vertical 面試/JD/blog 被引用頻率 + 可實作性。

| Package / Item | HFT | Crypto | AI Infra | FAANG | Dubai | 全球 Composite |
|---|---|---|---|---|---|---|
| **queue/** (SPSC/MPSC/MPMC) | ★5 | ★5 | ★5 | ★5 | ★4 | **4.8 / T0** |
| **ratelimit/** | ★4 | ★4 | ★4 | ★5 | ★5 | **4.4 / T0** |
| **syncx/Channel** | ★4 | ★4 | ★4 | ★5 | ★4 | **4.2 / T0** |
| **parallel/** | ★3 | ★5 | ★5 | ★4 | ★3 | **4.0 / T0** |
| **map/** | ★3 | ★4 | ★4 | ★5 | ★3 | **3.8 / T1** |
| **syncx/Mutex+RWMutex** | ★5 | ★3 | ★3 | ★4 | ★3 | **3.6 / T1** |
| **syncx/Barrier**(基礎) | ★2 | ★3 | ★5 | ★5 | ★3 | **3.6 / T1** |
| **scope/** | ★3 | ★3 | ★4 | ★5 | ★3 | **3.6 / T1** |
| **syncx/WaitGroup+Latch** | ★3 | ★3 | ★3 | ★5 | ★3 | **3.4 / T1** |
| **syncx/Future** | ★3 | ★3 | ★4 | ★4 | ★3 | **3.4 / T1** |
| **syncx/Lock 進階**(MCS/TTAS/Seqlock) | ★5 | ★4 | ★3 | ★2 | ★3 | **3.4 / T1** |
| **memory/** | ★5 | ★4 | ★3 | ★2 | ★3 | **3.4 / T1** |
| **syncx/Semaphore** | ★3 | ★3 | ★3 | ★4 | ★4 | **3.4 / T1** |
| **syncx/Barrier 進階**(NCCL) | ★2 | ★3 | ★5 | ★2 | ★3 | **3.0 / T2** |
| **hazard/+reclamation/+rcu/** | ★3 | ★3 | ★4 | ★2 | ★3 | **3.0 / T2** |
| **actor/** | ★2 | ★3 | ★4 | ★3 | ★3 | **3.0 / T2** |
| **clock/** | ★2 | ★5 | ★2 | ★3 | ★3 | **3.0 / T2** |
| **crdt/** | ★1 | ★5 | ★2 | ★4 | ★3 | **3.0 / T2** |
| **arena/** | ★4 | ★3 | ★4 | ★1 | ★2 | **2.8 / T2** |
| **syncx/STM** | ★1 | ★5 | ★3 | ★2 | ★3 | **2.8 / T2** |
| **stack/** | ★3 | ★3 | ★2 | ★3 | ★2 | **2.6 / T2** |
| **syncx/Once** | ★2 | ★2 | ★3 | ★4 | ★2 | **2.6 / T2** |
| **park/** | ★2 | ★3 | ★3 | ★2 | ★3 | **2.6 / T3** |
| **deque/** | ★2 | ★2 | ★3 | ★3 | ★2 | **2.4 / T3** |
| **syncx/Cond** | ★2 | ★2 | ★3 | ★3 | ★2 | **2.4 / T3** |

> **注意**:全球 Tier 跟 Dubai Phase 不同。例:`syncx/STM` 全球 T2 但 crypto ★5;`memory/`/`MCS` 全球對 HFT ★5 但杜拜當前需求低(HFT 未落地)。Dubai Phase(Part I)已經把這些重新加權。

## 11. 全球 Per-Vertical Tracks(若 pivot)

- **HFT**(Citadel/HRT/Optiver/JS,C++ 為主):queue(SPSC)→ memory ordering → Lock 進階(MCS/Seqlock)→ arena/hazard → Disruptor。語言:C++ 主,Rust 成長。
- **Crypto/L1**(Aptos/Solana/Monad,Rust 80%):STM(Block-STM)→ clock+crdt → parallel → queue。**直接用 Rust 寫 signal 最強**。
- **AI Infra**(Anthropic/OpenAI/vLLM/Ray):parallel+AllReduce → Barrier 進階(NCCL)→ queue(vLLM scheduler)→ actor(Ray)→ ratelimit。語言:Python 主 + Rust 成長。
- **FAANG**(Meta/Google/Cloudflare/Discord):全 Tier 0 → scope(context)→ map(sync.Map)→ Future → crdt(Figma/Notion)。語言:Go/Java/Python,Rust at Cloudflare/Discord。

## 12. 方法論 / 來源

- 5 vertical 各 40-55 獨立 URL,合計 250+。
- 完整證據(JD 引用、面試題、論文、per-package 評分)在 [docs/research/](docs/research/):`hft.md` / `crypto.md` / `ai_infra.md` / `faang.md` / `dubai.md`。
- 每 6 個月應重評 — Tier 隨技術潮流變(2026:STM/AI infra 上升;lock-free 仍是穩定核心)。

---

# Part III — Per-Package Priority 索引

> 每個 package 的 TODO 開頭都有對應的 Priority block(Dubai Phase + ROI + R_corr + 全球 Tier)。

### Phase A(3 個月 · 廉價完成)
- [syncx/TODO_LOCK.md](syncx/TODO_LOCK.md) · [syncx/TODO_SEMAPHORE.md](syncx/TODO_SEMAPHORE.md) · [syncx/TODO_LATCH.md](syncx/TODO_LATCH.md) · (Once、Barrier-Cyclic 在各自 TODO)

### Phase B(3 個月 · deep showcase)★ portfolio 主力
- [queue/TODO.md](queue/TODO.md) · [memory/TODO.md](memory/TODO.md) · [syncx/TODO_FUTURE.md](syncx/TODO_FUTURE.md)

### Phase C(3 個月 · 杜拜 anchor)
- [ratelimit/TODO.md](ratelimit/TODO.md)

### Phase D(6 個月 · crossbeam cluster)
- [hazard/TODO.md](hazard/TODO.md) · [reclamation/TODO.md](reclamation/TODO.md) · [deque/TODO.md](deque/TODO.md) · [park/TODO.md](park/TODO.md)

### Phase E(6 個月 · 容器 + 並行)
- [map/TODO.md](map/TODO.md) · [parallel/TODO.md](parallel/TODO.md) · [scope/TODO.md](scope/TODO.md)

### Later(6 個月以後 / 換 Rust 寫)
- [syncx/TODO_STM.md](syncx/TODO_STM.md) · [rcu/TODO.md](rcu/TODO.md) · [arena/TODO.md](arena/TODO.md) · [actor/TODO.md](actor/TODO.md) · [clock/TODO.md](clock/TODO.md) · [crdt/TODO.md](crdt/TODO.md) · [syncx/TODO_CHANNEL.md](syncx/TODO_CHANNEL.md) · [syncx/TODO_BARRIERS.md](syncx/TODO_BARRIERS.md)

### Lab(教學,不影響 career 排序)
- [_lab/pattern/TODO.md](_lab/pattern/TODO.md) · [_lab/verify/TODO.md](_lab/verify/TODO.md) · [_lab/excercise/TODO.md](_lab/excercise/TODO.md)

---

## 一句話總結

> **3 個月**:`syncx 補完 + queue + memory + Future + ratelimit` → 投杜拜 crypto 職位 + 3 篇 Go↔Rust 對照 blog。
> **6 個月**:加 `hazard/reclamation + deque + park + map + parallel`(crossbeam 全家桶)→ Rust 系統相關性拉滿。
> **之後**:把 1-2 個用 **Rust 真的重寫**,從 correlated 升級成 proven。
