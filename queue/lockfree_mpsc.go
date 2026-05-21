package queue

import "sync/atomic"

type LockFreeMPSC struct {
	tail atomic.Uint64
	head uint64
	arr  [BoundedQueueSize]slot
}

func NewLockFreeMPSC() *LockFreeMPSC {
	q := &LockFreeMPSC{}

	for i := range BoundedQueueSize {
		q.arr[i].seq.Store(uint64(i))
	}

	return q
}

func (b *LockFreeMPSC) Enqueue(v int) bool {
	var (
		tail uint64
		slot *slot
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

func (b *LockFreeMPSC) Dequeue() (int, bool) {
	slot := &b.arr[b.head&queueMask]

	seq := slot.seq.Load()
	if seq != b.head+1 {
		return 0, false
	}

	v := slot.val

	slot.seq.Store(b.head + BoundedQueueSize)
	b.head++
	return v, true
}
