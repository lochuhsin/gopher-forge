# excercise — Classical Concurrency Puzzles

> **訓練主題**:**用既有 primitive 解問題**,而不是再刻新 primitive。
>
> **為什麼放在這裡**:每個 puzzle 大概 30–100 行,但都有教學陷阱。**做完一題就用兩三種 primitive 各刻一次**,會看到設計取捨。
>
> **跟其他 package 的關係**:這裡的 import 只有 `syncx/` + standard library。不寫新 primitive,只用既有的。

---

## 🎯 Priority(Dubai-focused)

> **Lab — 教學用,不在 career 實作序列內**。完整排序見 [ROADMAP.md](../../ROADMAP.md)。
> 做完 Phase A-E 之後若想練,可用 經典 puzzle,用既有 primitive 練組合。

---

## 推薦做法

**每題至少寫兩個版本**,例如:
- v1:用 `syncx.Cond` + `sync.Mutex`
- v2:用 `chan struct{}` + `select`
- v3:用 `syncx.Semaphore`

然後 benchmark 對比,寫成 README 註解。

---

## 1. 經典死鎖題

### 1.1 Dining Philosophers ⭐⭐⭐
五位哲學家圍桌,左右各一隻叉,需要兩隻才能吃飯。
- **陷阱**:全員同時拿左叉 → 死鎖
- **解法**:
  - Resource hierarchy(編號小的先拿)
  - Asymmetric(奇數先拿左、偶數先拿右)
  - Chandy-Misra(token passing)
  - Arbitrator(僕人)

### 1.2 Drinking Philosophers ⭐⭐⭐⭐
比 Dining 進階,每位哲學家需要不同瓶子組合。
- **訓練**:Chandy-Misra 演算法的完整版

---

## 2. 經典飢餓題

### 2.1 Readers-Writers ⭐⭐
共享資源,讀者可以共享、寫者獨占。
- **三個版本**:
  - First Readers-Writers(reader 優先,writer 可能餓死)
  - Second Readers-Writers(writer 優先,reader 可能餓死)
  - Fair / Third(FIFO,看到對方就讓)

### 2.2 Sleeping Barber ⭐⭐
理髮店有 N 椅,顧客來了無位就走,理髮師沒人就睡。
- **訓練**:Multi-waiter coordination,經典 condvar 題
- **陷阱**:狀態檢查跟入睡之間的 race

### 2.3 Unisex Bathroom ⭐⭐⭐
男女共用廁所:同時最多 N 人,但不能男女混用。
- **訓練**:互斥群體 + 容量限制
- **陷阱**:starvation(一群男的進去後源源不絕,女的進不去)

---

## 3. 經典協調題

### 3.1 Cigarette Smokers ⭐⭐⭐
三抽菸者各有一種材料(tobacco / paper / matches),agent 隨機放兩種上桌,擁有第三種的 smoker 拿走捲菸。
- **訓練**:**為什麼 semaphore 不夠用**(原題挑戰:不能改 agent code,但 semaphore 解不出來)
- **解法**:需要更高層的協調(broadcast / pattern matching)

### 3.2 Santa Claus ⭐⭐⭐⭐
Santa 睡覺,9 隻馴鹿到齊叫醒他出發送禮;若 3 隻精靈找他問問題,他也起床。馴鹿優先。
- **訓練**:多個 wait queue + 優先級,Trono 1994 經典
- **陷阱**:精靈正在問問題時,馴鹿到了怎麼辦

### 3.3 Roller Coaster ⭐⭐
N 個座位的車,**坐滿才開**,沒坐滿就等。
- **訓練**:Reusable barrier 的應用

---

## 4. 多角色 barrier-like 題

### 4.1 H2O Building ⭐⭐⭐
H 線程跟 O 線程,**每 2 H + 1 O 才能組成一個分子**,組完才放行。
- **訓練**:Group rendezvous,不只是「N 個都到」,是「特定角色組合」

### 4.2 Hydrogen-Oxygen-Bond ⭐⭐⭐
變體:H 跟 O 配對的細節控制(誰先 print 'H'、誰先 'O'、'O' 在 'H' 之間…)。
- **訓練**:LeetCode 多線程系列

### 4.3 Building H2O with errors ⭐⭐⭐⭐
基於 4.1,加上「中途出錯怎麼回滾已配對的同伴」(rollback semantics)。

---

## 5. LeetCode-style 多線程題

| 題目 | 訓練 |
|------|------|
| **Print in Order**(1114) | 三個函式按序執行 → 兩個 latch / channel |
| **Print FooBar Alternately**(1115) | 兩個 thread 輪流,交替印 |
| **Print Zero Even Odd**(1116) | 三個 thread 同步打印 010203... |
| **Fizz Buzz Multithreaded**(1195) | 4 個 thread 各印對應字串 |
| **Building H2O**(1117) | 同 4.1 |
| **Web Crawler Multithreaded**(1242) | 平行 BFS + visited set |
| **Traffic Light**(1279) | 兩條路十字路口的互斥 |
| **The Dining Philosophers**(1226) | 同 1.1 |

---

## 6. 橋 / 渡輪 / 過河題

### 6.1 Crossing Bridge ⭐⭐⭐
窄橋一次最多 N 人,方向相反不能同時。
- **訓練**:方向控制 + 容量

### 6.2 Niagara Falls Tour Boat ⭐⭐⭐
船有 4 個位,2H + 2C 或 4H 才能開,不能 3H + 1C(怕欺負小孩)。
- **訓練**:組合約束的協調

### 6.3 Cannibals & Missionaries ⭐⭐⭐⭐
河邊 N 食人族 + N 傳教士,船每次載 1-2 人。**任何時候 cannibals > missionaries 在任一岸 → missionary 被吃**。
- **訓練**:全域 invariant 維護(這題其實是 search 題,但可以做成 thread 協調版)

---

## 7. 進階 / 自由發揮

| 題目 | 訓練 |
|------|------|
| **Web Crawler with rate limit** | 接 `ratelimit/` |
| **Pub-Sub system** | actor + topic 路由,接 `actor/` |
| **Distributed Cache(simulated)** | LRU + 多 reader/writer |
| **Mini Redis(commands + transactions)** | 接 `syncx/STM` 或 cond/mutex |
| **K-V store with snapshot isolation** | 接 `clock/HLC` |

---

## 建議實作順序

```
1. Print in Order              ← 暖身,2 個 latch
2. Dining Philosophers         ← 死鎖經典(3 種解法都做)
3. Readers-Writers(3 個版本)   ← 飢餓 / 公平性
4. Sleeping Barber             ← Condvar 應用
5. Cigarette Smokers           ← 看 semaphore 的極限
6. H2O Building                ← Group rendezvous
7. Roller Coaster              ← Reusable barrier
8. Santa Claus                 ← 多 queue + 優先級
9. Cannibals & Missionaries    ← Invariant 維護
10. (自由發揮)mini Redis / web crawler 等
```

---

## 參考資料

- Downey, *"The Little Book of Semaphores"* — 上面大部分題的標準教材
- Dijkstra 原始論文(Dining Philosophers, Sleeping Barber)
- LeetCode Concurrency 系列
- Trono 1994 *"A New Exercise in Concurrency"*(Santa Claus 原題)

---

## Career signal

- **Senior infra 面試**:Dining Philosophers / Readers-Writers / Sleeping Barber 是 onsite 白板的範圍
- **HFT / Crypto exchange**:Cigarette Smokers 啟發的「**為什麼 semaphore 不夠**」是好的 storytelling
- **每題寫 3 種解法 + benchmark**,signal 比「我做了一個 lock-free queue」更稀有

---

## Career Signal (Cross-Vertical Research)

> 來源:`docs/research/{hft,crypto,ai_infra,faang,dubai}.md`(250+ sources 聚合)。對應 [ROADMAP.md](../../ROADMAP.md) **Tier 2(Composite ★2.8)**。

### Scoring Matrix

| Vertical | Rating | Tier | Top Evidence |
|----------|--------|------|--------------|
| **HFT** | ★3/5 | Required | DE Shaw confirmed "five philosophers eating at a circular table" GeeksforGeeks; Optiver onsite includes Readers-Writers under pressure; Dining Philosophers = standard whiteboard at prop trading firms |
| **Crypto** | ★2/5 | Niche | Cigarette Smokers reasoning ("why semaphore isn't enough") maps to smart contract reentrancy intuition; not directly tested |
| **AI Infra** | ★2/5 | Niche | H2O Building = Group rendezvous analogy for AllReduce barrier synchronization; Readers-Writers maps to model weight read vs gradient update write |
| **FAANG** | ★4/5 | Required | Producer-Consumer = standard at ALL FAANG firms; Readers-Writers (three variants) = senior Go concurrency question; Dining Philosophers = L5/L6 whiteboard; LeetCode 1114-1195 multithreaded series |
| **Dubai** | ★2/5 | Niche | Standard entry-level concurrency verification; not a differentiator |
| **Composite** | **★2.8/5.0** | **Tier 2** | — |

### 必要(Required for senior infra interviews)

> 集中在 **HFT** 和 **FAANG** 的白板題範圍:

- **Dining Philosophers (3 解法)** — resource hierarchy / asymmetric / Chandy-Misra;DE Shaw confirmed as onsite whiteboard
  - Evidence: [HFT research](../../docs/research/hft.md) — DE Shaw: "five philosophers eating at a circular table" confirmed; deadlock reasoning required at all HFT prop trading firms
- **Readers-Writers (3 個版本)** — reader優先 / writer優先 / FIFO Fair;飢餓 / 公平性分析
  - Evidence: [FAANG research](../../docs/research/faang.md) — "Readers-Writers three variants = senior Go concurrency question"; RWMutex design maps to Second Readers-Writers

### 進階(Advanced / Senior-to-Staff Differentiator)

> 在 **1-2 個 vertical** 是 differentiator。

- **Cigarette Smokers** — 展示「為什麼 semaphore 不夠用」;是面試 storytelling 大殺器
  - Best for: HFT / Crypto (maps to "why compare-and-swap is not enough for certain coordination problems")
- **Santa Claus** — 多 wait queue + 優先級;Trono 1994;展示 composite primitive 設計能力
  - Best for: FAANG (L6 design discussion: priority queues + conditional wakeup)
- **H2O Building** — Group rendezvous;特定角色組合才放行;LeetCode 1117
  - Best for: AI Infra (AllReduce barrier = "N特定角色都就緒才繼續"); FAANG LeetCode multithreaded series

### Recommended Order(本 package 內部)

1. Print in Order(暖身,2 個 latch)
2. Dining Philosophers(3 種解法)
3. Readers-Writers(3 個版本)
4. Sleeping Barber(Condvar 應用)
5. Cigarette Smokers(semaphore 的極限)
6. H2O Building(Group rendezvous)
7. Roller Coaster(Reusable barrier)
8. Santa Claus(多 queue + 優先級)
9. Cannibals & Missionaries(Invariant 維護)
10. 自由發揮(mini Redis / web crawler)

### 對應的 Blog 題材(若想寫)

- "Dining Philosophers 三種解法的設計哲學:為什麼 resource hierarchy 是最常用的"
- "Cigarette Smokers:為什麼這題讓 semaphore 失敗 — 從 puzzle 到 smart contract reentrancy"
