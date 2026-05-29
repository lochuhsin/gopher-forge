package mapx

import "sync"

type node[T comparable, V any] struct {
	key  T
	val  V
	prev *node[T, V]
	next *node[T, V]
}

// sentinel implementation
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

	if len(l.cache) >= int(l.cap) {
		evict := l.head.next
		l.removeNode(evict)
		delete(l.cache, evict.key)
	}

	node := &node[T, V]{key: key, val: value}
	l.appendTail(node)
	l.cache[key] = node
}

func (l *SyncLRU[T, V]) Get(key T) (V, bool) { // update order
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

	// clean up reference
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
