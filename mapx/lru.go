package mapx

import "sync"

type node[T comparable, V any] struct {
	key  T
	val  V
	prev *node[T, V]
	next *node[T, V]
}

// SyncLRU is a fixed-capacity, concurrency-safe LRU cache. It pairs a hash
// map (O(1) key lookup) with an intrusive doubly-linked list (O(1) recency
// reordering). The list uses two dummy sentinel nodes, head and tail, that
// always exist; real nodes live strictly between them. head.next is the
// least-recently-used victim; tail.prev is the most-recently-used. An empty
// cache has head.next == tail.
//
// Pros:
//   - Both Get and Put are O(1): a map lookup plus a constant number of
//     pointer splices. No scan, no shifting.
//   - Sentinels remove every nil / empty / head / tail boundary
//     special-case: a real node's prev/next are never nil (worst case they
//     point at a sentinel), so removeNode and appendTail are unconditional
//     splices.
//   - Zero per-operation allocation except the one node created when a new
//     key is inserted.
//
// Cons:
//   - A single Mutex serializes every operation. Get MUTATES the list
//     (move-to-MRU), so it is a write, not a read — a sync.RWMutex with
//     RLock in Get would let concurrent Gets splice the list at once and
//     corrupt it. The plain Mutex is therefore correct, but it means there
//     is no read parallelism; a sharded or amortized-recency design would
//     be needed to scale reads.
//   - Recency is exact (every touch reorders), which costs more than an
//     approximate policy (CLOCK / second-chance) under heavy read load.
//
// Mental model — all ops linearize at the single Mutex:
//
//	Put(k, v):
//	    if cap == 0:        no-op
//	    if k present:       update value, move node to MRU (tail.prev)
//	    else:
//	        if full:        evict head.next (LRU), delete from map
//	        new node -> append at MRU, insert into map
//
//	Get(k):
//	    if k present:       move node to MRU, return (val, true)
//	    else:               return (zero, false)
//
// Substrate: ordinary Go map + a hand-rolled intrusive doubly-linked list,
// with sync.Mutex as the baseline serialization primitive.
type SyncLRU[T comparable, V any] struct {
	cap   uint64
	cache map[T]*node[T, V]
	head  *node[T, V]
	tail  *node[T, V]

	mu sync.Mutex
}

func NewSyncLRU[T comparable, V any](n uint) *SyncLRU[T, V] {
	head := &node[T, V]{}
	tail := &node[T, V]{}
	head.next = tail
	tail.prev = head
	return &SyncLRU[T, V]{cap: uint64(n), cache: make(map[T]*node[T, V], n), head: head, tail: tail}
}

func (l *SyncLRU[T, V]) Put(key T, value V) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.cap == 0 {
		return
	}

	if n, ok := l.cache[key]; ok {
		n.val = value
		l.removeNode(n)
		l.appendTail(n)
		return
	}

	if uint64(len(l.cache)) >= l.cap {
		evict := l.head.next
		l.removeNode(evict)
		delete(l.cache, evict.key)
	}

	node := &node[T, V]{key: key, val: value}
	l.appendTail(node)
	l.cache[key] = node
}

func (l *SyncLRU[T, V]) Get(key T) (V, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()

	var zero V
	if len(l.cache) == 0 {
		return zero, false
	}

	if node, ok := l.cache[key]; ok {
		l.removeNode(node)
		l.appendTail(node)
		return node.val, true
	}
	return zero, false
}

func (l *SyncLRU[T, V]) removeNode(n *node[T, V]) {
	if n == nil {
		return
	}
	n.prev.next = n.next
	n.next.prev = n.prev

	n.next = nil
	n.prev = nil
}

func (l *SyncLRU[T, V]) appendTail(n *node[T, V]) {
	last := l.tail.prev

	last.next = n
	n.prev = last
	n.next = l.tail

	l.tail.prev = n
}
