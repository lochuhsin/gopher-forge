# verify — Concurrency Verification Tools

> **訓練主題**:**「我怎麼知道我的 lock-free 結構真的對?」** 寫完 primitive 之後,自刻**驗證工具**比再多 unit test 都有效。
>
> **為什麼獨立 package**:
> - 跟所有其他 package 正交 — verify 是「**工具**」不是「primitive」或「data structure」
> - 每個工具都是獨立演算法(linearizability check / lockset / etc.),有教學價值
> - 可以**反向用來驗證自己的 queue / stack / map** 是否正確

---

## 🎯 Priority(Dubai-focused)

> **Lab — 教學用,不在 career 實作序列內**。完整排序見 [ROADMAP.md](../../ROADMAP.md)。
> 做完 Phase A-E 之後若想練,可用 linearizability checker 拿來測自己的 queue/stack 是否正確。

---

## 核心概念

```
寫 concurrent 結構的人:               用 verify 工具的人:
"我的 queue 是 lock-free,測試 PASS"  "讓我用 history 跑 linearizability check"
"我覺得這個 lock 沒問題"              "讓我跑 wait-for graph 找 cycle"
"我覺得這段 race-free"                "讓我跑 lockset algorithm 找 shared var"
```

**直覺不可靠;工具會抓出你沒想到的 case。**

---

## Inventory

| 名稱 | 抓什麼 | 演算法 |
|------|--------|--------|
| **Linearizability Checker** | concurrent operation 序列**是否能線性化** | Wing-Gong 1993 / Lowe 2017 |
| **Sequential Consistency Checker** | 比 linearizability 弱一級的正確性 | 同 Wing-Gong 變體 |
| **Litmus Test Framework** | 在多平台跑 reorder-sensitive code,統計觀察到的行為 | herd / litmus7 風格 |
| **Property-based Test Runner** | 隨機 schedule + invariant check | QuickCheck for concurrency |
| **Deadlock Detector(static)** | Lock-order graph + cycle finder | go-deadlock 風格 |
| **Deadlock Detector(runtime)** | 攔截 Lock/Unlock,動態建 wait-for graph,DFS cycle | Pthread mutex chain |
| **Eraser Lockset** | 每個 shared var 持有「保護它的鎖集合」,集合空了 → 報 race | Savage 1997 |
| **Happens-Before Detector** | Vector clock 在每個 goroutine,debug race | `go -race` 內部 |
| **Schedule Explorer (DPOR)** | 系統性枚舉所有可能 schedule 找 bug | Flanagan-Godefroid 2005 |

---

## 建議實作順序

```
1. Linearizability Checker(brute-force,小 history)
       ↓
2. Property-based Test Runner(用 quick check + 隨機 schedule)
       ↓
3. Wait-for Graph Deadlock Detector(runtime 攔截 Lock)
       ↓
4. Eraser Lockset(Savage 1997 的最小實作)
       ↓
5. Litmus Test Framework(寫一組 SC/TSO/PSO 的微 code,跑 N 次統計)
       ↓
6. Happens-Before Detector(自刻 mini -race)
       ↓
7. DPOR / Schedule Explorer(進階,可選)
```

---

## 每個工具的測試對象(反向應用)

| 工具 | 拿來測這個 package |
|------|--------------------|
| Linearizability | `queue/`, `stack/`, `map/` 的所有 lock-free 實作 |
| Lockset | `syncx/cond` 的 wait queue 操作 |
| Wait-for Graph | `syncx/lock` 的死鎖測試 |
| HB Detector | 任何手刻 atomic 序列,validate memory ordering |
| Litmus | `memory/` 的 release/acquire 練習 |

---

## 設計重點(每個工具的核心難題)

### Linearizability Checker
- **History 怎麼記錄**:每個 operation 有 (start_time, end_time, op, result)
- **核心演算法**:枚舉所有「合法」線性順序,看哪個能解釋觀察到的結果
- **複雜度**:NP-hard 一般情況,但 small history 可以 brute-force(Wing-Gong)
- **參考**:Jepsen 的 Knossos / Porcupine(Go 版,直接讀原始碼)

### Wait-for Graph Detector
- **怎麼攔 Lock**:wrap 你的 `Locker` interface,記錄 (goroutine_id, holding_lock, waiting_for_lock)
- **建圖**:有向圖 `goroutine → lock_waiting → owner_goroutine`
- **找環**:Tarjan SCC 或 DFS
- **gotcha**:Go 沒有公開的 goroutine_id,要用 `runtime.Callers` 或 `runtime.Stack` 提取

### Lockset / Eraser
- **State machine**:每個 shared var 在「virgin / exclusive / shared / shared-modified」之間轉換
- **核心**:每次 access,intersect 當前線程持有的 lockset 跟 var 的 lockset
- **gotcha**:會 false positive(被「barrier-like」protection 騙)

### Litmus Test
- **microbenchmark 結構**:兩個 goroutine 各做特定 store/load,統計 N 次後看哪些結果出現
- **參考 herd / litmus7**(formal memory model tool)的 test 寫法
- **教學價值**:跑出來才知道「Go memory model」實際允許什麼

---

## Dependencies

- → `clock/` 的 vector clock(happens-before detector 內部)
- → `syncx/` 的所有 lock 跟 atomic
- 反向被 **所有** package 使用(測試自己)

---

## Career signal

- **Jepsen 風格分散式測試** — Aphyr / Kyle Kingsbury 的招牌,影響整個產業
- **Database infra**(FoundationDB / CockroachDB / TiDB)都跑類似工具測自己
- 自刻 linearizability checker + 用它找到「自己 queue 的 bug」是面試大殺器
- **TLA+ / Coq** 是 formal verification 的下一步,做完這套是入門

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../../ROADMAP.md) **Tier 2(Composite ★2.8)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★3/5 | Advanced | HRT prep: "Sanitizers (ASan/TSan/UBSan) appropriateness" as interview topic; Optiver: correctness under stress testing; linearizability knowledge = signal at prop trading firms |
| **Crypto** | ★3/5 | Advanced | FoundationDB / CockroachDB / TiDB all run Jepsen-style verification; Block-STM needs conflict detection = linearizability reasoning |
| **AI Infra** | ★2/5 | Niche | Distributed training checkpoint consistency needs linearizability-style reasoning; TSan for framework-level races |
| **FAANG** | ★3/5 | Advanced | Jepsen-style correctness reasoning = L6+ signal at Amazon/Google; "design a correct rate limiter" implies linearizability; go -race = standard; Porcupine (Go linearizability checker) open-sourced by Anish Athalye |
| **Dubai** | ★2/5 | Niche | Exchange correctness testing at Binance/Bybit; not mainstream interview question |
| **Composite** | **★2.8/5.0** | **Tier 2** | — |

### 必要(Required for senior infra interviews)

> 本 package 的必要項集中在 correctness reasoning 能力展示:

- **Linearizability Checker** — brute-force Wing-Gong on small history;用來驗證自己的 queue/ stack/ map 實作
  - Evidence: [FAANG research](../../docs/research/faang.md) — Porcupine (Go linearizability checker) cited as production-grade tool; Jepsen correctness = L6+ interview signal at Amazon/Google
- **Wait-for Graph Deadlock Detector (runtime)** — 攔截 Lock/Unlock;Tarjan SCC 找環;go-deadlock 風格
  - Evidence: [HFT research](../../docs/research/hft.md) — HRT: "TSan/ASan/UBSan appropriateness" = interview topic; deadlock prevention = required at all HFT firms

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator。

- **Eraser Lockset (Savage 1997)** — shared var 的 lockset 交集;mini TSan;go -race 的底層原理
  - Best for: HFT (race-free guarantees at nanosecond precision); FAANG (explain how -race works = L6 question)
- **Litmus Test Framework** — SC/TSO/PSO 微 code 統計;validate `memory/` 的 release/acquire 練習
  - Best for: HFT (memory ordering litmus tests are standard at prop trading firms)
- **DPOR (Dynamic Partial Order Reduction)** — 系統性枚舉所有可能 schedule;Flanagan-Godefroid 2005
  - Best for: Crypto (Block-STM schedule space exploration); FAANG (formal methods L7 discussion)

### Recommended Order(本 package 內部)

1. Linearizability Checker(brute-force,小 history)
2. Property-based Test Runner(random schedule + invariant)
3. Wait-for Graph Deadlock Detector(runtime 攔截 Lock)
4. Eraser Lockset(Savage 1997 最小實作)
5. Litmus Test Framework(SC/TSO/PSO 統計)
6. Happens-Before Detector(自刻 mini -race)
7. DPOR / Schedule Explorer(進階可選)

### 對應的 Blog 題材(若想寫)

- "自刻 linearizability checker + 找到自己 lock-free queue 的 bug:go -race 不夠用的時候"
- "Eraser Lockset = go -race 的前身:Savage 1997 到現代 TSan 的演進"
