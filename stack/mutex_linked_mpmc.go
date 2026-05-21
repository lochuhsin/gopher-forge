package stack

import "sync"

type MutexLinkedMPMC struct {
	head *node
	mu   sync.Mutex
}

func NewMutexLinkedMPMC() *MutexLinkedMPMC {
	return &MutexLinkedMPMC{}
}

func (b *MutexLinkedMPMC) Pop() (int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.head == nil {
		return 0, false
	}

	val := b.head.v
	b.head = b.head.next
	return val, true
}

func (b *MutexLinkedMPMC) Push(v int) {
	b.mu.Lock()
	defer b.mu.Unlock()

	n := &node{
		v: v,
	}

	if b.head == nil {
		b.head = n
		return
	}

	n.next = b.head
	b.head = n
}
