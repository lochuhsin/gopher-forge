package stack

import "sync"

type MutexSliceMPMC struct {
	arr []int
	mu  sync.Mutex
}

func NewMutexSliceMPMC() *MutexSliceMPMC {
	return &MutexSliceMPMC{
		arr: []int{},
	}
}

func (b *MutexSliceMPMC) Pop() (int, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if len(b.arr) == 0 {
		return 0, false
	}

	val := b.arr[len(b.arr)-1]
	b.arr = b.arr[:len(b.arr)-1]
	return val, true
}

func (b *MutexSliceMPMC) Push(v int) {
	b.mu.Lock()
	b.arr = append(b.arr, v)
	b.mu.Unlock()
}
