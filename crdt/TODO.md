# crdt — Conflict-free Replicated Data Types

> **訓練主題**:**「不需要協調」就能保證最終一致**。多副本各自改,任意順序合併都收斂到同一個值。
>
> **為什麼獨立 package**:
> - 跟 syncx primitive 正交 — CRDT 不是同步,是「**設計可交換的更新**」
> - 數學基礎是 lattice / join-semilattice,自成一個體系
> - 每個 CRDT 都是 case study,值得獨立檔案

---

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **later(6mo+)** | **1.38** | 5.5/10 | 0.75 `rust-crdt` | 3.0 週 | T2(crypto★5/FAANG★4) |

> crypto 分散式 + Figma/Notion 協作;但 R_corr 中等,工時大。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心概念

CRDT 必須滿足三個性質(對 merge):
1. **Commutative**:`a ⊕ b = b ⊕ a`(順序無關)
2. **Associative**:`(a ⊕ b) ⊕ c = a ⊕ (b ⊕ c)`(分組無關)
3. **Idempotent**:`a ⊕ a = a`(重複合併無害)

三個一起 → 可以亂序、重複、丟訊息,只要訊息**最終**都到了,所有副本收斂。

---

## 兩大流派

```
                  CRDT
                /      \
        State-based    Op-based
        (CvRDT)        (CmRDT)
        ────           ────
        傳整個 state   傳每個 operation
        merge 用 lattice  必須 reliable + causal delivery
        簡單但 bandwidth 大  bandwidth 小但需要 broadcast 保證
```

---

## Inventory

### 計數器類

| 名稱 | 操作 | 限制 |
|------|------|------|
| **G-Counter** (grow-only) | Increment only | 不能減 |
| **PN-Counter** | +/- | 兩個 G-Counter 拼 |
| **Bounded Counter** | +/-,有上下界 | 需要 reservation 機制(escrow) |

### 集合類

| 名稱 | 操作 | 限制 |
|------|------|------|
| **G-Set** (grow-only) | Add only | 不能 remove |
| **2P-Set** | Add + Remove(remove 後不能再 add) | tombstone |
| **OR-Set** (Observed-Remove) | Add + Remove 自由 | 每個 add 帶 unique tag |
| **LWW-Element-Set** | Add + Remove,timestamp 大者贏 | 需 HLC / wall clock |

### 暫存器類

| 名稱 | 操作 | 衝突解決 |
|------|------|---------|
| **LWW-Register** | Last-Writer-Wins | timestamp 大者贏 |
| **MV-Register** (Multi-Value) | 並發寫保留多值 | 上層應用決定怎麼挑 |

### Map / 結構類

| 名稱 | 操作 | 注意 |
|------|------|------|
| **OR-Map** | Key-Value,key 是 OR-Set,value 是 CRDT | Map of CRDTs |
| **CRDT JSON** | Map + List + Register 巢狀 | Automerge / Yjs 的核心 |

### 序列 / List

| 名稱 | 注意 |
|------|------|
| **RGA** (Replicated Growable Array) | Insert with origin pointer |
| **LSEQ** | Distributed identifier for ordering |
| **Yjs / YATA** | 工業界主流(Yjs / Automerge) |

---

## 建議實作順序

```
1. G-Counter            ← 最小:就是 max of vectors
2. PN-Counter           ← 兩個 G-Counter
3. G-Set                ← max of sets (union)
4. OR-Set               ← 第一個有趣的 CRDT(tag-based)
5. LWW-Register         ← 用上 clock/ 的 HLC
6. OR-Map               ← Map of CRDTs,compose 概念
7. RGA / LSEQ           ← 序列,Google Docs / Notion 等級
8. Delta-CRDT           ← 優化:只傳 diff 不傳整個 state
```

---

## 跟其他 package 的依賴

- → `clock/`(LWW-Register 需要 HLC;OR-Set 的 tag 通常用 (replica_id, counter))
- → `syncx/`(本地 replica 的並發保護)
- ← `verify/` 的 property test:可驗證 commutativity / associativity / idempotence

---

## 學習資源

- **Shapiro et al. 2011** "Conflict-free Replicated Data Types" (CRDT 開山論文)
- **Automerge** / **Yjs** 原始碼(JSON CRDT 工業實作)
- **Riak DT** (Erlang,CRDT 的第一個 production)
- **Roshi**(SoundCloud 的 LWW-Set,Go 寫的)

---

## Career signal

- **Notion / Figma / Linear** infra 都用 CRDT 做 collaborative editing
- **Edge databases**(Cloudflare D1、Turso)在用 CRDT 做多區域複製
- 寫一篇「**為什麼 OR-Set 比 2P-Set 強**」+ benchmark,signal 強過 80% 求職 portfolio

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 2(Composite ★3.0,但 Crypto ★5 / FAANG ★4)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★1/5 | Not Required | Zero evidence of CRDT being tested at any HFT firm; potential only for multi-datacenter risk aggregation at Two Sigma/DE Shaw research |
| **Crypto** | ★5/5 | Advanced / Very Strong Signal | Sui object model is CRDT-inspired; Narwhal mempool gossip = CRDT-convergent; Solana read-only accounts = CRDT-readable |
| **AI Infra** | ★2/5 | Niche (growing) | Multi-datacenter inference routing table convergence; geo-distributed feature flag propagation |
| **FAANG** | ★4/5 | Advanced (growing) | Amazon SDE3 explicitly lists CRDTs; "Design Google Docs" = CRDT canonical answer in 2024+; Figma/Notion/Linear collaborative products |
| **Dubai** | ★3/5 | Advanced | Convergent replicated state at Binance/Bybit multi-region; VARA compliance state |
| **Composite** | **★3.0/5.0** | **Tier 2** | — |

### 必要(Required for senior infra interviews)

> 本 package 的必要項集中在 **Crypto L1** 和 **FAANG collaborative system design**:

- **G-Counter + OR-Set** — 最基礎的 CRDT;展示 join-semilattice 理解;面試 "design collaborative counter" 基礎
  - Evidence: [FAANG research](../docs/research/faang.md) — "Amazon SDE3 explicitly lists CRDTs"; "Design Google Docs = CRDT canonical answer in 2024+"
- **LWW-Register** — 與 clock/ 的 HLC 組合;Last-Writer-Wins 是最常見的衝突解決策略
  - Evidence: [Crypto research](../docs/research/crypto.md) — Wormhole guardian set updates = convergent replicated state

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator。

- **OR-Map** — Map of CRDTs;Sui object model 的數學基礎
  - Best for: Crypto L1 (Sui物件模型 = CRDT-inspired ownership; Narwhal gossip DAG = strongly eventual consistency)
- **RGA / LSEQ** — Replicated Growable Array;collaborative text editing (Google Docs level)
  - Best for: FAANG (Figma/Notion/Linear design interviews; Martin Kleppmann CRDT papers)
- **Delta-CRDT** — 只傳 diff 不傳整個 state;bandwidth optimization
  - Best for: Crypto (P2P validator state propagation optimization)

### Recommended Order(本 package 內部)

1. G-Counter(最小可行)
2. PN-Counter(兩個 G-Counter)
3. G-Set → OR-Set(tag-based remove)
4. LWW-Register(接 clock/)
5. OR-Map(Map of CRDTs)
6. RGA / LSEQ(序列 CRDT)
7. Delta-CRDT(優化傳輸)

### 對應的 Blog 題材(若想寫)

- "Sui 物件模型 = CRDT:為什麼 single-owner object 不需要共識就能並發"
- "Design Google Docs 的 2024 答案:OR-Set + RGA 比 OT 好在哪裡"