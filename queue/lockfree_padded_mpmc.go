package queue

import "sync/atomic"

type LockFreePaddedMPMC[T any] struct {
	arr [BoundedQueueSize]slot[T]
	_   [120]byte // 64 - 8

	head atomic.Uint64
	_    [120]byte // 64 - 8

	tail atomic.Uint64
}

func NewLockFreePaddedMPMC[T any]() *LockFreePaddedMPMC[T] {
	q := &LockFreePaddedMPMC[T]{}
	for i := range BoundedQueueSize {
		q.arr[i].seq.Store(uint64(i))
	}
	return q
}

func (b *LockFreePaddedMPMC[T]) Enqueue(v T) bool {
	var (
		slot *slot[T]
		tail uint64
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
				break
			}
		}
	}

	slot.val = v
	slot.seq.Store(tail + 1)
	return true
}

func (b *LockFreePaddedMPMC[T]) Dequeue() (T, bool) {
	var (
		slot *slot[T]
		head uint64
		seq  uint64
		zero T
	)
	for {
		head = b.head.Load()
		slot = &b.arr[head&queueMask]
		seq = slot.seq.Load()

		diff := int64(seq) - int64(head+1)

		if diff < 0 {
			return zero, false
		}

		if diff == 0 {
			if b.head.CompareAndSwap(head, head+1) {
				val := slot.val
				slot.val = zero
				slot.seq.Store(head + BoundedQueueSize)
				return val, true
			}
		}
	}
}
