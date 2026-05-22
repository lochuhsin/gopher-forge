package stack

import "sync/atomic"

type LockFreeMPMC[T any] struct {
	head atomic.Pointer[node[T]]
}

func NewLockFreeMPMC[T any]() *LockFreeMPMC[T] {
	return &LockFreeMPMC[T]{}
}

func (l *LockFreeMPMC[T]) Push(v T) {
	n := &node[T]{v: v}
	for {
		prev := l.head.Load()
		n.next = prev

		if l.head.CompareAndSwap(prev, n) {
			return
		}
	}
}

func (l *LockFreeMPMC[T]) Pop() (T, bool) {
	var zero T
	for {
		prev := l.head.Load()
		if prev == nil {
			return zero, false
		}

		val, next := prev.v, prev.next
		if l.head.CompareAndSwap(prev, next) {
			return val, true
		}
	}
}
