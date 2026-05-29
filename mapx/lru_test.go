package mapx

import (
	"sync"
	"testing"
)

// ---------- SyncLRU[T, V] ----------
//
// A fixed-capacity, concurrency-safe LRU cache.
//
// Contract under test:
//  1. Capacity bound: len(cache) never exceeds the cap passed to NewSyncLRU.
//  2. Lookup: Get(k) returns (value, true) for a live key, (zero, false)
//     otherwise.
//  3. Insert/overwrite: Put(k, v) inserts k, or overwrites an existing k's
//     value. An overwrite updates the value and counts as a use — it does NOT
//     grow the cache and does NOT create a duplicate node.
//  4. Recency: both Put and Get mark k most-recently-used. When the cache is
//     full, the next insert of a NEW key evicts the least-recently-used key.
//  5. Concurrency: all of the above hold under concurrent callers. The cache
//     must be free of data races (run with -race) AND keep its internal
//     doubly-linked list consistent with its map at every lock release.
//
// NOTE — concurrency safety is NOT correctness. A single mutex serializes
// access, so -race passes even when the list surgery is wrong. The
// list-integrity invariant (checkIntegrity) is what actually catches a
// mis-spliced pointer — the bug class a mutex does not protect you from.

// ---------- white-box helpers ----------
//
// These read internal state directly (same package). contains/size probe the
// MAP, so unlike Get they do NOT mutate recency — essential when asserting
// eviction order.

func size[T comparable, V any](l *SyncLRU[T, V]) int { return len(l.cache) }

func contains[T comparable, V any](l *SyncLRU[T, V], key T) bool {
	_, ok := l.cache[key]
	return ok
}

func mustGet[T, V comparable](t *testing.T, l *SyncLRU[T, V], key T, want V) {
	t.Helper()
	got, ok := l.Get(key)
	if !ok {
		t.Fatalf("Get(%v) = _, false; want %v, true", key, want)
	}
	if got != want {
		t.Fatalf("Get(%v) = %v; want %v", key, got, want)
	}
}

func mustMiss[T comparable, V any](t *testing.T, l *SyncLRU[T, V], key T) {
	t.Helper()
	if got, ok := l.Get(key); ok {
		t.Fatalf("Get(%v) = %v, true; want miss", key, got)
	}
}

// checkIntegrity verifies the doubly-linked list is internally consistent and
// agrees with the map. It is written for your TWO-SENTINEL layout: head and
// tail are dummy nodes that always exist; real nodes live strictly between them
// (head.next .. tail.prev), and an empty cache has head.next == tail. The
// invariants are design-independent; only the traversal is layout-specific:
//   - sentinels always exist; head.prev == nil and tail.next == nil
//   - real-node count (between the sentinels) == len(cache); no cycle
//   - every cached key maps to the exact node found in the list (same pointer)
//   - links are symmetric: n.next.prev == n for every link
func checkIntegrity[T comparable, V any](t *testing.T, l *SyncLRU[T, V]) {
	t.Helper()

	if l.head == nil || l.tail == nil {
		t.Fatal("head/tail sentinel is nil — NewSyncLRU must allocate both")
	}
	if l.head.prev != nil {
		t.Fatal("head.prev != nil; the head sentinel must be the absolute front")
	}
	if l.tail.next != nil {
		t.Fatal("tail.next != nil; the tail sentinel must be the absolute end")
	}

	seen := make(map[*node[T, V]]bool, len(l.cache))
	count := 0
	for n := l.head.next; n != l.tail; n = n.next {
		if n == nil {
			t.Fatal("forward walk hit nil before the tail sentinel — list is severed")
		}
		if seen[n] {
			t.Fatalf("cycle detected: node for key %v visited twice", n.key)
		}
		seen[n] = true

		if n.next == nil || n.next.prev != n {
			t.Fatalf("links not symmetric at key %v (n.next.prev != n)", n.key)
		}
		if got, ok := l.cache[n.key]; !ok || got != n {
			t.Fatalf("list node key %v is not the same pointer the cache holds", n.key)
		}
		count++
	}
	if count != len(l.cache) {
		t.Fatalf("list has %d real nodes but cache has %d entries", count, len(l.cache))
	}
	if count > int(l.cap) {
		t.Fatalf("size %d exceeds cap %d", count, l.cap)
	}
}

// ---------- functional correctness ----------

// The most fundamental test: a freshly constructed cache must already be a
// consistent (empty) list. If this fails, NewSyncLRU did not wire up sentinels
// and nothing downstream can work.
func TestSyncLRUNewIsConsistentAndEmpty(t *testing.T) {
	l := NewSyncLRU[string, int](4)
	if got := size(l); got != 0 {
		t.Fatalf("fresh cache size = %d, want 0", got)
	}
	mustMiss(t, l, "absent")
	checkIntegrity(t, l)
}

func TestSyncLRUBasicPutGet(t *testing.T) {
	l := NewSyncLRU[string, int](2)
	l.Put("a", 1)
	l.Put("b", 2)

	mustGet(t, l, "a", 1)
	mustGet(t, l, "b", 2)
	mustMiss(t, l, "c")
	checkIntegrity(t, l)
}

// Overwriting a live key updates its value without growing the cache or
// duplicating its list node. Catches a Put that lacks an "already present"
// branch and blindly appends a second node.
func TestSyncLRUOverwriteUpdatesValueNotSize(t *testing.T) {
	l := NewSyncLRU[string, int](2)
	l.Put("a", 1)
	l.Put("a", 100)

	if got := size(l); got != 1 {
		t.Fatalf("size after overwrite = %d, want 1", got)
	}
	mustGet(t, l, "a", 100)
	checkIntegrity(t, l)
}

// The defining LRU behavior: when full, inserting a new key evicts the
// least-recently-used one. contains() probes the map so it does not itself
// disturb recency.
func TestSyncLRUEvictsLeastRecentlyUsed(t *testing.T) {
	l := NewSyncLRU[int, int](3)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3) // full; recency (LRU→MRU): 1, 2, 3
	l.Put(4, 4) // evicts 1; now: 2, 3, 4

	if contains(l, 1) {
		t.Fatal("key 1 (LRU) should have been evicted")
	}
	for _, k := range []int{2, 3, 4} {
		if !contains(l, k) {
			t.Fatalf("key %d should still be present", k)
		}
	}
	if got := size(l); got != 3 {
		t.Fatalf("size = %d, want 3 (== cap)", got)
	}
	checkIntegrity(t, l)
}

// Get must refresh recency. After touching key 1, it is MRU, so the next
// insert evicts key 2 (the new LRU) and key 1 survives.
func TestSyncLRUGetRefreshesRecency(t *testing.T) {
	l := NewSyncLRU[int, int](3)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3)         // LRU→MRU: 1, 2, 3
	mustGet(t, l, 1, 1) // touch 1 → LRU→MRU: 2, 3, 1
	l.Put(4, 4)         // evicts 2

	if contains(l, 2) {
		t.Fatal("key 2 should have been evicted after key 1 was refreshed by Get")
	}
	if !contains(l, 1) {
		t.Fatal("key 1 should survive — Get must refresh recency")
	}
	checkIntegrity(t, l)
}

// Re-Putting a live key also refreshes recency (it is a use, not just a write).
func TestSyncLRUPutRefreshesRecency(t *testing.T) {
	l := NewSyncLRU[int, int](3)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3)
	l.Put(1, 11) // overwrite + touch 1 → LRU→MRU: 2, 3, 1
	l.Put(4, 4)  // evicts 2

	if contains(l, 2) {
		t.Fatal("key 2 should have been evicted after key 1 was re-Put")
	}
	mustGet(t, l, 1, 11)
	checkIntegrity(t, l)
}

func TestSyncLRUCapacityOne(t *testing.T) {
	l := NewSyncLRU[int, int](1)
	l.Put(1, 1)
	l.Put(2, 2) // evicts 1

	mustMiss(t, l, 1)
	mustGet(t, l, 2, 2)
	if got := size(l); got != 1 {
		t.Fatalf("size = %d, want 1", got)
	}
	checkIntegrity(t, l)
}

// Capacity-zero is a contract DECISION, not a given. This test encodes the
// "a zero-capacity cache stores nothing" interpretation: every Put is a no-op
// and every Get misses. If you instead choose to reject n==0 in the
// constructor (panic / error), delete this test and test that instead.
func TestSyncLRUCapacityZeroStoresNothing(t *testing.T) {
	l := NewSyncLRU[int, int](0)
	l.Put(1, 1)
	l.Put(2, 2)

	mustMiss(t, l, 1)
	mustMiss(t, l, 2)
	if got := size(l); got != 0 {
		t.Fatalf("size = %d, want 0", got)
	}
	checkIntegrity(t, l)
}

// ---------- edge cases ----------

// Filling exactly to capacity must evict nothing — eviction begins only when a
// further DISTINCT key arrives. Guards the off-by-one where eviction fires at
// len==cap and the cache silently holds only cap-1 entries.
func TestSyncLRUFillToCapEvictsNothing(t *testing.T) {
	l := NewSyncLRU[int, int](4)
	for k := range 4 {
		l.Put(k, k)
	}
	if got := size(l); got != 4 {
		t.Fatalf("size = %d after filling to cap, want 4", got)
	}
	for k := range 4 {
		if !contains(l, k) {
			t.Fatalf("key %d evicted while merely filling to cap", k)
		}
	}
	checkIntegrity(t, l)
}

// Touching the SOLE node (head == tail) is the tightest boundary: that node has
// nil prev AND nil next. The remove+reinsert must not panic or self-loop.
func TestSyncLRUGetSoleElement(t *testing.T) {
	l := NewSyncLRU[int, int](2)
	l.Put(7, 70)
	mustGet(t, l, 7, 70) // move-to-MRU on the only node
	if got := size(l); got != 1 {
		t.Fatalf("size = %d, want 1", got)
	}
	checkIntegrity(t, l)
}

// Touching the current MRU (tail) node: removeNode runs on a node whose next is
// nil, and a naive impl writes l.tail.next into the very node it is relocating,
// creating a self-loop. Recency order must be unchanged afterward.
func TestSyncLRUTouchMostRecentNode(t *testing.T) {
	l := NewSyncLRU[int, int](3)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3)         // LRU→MRU: 1, 2, 3
	mustGet(t, l, 3, 3) // 3 is already MRU; order must stay 1, 2, 3
	checkIntegrity(t, l)

	l.Put(4, 4) // must still evict 1 (the LRU)
	if contains(l, 1) {
		t.Fatal("touching the MRU node wrongly changed which key is LRU")
	}
	for _, k := range []int{2, 3, 4} {
		if !contains(l, k) {
			t.Fatalf("key %d missing after touching MRU", k)
		}
	}
	checkIntegrity(t, l)
}

// Touching a MIDDLE node exercises removeNode with both links non-nil and the
// relink of its two former neighbors.
func TestSyncLRUTouchMiddleNode(t *testing.T) {
	l := NewSyncLRU[int, int](3)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3)         // LRU→MRU: 1, 2, 3
	mustGet(t, l, 2, 2) // move middle → LRU→MRU: 1, 3, 2
	checkIntegrity(t, l)

	l.Put(4, 4) // evicts 1
	if contains(l, 1) {
		t.Fatal("key 1 should be LRU and evicted")
	}
	for _, k := range []int{2, 3, 4} {
		if !contains(l, k) {
			t.Fatalf("key %d missing after touching middle", k)
		}
	}
	checkIntegrity(t, l)
}

// A stored ZERO value must come back as (zero, true) — "present" rides on the
// bool, never inferred from the value. Classic cache bug: treating the zero
// value as "absent".
func TestSyncLRUStoresZeroValue(t *testing.T) {
	l := NewSyncLRU[int, int](2)
	l.Put(1, 0)

	got, ok := l.Get(1)
	if !ok {
		t.Fatal("Get(1) = _, false; a stored zero value must report present")
	}
	if got != 0 {
		t.Fatalf("Get(1) = %d, want 0", got)
	}
	checkIntegrity(t, l)
}

// Re-inserting a previously evicted key must behave as a fresh insert — proof
// that eviction fully detached the old node from BOTH the map and the list (no
// stale node lingering to corrupt the count or recency).
func TestSyncLRUReinsertEvictedKey(t *testing.T) {
	l := NewSyncLRU[int, int](2)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3) // evicts 1 → {2,3}
	if contains(l, 1) {
		t.Fatal("key 1 should have been evicted")
	}

	l.Put(1, 100) // re-insert the evicted key; evicts 2 → {3,1}
	mustGet(t, l, 1, 100)
	mustMiss(t, l, 2)
	if got := size(l); got != 2 {
		t.Fatalf("size = %d, want 2", got)
	}
	checkIntegrity(t, l)
}

// ---------- concurrency (run with -race) ----------

// Many goroutines Put and Get concurrently. Each key k always stores the same
// value (k*10), so any successful Get must observe exactly that — a torn or
// stale read shows up as a value mismatch. After the storm: no race (enforced
// by -race), size within cap, list still consistent.
//
// t.Errorf (not Fatalf) is used inside goroutines: FailNow/Fatal from a
// non-test goroutine is illegal.
func TestSyncLRUConcurrentNoRaceOrCorruption(t *testing.T) {
	const (
		capacity   = 64
		goroutines = 32
		ops        = 2000
		keyspace   = 256 // > cap, so eviction is constantly in play
	)
	l := NewSyncLRU[int, int](capacity)

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Go(func() {
			for i := range ops {
				k := (g*ops + i) % keyspace
				if i%2 == 0 {
					l.Put(k, k*10)
				} else if v, ok := l.Get(k); ok && v != k*10 {
					t.Errorf("Get(%d) = %d, want %d", k, v, k*10)
				}
			}
		})
	}
	wg.Wait()

	if got := size(l); got > capacity {
		t.Fatalf("size = %d exceeds cap %d", got, capacity)
	}
	checkIntegrity(t, l)
}

// Tiny cache, large keyspace, many writers → maximum eviction pressure. The
// point is the invariant: no interleaving may push size past cap or corrupt
// the list.
func TestSyncLRUConcurrentSizeStaysWithinCap(t *testing.T) {
	const (
		capacity   = 8
		goroutines = 16
		ops        = 5000
		keyspace   = 1024
	)
	l := NewSyncLRU[int, int](capacity)

	var wg sync.WaitGroup
	for g := range goroutines {
		wg.Go(func() {
			for i := range ops {
				l.Put((g*31+i)%keyspace, i)
				l.Get((g*17 + i) % keyspace)
			}
		})
	}
	wg.Wait()

	if got := size(l); got > capacity {
		t.Fatalf("size %d exceeds cap %d", got, capacity)
	}
	checkIntegrity(t, l)
}

// All goroutines hammer the SAME key, maximizing contention on a single node's
// move-to-MRU path: that one node is removed and reinserted under the lock from
// every goroutine at once. A correct impl keeps exactly one node, value intact,
// list uncorrupted.
func TestSyncLRUConcurrentHotKey(t *testing.T) {
	const (
		goroutines = 32
		ops        = 4000
	)
	l := NewSyncLRU[int, int](8)

	var wg sync.WaitGroup
	for range goroutines {
		wg.Go(func() {
			for range ops {
				l.Put(0, 7)
				if v, ok := l.Get(0); ok && v != 7 {
					t.Errorf("Get(0) = %d, want 7", v)
				}
			}
		})
	}
	wg.Wait()

	if got := size(l); got != 1 {
		t.Fatalf("size = %d, want exactly 1 (only one key was ever inserted)", got)
	}
	mustGet(t, l, 0, 7)
	checkIntegrity(t, l)
}

// Repeats the storm over many fresh caches to shake out rare interleavings a
// single run misses. Gated by -short, matching the repo's stress-test pattern.
func TestSyncLRUConcurrentStress(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped under -short")
	}
	const (
		runs       = 50
		capacity   = 16
		goroutines = 24
		ops        = 1500
		keyspace   = 128
	)
	for range runs {
		l := NewSyncLRU[int, int](capacity)
		var wg sync.WaitGroup
		for g := range goroutines {
			wg.Go(func() {
				for i := range ops {
					k := (g*ops + i) % keyspace
					l.Put(k, k*10)
					if v, ok := l.Get(k); ok && v != k*10 {
						t.Errorf("Get(%d) = %d, want %d", k, v, k*10)
					}
				}
			})
		}
		wg.Wait()

		if got := size(l); got > capacity {
			t.Fatalf("size %d exceeds cap %d", got, capacity)
		}
		checkIntegrity(t, l)
	}
}
