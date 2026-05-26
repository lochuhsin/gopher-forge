# Concurrent Map Family TODO

> Category 抽象: Associative container (key → value),thread-safe lookup / insert / delete。
> **Note:** 對你目前的 target (HFT > AI infra > Crypto L1) **ROI 較低** — 看 `docs/purpose/syncx_career_value.md`。建議優先級放後。
> 對廣義 FAANG senior infra (thread-safe LRU 等) ROI 較高。

## 🎯 Priority(Dubai-focused)

| Dubai Phase | ROI | V_dubai | R_corr | 剩餘工時 | 全球 Tier |
|---|---|---|---|---|---|
| **E(6mo·容器)** | **3.40** | 8.0/10 | 0.85 `dashmap` | 2.0 週 | T1 |

> 撮合引擎狀態、帳戶 map;sharded + sync.Map 自刻版。 完整排序見 [ROADMAP.md](../ROADMAP.md)。

---

## 核心 invariant

- Get / Put / Delete 在 happens-before 下 linearizable
- Resize 是 concurrent map 獨有的難題:rehash 期間既要服務 old table 又要 migrate 到 new table
- Hash collision 處理: chaining (link list) vs open addressing (linear / quadratic probe)

## Inventory

| Variant | Lock 策略 | Resize 策略 | Status |
|---|---|---|---|
| **Striped lock map** | Per-bucket mutex (N stripes) | stop-the-world | TODO (簡單入門) |
| **Java ConcurrentHashMap** | striping + treebin on collision | concurrent (helping migrate) | TODO |
| **Cliff Click NonBlockingHashMap** | open addressing + CAS state machine | concurrent | TODO |
| **Lock-free skip list (Fraser / Pugh)** | CAS marking + helping | n/a (sorted, no resize) | TODO |
| **CTrie (Concurrent Trie)** | hash-trie + GCAS | snapshot-able | TODO (進階) |
| **Read-mostly cache (sync.Map 風格)** | dirty / read 雙 map | append-mostly 優化 | TODO |

## Variant contracts (TODO)

### Striped Lock Map (簡單入門)

**Signature:**
```
type StripedMap[K comparable, V any] struct {
    stripes []stripe[K, V]   // power-of-2 count
    mask    uint64
}
type stripe[K comparable, V any] struct {
    mu      sync.Mutex
    buckets map[K]V
}
```

**Contract:**
- 64 / 128 個 stripe,每個 stripe 一個 mutex + 自己的 map
- 操作: `s = &stripes[hash(k) & mask]; s.mu.Lock(); ...`
- Resize: 不做,或 per-stripe 個別 resize

**Key insight:** 把 contention 從「1 個 mutex」拆成「N 個 mutex」,代價是無法做 atomic operation 跨 stripe (例如 `Size()` 必須 lock 全部 stripe)。實用且簡單,Java `ConcurrentHashMap` 1.7 之前就是這個。

### Java ConcurrentHashMap (Java 8+)

**Contract:**
- Open chaining,bucket array;collision 多時把 list 轉成紅黑樹 (TREEBIN)
- Insert: CAS bucket head (空時) 或 lock bucket head (非空時)
- Resize: incremental — 任何 reader / writer 看到 forwarding node 就幫忙 migrate 自己的 bucket
- *Reader 不阻塞 writer,writer 不全域阻塞 reader*

**Key insight:** "helping migrate" — resize 不是 stop-the-world,而是把工作分攤到所有 reader / writer。這跟 lock-free queue 的 helping protocol 同精神。

**Reference:** Doug Lea, `java.util.concurrent.ConcurrentHashMap` (Java 8 完整改寫)。

### Cliff Click NonBlockingHashMap

**Contract:**
- Open addressing (linear probe)
- Slot 是 (key, value) state machine: `EMPTY → CLAIMED_K → KV → TOMBSTONE`
- Insert: linear probe 找 EMPTY,CAS EMPTY → CLAIMED_K → KV
- Resize: 觸發時建 new table,reader 同時看 old + new (有 "see new table" flag)
- *Reader 看到舊 table 也對* — 只是 trigger migrate

**Key insight:** State machine + CAS-only,沒有 lock。Resize 是 concurrent (reader 幫忙 migrate)。在 read-mostly workload 上是 Java 最快的 concurrent map。

**Reference:** Cliff Click 2007 JavaOne talk "A Lock-Free Wait-Free Hash Table",`org.cliffc.high_scale_lib.NonBlockingHashMap`。

### Lock-free Skip List

**Contract:**
- Ordered map (支援 range query)
- Insert: 從底層找位置 → CAS bottom level node → 逐層往上 CAS upper level pointers
- Delete: mark node (logical delete) → unlink (physical)
- 需要 reclamation

**Reference:** Pugh 1990 (sequential skip list),Fraser PhD 2003 (lock-free version),Sundell & Tsigas 2003。

### CTrie (Concurrent Trie)

**Contract:**
- Hash-array mapped trie + GCAS (generation CAS) 保證 snapshot 一致
- 支援 lock-free snapshot (即取得當前狀態的 immutable view)
- 用於 Scala `TrieMap`

**Reference:** Prokopec, Bagwell, Odersky 2011 "Cache-Aware Lock-Free Concurrent Hash Tries"。

### sync.Map 風格 (Go-specific)

**Contract:**
- 兩個內部 map: `read` (atomic.Value, immutable snapshot) + `dirty` (mutex-protected)
- 大多數 read 直接走 read map (no lock)
- Write 走 dirty map;miss read 達閾值 → promote dirty 成新 read
- 適合 *append-mostly* + *讀少寫少改頻繁* 的特定 workload (其他情況不如 striped map)

**Reference:** Go stdlib `sync.Map` source code。

## 跨語言對照

| 語言 | Variant |
|---|---|
| Java | `ConcurrentHashMap` (striped + tree),`NonBlockingHashMap` (Cliff Click) |
| C++ | Folly `ConcurrentHashMap` (split-order list),TBB `concurrent_hash_map` |
| Rust | `dashmap` (striped),`flurry` (Java CHM port),`scc` (lock-free) |
| Go | `sync.Map` (read/dirty 雙 map),`xsync.Map` 等第三方 |
| Erlang | `ets` table (mutex-protected, ets:lookup is lock-free for read_concurrency) |

## Use case 對應

- **HFT order book:** 不是 — order book 是 price-ladder array
- **CEX matching:** 不是 — matching 是 single-threaded per pair
- **Crypto account state:** 概念上是 — 但實作通常 sharded + MVCC
- **AI infra:** parameter server, model registry
- **FAANG senior:** thread-safe LRU 是 Meta E5 / Google L5 必考。LRU = concurrent map + linked list (or Caffeine 風格 W-TinyLFU)

## Career signal (低,但有特例)

- **HFT / AI infra 直接 ROI 低**
- **FAANG senior interview 高** — thread-safe LRU 必考
- **Crypto L1 中度** — account state 概念匹配,但實作不同

## 建議

如果之後要做 order book / LRU cache side project,再回來補。**不建議當「為了學而學」的下一步** — 同樣時間投資 `reclamation/` 或 `queue/SPSC` 更值錢。

## Recommended order (若決定要做)

1. **Striped Lock Map** (簡單,實用,夠 80% 場景)
2. **sync.Map 風格** (Go 標準庫對照,learn-by-rewriting)
3. **Lock-free skip list** (ordered map + reclamation 練習)
4. **Cliff Click NonBlockingHashMap** (進階)
5. **Java ConcurrentHashMap port** (concurrent resize 學習)

## Dependencies

- Lock-free 版本 → `reclamation/` (HP 或 EBR)
- 全部 lock-free → `memory/`
- Skip list 跟 `queue/` priority queue 共用底層 (skip list ordered structure)

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../ROADMAP.md) **Tier 1(Composite ★3.8)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★3/5 | Required (concept) / Advanced (impl) | GTS Glassdoor: "Why might array be faster than unordered_map?"; concurrent hash map is standard system design sub-question |
| **Crypto** | ★4/5 | Required (practical) | Block-STM uses Dashmap (sharded concurrent hash map) as its multi-version data structure; Solana AccountsDB = concurrent hash map |
| **AI Infra** | ★4/5 | Advanced | Parameter server, model registry, KV cache prefix map |
| **FAANG** | ★5/5 | Required | "`sync.Map` vs sharded mutex map" = standard Go senior question; Java `ConcurrentHashMap` = senior Java staple at Snowflake/Databricks/LinkedIn |
| **Dubai** | ★3/5 | Required | Common in crypto service account state management |
| **Composite** | **★3.8/5.0** | **Tier 1** | — |

### 必要(Required for senior infra interviews)

> 在 **≥2 個 vertical** 被列為 Required,或 composite ≥ 3.4。

- **Striped mutex map (`shardedMap[T]`)** — 64-128 stripe;每個 stripe 一個 mutex;實用且面試標準
  - Evidence: [FAANG research](../docs/research/faang.md) — "`sync.Map` vs sharded mutex map is a standard Go senior interview question"
- **sync.Map 風格 (自刻教學版)** — read/dirty 雙 map;append-mostly 優化;Go runtime 對照
  - Evidence: FAANG ★★★★★; [Crypto research](../docs/research/crypto.md) — Block-STM uses Dashmap (Rust equivalent of sharded map)

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator。

- **Robin Hood hashing** — open-addressing with variance reduction;NUMA-friendly;Crypto L1 signal
  - Best for: Crypto (Block-STM multi-version data structure discussion; why Dashmap vs RwLock<HashMap>)
- **Lock-free open-addressing (Cliff Click NonBlockingHashMap)** — CAS-only state machine;讀不阻塞寫
  - Best for: HFT (C++ Folly equivalent discussion); AI Infra (parameter server hot path)
- **Lock-free skip list** — ordered map + range query;reclamation 練習;Sundell-Tsigas 2003
  - Best for: FAANG (sorted concurrent map; interval query in Databricks context)

### Recommended Order(本 package 內部)

1. Striped Lock Map(簡單,實用)
2. sync.Map 風格(Go 標準庫對照)
3. Lock-free skip list(接 reclamation)
4. Cliff Click NonBlockingHashMap(進階)
5. Java ConcurrentHashMap port(concurrent resize 學習)

### 對應的 Blog 題材(若想寫)

- "`sync.Map` 解析:什麼時候比 sharded mutex map 快,什麼時候慢"
- "Block-STM 為什麼選 Dashmap:sharded 並發 hash map 在 L1 區塊鏈的應用"
