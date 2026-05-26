# clock — Logical Clocks

> **訓練主題**:把「**happens-before**」從抽象概念變成你能 `print()` 出來的物件。
>
> **為什麼獨立 package**:
> - 跟 syncx primitive 正交 — clock 不是同步原語,是**因果關係的表達**
> - 對於理解 memory model / distributed system 的本質有獨立教學價值
> - 多個 clock 演算法分屬不同 trade-off,自成一個小領域

---

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **later(6mo+)** | **2.44** | 6.5/10 | 0.75 | 2.0 週 | T2(crypto★5) |

> crypto 分散式(Block-STM 版本=Lamport)+ VARA 合規 timestamp。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心概念

```
真實時間 (wall clock):  T1 < T2 不代表 T1 happens-before T2
                       (clock skew、相對論、NTP 跳變)

邏輯時鐘 (logical):    用 counter / vector 表達「因果關係」
                       Lamport: ≤ 比較,只能推單向(causal → numeric)
                       Vector:  ≤ 比較,能推雙向 + concurrent
```

關鍵問題:**兩個事件是 happens-before 還是 concurrent**?

---

## Inventory

| 名稱 | 一句話 | 比較能力 | 大小 |
|------|--------|---------|------|
| **Lamport Clock** | 單一 counter,recv 取 max(local, msg)+1 | 偏序 → 全序(會誤判 concurrent 為 ordered) | O(1) |
| **Vector Clock** | 每個 process 一個 counter,recv 取 element-wise max | 真實 happens-before vs concurrent | O(n) |
| **Matrix Clock** | 每個 process 持有「其他人 vector clock 的最近值」 | 可推斷「對方知道我知道什麼」 | O(n²) |
| **Hybrid Logical Clock (HLC)** | physical time + logical counter,單調且接近 wall clock | causal + 近 wall clock | O(1) |
| **Interval Tree Clock (ITC)** | 動態 process 加入/離開,不需預先知道 N | 因果 + 動態 | O(log n) |
| **Bloom Clock** | Probabilistic vector clock,空間 sublinear | 機率性,可能 false-positive | O(k) |

---

## 經典練習(每個 clock 對應一題)

1. **Lamport**:模擬 5 個 process 互傳訊息,印出每個事件的 timestamp,**找出哪些事件被誤判為 ordered**
2. **Vector**:在同樣 trace 下,**正確識別** concurrent vs causal pairs
3. **HLC**:用一個跳變的假 physical clock(模擬 NTP jump back),驗證 HLC 仍然單調
4. **Matrix**:實作 garbage collection — 「**所有人都已經知道**某個訊息」就可以刪除

---

## 設計重點

- **Send rule**:發送前 `local++`,附帶 local 到訊息
- **Recv rule**:收到時 `local = max(local, msg) + 1`(Lamport)/ element-wise max + own++(Vector)
- **Compare**:Vector v1 ≤ v2 iff ∀i, v1[i] ≤ v2[i];v1 < v2 iff ≤ 且不相等;否則 concurrent
- **儲存壓縮**:Vector clock 的稀疏表示(很多 process 通常 0)

---

## 建議實作順序

```
1. Lamport Clock           ← 最小可行,寫一組 trace 測試
2. Vector Clock            ← 加 happens-before 比較
3. Causal Broadcast        ← 用 vector clock 確保接收順序
4. Hybrid Logical Clock    ← physical + logical 混合
5. Matrix Clock            ← 進階:推斷對方知識
6. Interval Tree Clock     ← 進階:動態 process
```

---

## 跨領域對照

| 領域 | 應用 |
|------|------|
| **Distributed DB** | CockroachDB 用 HLC、DynamoDB 用 vector clock、Spanner 用 TrueTime |
| **CRDT** | OR-Set / LWW 內部都用 logical clock 解衝突 |
| **Event Sourcing** | Kafka offset 是退化的 Lamport clock |
| **Memory Model** | happens-before relation 跟 vector clock 同構 — 編譯器/race detector 內部用類似結構 |
| **Git** | DAG of commits 是另一種 partial order,概念近 vector clock |

---

## Dependencies

- → `syncx/atomic` 計數器
- ← `crdt/`(OR-Set / LWW 必須用 vector 或 HLC)
- ← `verify/` 的 happens-before checker 可以借用 vector clock 結構

---

## Career signal

- **Distributed DB infra**(CockroachDB / TiDB / FoundationDB)— HLC 是 day-1 概念
- **AI infra**:訓練 checkpoint 的「最近 consistent state」依賴類似時間戳
- **跨系統 audit log**:寫一篇「vector clock 是 race detector 內部結構」的 blog 有強 signal

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 2(Composite ★3.0,但 Crypto ★5)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★2/5 | Not Required at execution firms | No execution HFT interview asked about logical clocks; relevant at research teams (Two Sigma, DE Shaw) |
| **Crypto** | ★5/5 | Required at L1 | Block-STM version numbers = Lamport timestamps; Narwhal DAG rounds = vector clock; Wormhole/IBC cross-chain message ordering requires causal order |
| **AI Infra** | ★2/5 | Niche | Training checkpoint "latest consistent state" uses clock analogies; distributed trace IDs have Lamport-like ordering |
| **FAANG** | ★3/5 | Advanced | Amazon SDE3 explicitly lists "vector clocks"; Google TrueTime (Spanner) = L6 context |
| **Dubai** | ★3/5 | Advanced | Binance/Bybit cross-chain coordination; distributed exchange ordering |
| **Composite** | **★3.0/5.0** | **Tier 2** | — |

### 必要(Required for senior infra interviews)

> 本 package 的必要項集中在 **Crypto L1** vertical:

- **Lamport Clock** — 最小因果序;Block-STM 版本號 = Lamport-style logical timestamp
  - Evidence: [Crypto research](../docs/research/crypto.md) — "Block-STM versioning (Lamport-equivalent timestamps)"; dYdX CometBFT uses block heights as logical clocks
- **Vector Clock** — 真實 happens-before vs concurrent;Narwhal DAG rounds = vector clock principle
  - Evidence: [FAANG research](../docs/research/faang.md) — "Amazon SDE3 explicitly lists vector clocks as tested topic"; [Narwhal Mysten Labs](https://github.com/MystenLabs/narwhal)

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator。

- **HLC (Hybrid Logical Clock)** — physical time + logical counter;CockroachDB/Spanner;cross-chain bridges
  - Best for: Crypto (Wormhole/IBC cross-chain message ordering; AlpenGlow certificate chains = causally ordered); FAANG (Google TrueTime L6 context)
- **Matrix Clock** — 推斷「對方知道我知道什麼」;GC 舊訊息
  - Best for: Crypto (Narwhal gossip mempool state propagation; validator state awareness)

### Recommended Order(本 package 內部)

1. Lamport Clock(最小可行)
2. Vector Clock(happens-before 比較)
3. Causal Broadcast(用 vector clock)
4. HLC(physical + logical)
5. Matrix Clock(進階)

### 對應的 Blog 題材(若想寫)

- "Block-STM 的版本號 = Lamport clock:從分散式系統論文到區塊鏈並行執行"
- "Narwhal DAG 為什麼等於 vector clock:因果關係在 L1 共識的應用"