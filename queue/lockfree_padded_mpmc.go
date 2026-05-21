package queue

import "sync/atomic"

type LockFreePaddedMPMC struct {
	arr [BoundedQueueSize]slot
	_   [120]byte // 64 - 8

	head atomic.Uint64
	_    [120]byte // 64 - 8

	tail atomic.Uint64
}

func NewLockFreePaddedMPMC() *LockFreePaddedMPMC {
	q := &LockFreePaddedMPMC{}
	for i := range BoundedQueueSize {
		q.arr[i] = slot{}
		q.arr[i].seq.Store(uint64(i))
	}
	return q
}

func (b *LockFreePaddedMPMC) Enqueue(v int) bool {
	var (
		slot *slot
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

func (b *LockFreePaddedMPMC) Dequeue() (int, bool) {
	var (
		slot *slot
		head uint64
		seq  uint64
		val  int
	)
	for {
		head = b.head.Load()
		slot = &b.arr[head&queueMask]
		seq = slot.seq.Load()

		diff := int64(seq) - int64(head+1)

		if diff < 0 {
			return 0, false
		}

		if diff == 0 {
			if b.head.CompareAndSwap(head, head+1) {
				val = slot.val
				slot.seq.Store(head + BoundedQueueSize)
				return val, true
			}
		}
	}
}
