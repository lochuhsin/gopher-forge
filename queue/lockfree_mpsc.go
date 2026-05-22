package queue

import "sync/atomic"

type LockFreeMPSC[T any] struct {
	tail atomic.Uint64
	head uint64
	arr  [BoundedQueueSize]slot[T]
}

func NewLockFreeMPSC[T any]() *LockFreeMPSC[T] {
	q := &LockFreeMPSC[T]{}
	for i := range BoundedQueueSize {
		q.arr[i].seq.Store(uint64(i))
	}
	return q
}

func (b *LockFreeMPSC[T]) Enqueue(v T) bool {
	var (
		tail uint64
		slot *slot[T]
		seq  uint64
	)

	for {
		tail = b.tail.Load()
		slot = &b.arr[tail&queueMask]
		seq = slot.seq.Load()

		diff := int64(seq) - int64(tail)

		if diff < 0 {
			return false
		}

		if diff == 0 {
			if b.tail.CompareAndSwap(tail, tail+1) {
				slot.val = v
				slot.seq.Store(tail + 1)
				return true
			}
		}
	}
}

func (b *LockFreeMPSC[T]) Dequeue() (T, bool) {
	var zero T
	slot := &b.arr[b.head&queueMask]

	seq := slot.seq.Load()
	if seq != b.head+1 {
		return zero, false
	}

	v := slot.val
	slot.val = zero

	slot.seq.Store(b.head + BoundedQueueSize)
	b.head++
	return v, true
}
