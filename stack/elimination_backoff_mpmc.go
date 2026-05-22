package stack

import (
	"runtime"
	"sync/atomic"
)

type eliminationKind int32

const (
	eliminationPush eliminationKind = iota
	eliminationPop

	eliminationArraySize = 64
	eliminationAttempts  = 2
	eliminationWaitSpins = 2048
)

// value is plain (not atomic). Synchronisation: writer publishes value
// before either (a) s.op CAS-install that hands the op to a reader, or
// (b) done.Store(true) that releases a waiting peer. The matching
// acquire (s.op.Load / done.Load) happens-before any read of value.
type eliminationOp[T any] struct {
	kind  eliminationKind
	value T
	done  atomic.Bool
}

type eliminationSlot[T any] struct {
	op atomic.Pointer[eliminationOp[T]]
}

type EliminationBackoffMPMC[T any] struct {
	head        atomic.Pointer[node[T]]
	ticket      atomic.Uint64
	elimination []eliminationSlot[T]
}

func NewEliminationBackoffMPMC[T any]() *EliminationBackoffMPMC[T] {
	return &EliminationBackoffMPMC[T]{
		elimination: make([]eliminationSlot[T], eliminationArraySize),
	}
}

func (e *EliminationBackoffMPMC[T]) Push(v T) {
	n := &node[T]{v: v}
	for {
		prev := e.head.Load()
		n.next = prev

		if e.head.CompareAndSwap(prev, n) {
			return
		}

		if e.tryEliminatePush(v) {
			return
		}
	}
}

func (e *EliminationBackoffMPMC[T]) Pop() (T, bool) {
	var zero T
	for {
		prev := e.head.Load()
		if prev == nil {
			if v, ok := e.tryEliminatePop(); ok {
				return v, true
			}
			if e.head.Load() == nil {
				return zero, false
			}
			continue
		}

		val, next := prev.v, prev.next
		if e.head.CompareAndSwap(prev, next) {
			return val, true
		}

		if v, ok := e.tryEliminatePop(); ok {
			return v, true
		}
	}
}

func (e *EliminationBackoffMPMC[T]) tryEliminatePush(v T) bool {
	for range eliminationAttempts {
		slot := &e.elimination[e.randomSlot()]
		if slot.visitPush(v) {
			return true
		}
	}
	return false
}

func (e *EliminationBackoffMPMC[T]) tryEliminatePop() (T, bool) {
	var zero T
	for range eliminationAttempts {
		slot := &e.elimination[e.randomSlot()]
		if v, ok := slot.visitPop(); ok {
			return v, true
		}
	}
	return zero, false
}

func (e *EliminationBackoffMPMC[T]) randomSlot() int {
	x := e.ticket.Add(0x9e3779b97f4a7c15)
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return int(x % uint64(len(e.elimination)))
}

func (s *eliminationSlot[T]) visitPush(v T) bool {
	mine := &eliminationOp[T]{kind: eliminationPush, value: v}

	for {
		curr := s.op.Load()
		switch {
		case curr == nil:
			if s.op.CompareAndSwap(nil, mine) {
				return s.waitForPushMatch(mine)
			}
		case curr.kind == eliminationPop:
			if s.op.CompareAndSwap(curr, nil) {
				curr.value = v
				curr.done.Store(true)
				return true
			}
		default:
			return false
		}
	}
}

func (s *eliminationSlot[T]) visitPop() (T, bool) {
	var zero T
	mine := &eliminationOp[T]{kind: eliminationPop}

	for {
		curr := s.op.Load()
		switch {
		case curr == nil:
			if s.op.CompareAndSwap(nil, mine) {
				return s.waitForPopMatch(mine)
			}
		case curr.kind == eliminationPush:
			if s.op.CompareAndSwap(curr, nil) {
				v := curr.value
				curr.done.Store(true)
				return v, true
			}
		default:
			return zero, false
		}
	}
}

func (s *eliminationSlot[T]) waitForPushMatch(op *eliminationOp[T]) bool {
	if waitForExchange(op) {
		return true
	}
	if s.op.CompareAndSwap(op, nil) {
		return false
	}
	waitForExchange(op)
	return true
}

func (s *eliminationSlot[T]) waitForPopMatch(op *eliminationOp[T]) (T, bool) {
	var zero T
	if waitForExchange(op) {
		return op.value, true
	}
	if s.op.CompareAndSwap(op, nil) {
		return zero, false
	}
	waitForExchange(op)
	return op.value, true
}

func waitForExchange[T any](op *eliminationOp[T]) bool {
	for range eliminationWaitSpins {
		if op.done.Load() {
			return true
		}
		runtime.Gosched()
	}
	return op.done.Load()
}
