package stack

import "sync"

type MutexLinkedMPMC[T any] struct {
	head *node[T]
	mu   sync.Mutex
}

func NewMutexLinkedMPMC[T any]() *MutexLinkedMPMC[T] {
	return &MutexLinkedMPMC[T]{}
}

func (b *MutexLinkedMPMC[T]) Pop() (T, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var zero T
	if b.head == nil {
		return zero, false
	}

	val := b.head.v
	b.head = b.head.next
	return val, true
}

func (b *MutexLinkedMPMC[T]) Push(v T) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := &node[T]{v: v}

	if b.head == nil {
		b.head = n
		return
	}

	n.next = b.head
	b.head = n
}
