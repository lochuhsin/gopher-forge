package queue

import "sync"

type MutexMPMC struct {
	mu   sync.Mutex
	arr  [BoundedQueueSize]int
	tail uint64
	head uint64
}

func NewMutexMPMC() *MutexMPMC {
	return &MutexMPMC{}
}

func (b *MutexMPMC) Enqueue(v int) bool {
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

func (b *MutexMPMC) Dequeue() (int, bool) {
	b.mu.Lock()

	if (b.tail - b.head) == 0 {
		b.mu.Unlock()
		return 0, false
	}

	pos := b.head & queueMask
	val := b.arr[pos]

	b.head++
	b.mu.Unlock()
	return val, true
}
