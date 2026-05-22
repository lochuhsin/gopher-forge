package queue

import "sync"

type MutexMPSC[T any] struct {
	mu   sync.Mutex
	arr  [BoundedQueueSize]T
	tail uint64
	head uint64
}

func NewMutexMPSC[T any]() *MutexMPSC[T] {
	return &MutexMPSC[T]{}
}

func (b *MutexMPSC[T]) Enqueue(v T) bool {
	b.mu.Lock()

	if (b.tail - b.head) == BoundedQueueSize {
		b.mu.Unlock()
		return false
	}

	pos := b.tail & queueMask
	b.arr[pos] = v

	b.tail++
	b.mu.Unlock()
	return true
}

func (b *MutexMPSC[T]) Dequeue() (T, bool) {
	b.mu.Lock()

	var zero T
	if (b.tail - b.head) == 0 {
		b.mu.Unlock()
		return zero, false
	}

	pos := b.head & queueMask
	val := b.arr[pos]
	b.arr[pos] = zero

	b.head++
	b.mu.Unlock()
	return val, true
}
