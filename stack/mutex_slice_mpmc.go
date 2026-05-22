package stack

import "sync"

type MutexSliceMPMC[T any] struct {
	arr []T
	mu  sync.Mutex
}

func NewMutexSliceMPMC[T any]() *MutexSliceMPMC[T] {
	return &MutexSliceMPMC[T]{
		arr: []T{},
	}
}

func (b *MutexSliceMPMC[T]) Pop() (T, bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	var zero T
	if len(b.arr) == 0 {
		return zero, false
	}

	last := len(b.arr) - 1
	val := b.arr[last]
	b.arr[last] = zero
	b.arr = b.arr[:last]
	return val, true
}

func (b *MutexSliceMPMC[T]) Push(v T) {
	b.mu.Lock()
	b.arr = append(b.arr, v)
	b.mu.Unlock()
}
