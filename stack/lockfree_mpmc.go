package stack

import "sync/atomic"

type LockFreeMPMC struct {
	head atomic.Pointer[node]
}

func NewLockFreeMPMC() *LockFreeMPMC {
	return &LockFreeMPMC{}
}

func (l *LockFreeMPMC) Push(v int) {
	n := &node{v: v}
	for {
		prev := l.head.Load()
		n.next = prev

		if l.head.CompareAndSwap(prev, n) {
			return
		}
	}
}

func (l *LockFreeMPMC) Pop() (int, bool) {
	for {
		prev := l.head.Load()
		if prev == nil {
			return 0, false
		}

		val, next := prev.v, prev.next
		if l.head.CompareAndSwap(prev, next) {
			return val, true
		}
	}
}
