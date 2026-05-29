package mapx

import (
	"sync"
	"testing"
)

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

func TestSyncLRUEvictsLeastRecentlyUsed(t *testing.T) {
	l := NewSyncLRU[int, int](3)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3)
	l.Put(4, 4)

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

func TestSyncLRUGetRefreshesRecency(t *testing.T) {
	l := NewSyncLRU[int, int](3)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3)
	mustGet(t, l, 1, 1)
	l.Put(4, 4)

	if contains(l, 2) {
		t.Fatal("key 2 should have been evicted after key 1 was refreshed by Get")
	}
	if !contains(l, 1) {
		t.Fatal("key 1 should survive — Get must refresh recency")
	}
	checkIntegrity(t, l)
}

func TestSyncLRUPutRefreshesRecency(t *testing.T) {
	l := NewSyncLRU[int, int](3)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3)
	l.Put(1, 11)
	l.Put(4, 4)

	if contains(l, 2) {
		t.Fatal("key 2 should have been evicted after key 1 was re-Put")
	}
	mustGet(t, l, 1, 11)
	checkIntegrity(t, l)
}

func TestSyncLRUCapacityOne(t *testing.T) {
	l := NewSyncLRU[int, int](1)
	l.Put(1, 1)
	l.Put(2, 2)

	mustMiss(t, l, 1)
	mustGet(t, l, 2, 2)
	if got := size(l); got != 1 {
		t.Fatalf("size = %d, want 1", got)
	}
	checkIntegrity(t, l)
}

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

func TestSyncLRUGetSoleElement(t *testing.T) {
	l := NewSyncLRU[int, int](2)
	l.Put(7, 70)
	mustGet(t, l, 7, 70)
	if got := size(l); got != 1 {
		t.Fatalf("size = %d, want 1", got)
	}
	checkIntegrity(t, l)
}

func TestSyncLRUTouchMostRecentNode(t *testing.T) {
	l := NewSyncLRU[int, int](3)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3)
	mustGet(t, l, 3, 3)
	checkIntegrity(t, l)

	l.Put(4, 4)
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

func TestSyncLRUTouchMiddleNode(t *testing.T) {
	l := NewSyncLRU[int, int](3)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3)
	mustGet(t, l, 2, 2)
	checkIntegrity(t, l)

	l.Put(4, 4)
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

func TestSyncLRUReinsertEvictedKey(t *testing.T) {
	l := NewSyncLRU[int, int](2)
	l.Put(1, 1)
	l.Put(2, 2)
	l.Put(3, 3)
	if contains(l, 1) {
		t.Fatal("key 1 should have been evicted")
	}

	l.Put(1, 100)
	mustGet(t, l, 1, 100)
	mustMiss(t, l, 2)
	if got := size(l); got != 2 {
		t.Fatalf("size = %d, want 2", got)
	}
	checkIntegrity(t, l)
}

func TestSyncLRUConcurrentNoRaceOrCorruption(t *testing.T) {
	const (
		capacity   = 64
		goroutines = 32
		ops        = 2000
		keyspace   = 256
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
