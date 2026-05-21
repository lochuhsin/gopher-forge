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

type eliminationOp struct {
	kind  eliminationKind
	value atomic.Int64
	done  atomic.Bool
}

type eliminationSlot struct {
	op atomic.Pointer[eliminationOp]
}

type EliminationBackoffMPMC struct {
	head        atomic.Pointer[node]
	ticket      atomic.Uint64
	elimination []eliminationSlot
}

func NewEliminationBackoffMPMC() *EliminationBackoffMPMC {
	return &EliminationBackoffMPMC{
		elimination: make([]eliminationSlot, eliminationArraySize),
	}
}

func (e *EliminationBackoffMPMC) Push(v int) {
	n := &node{v: v}
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

func (e *EliminationBackoffMPMC) Pop() (int, bool) {
	for {
		prev := e.head.Load()
		if prev == nil {
			if v, ok := e.tryEliminatePop(); ok {
				return v, true
			}
			if e.head.Load() == nil {
				return 0, false
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

func (e *EliminationBackoffMPMC) tryEliminatePush(v int) bool {
	for range eliminationAttempts {
		slot := &e.elimination[e.randomSlot()]
		if slot.visitPush(v) {
			return true
		}
	}
	return false
}

func (e *EliminationBackoffMPMC) tryEliminatePop() (int, bool) {
	for range eliminationAttempts {
		slot := &e.elimination[e.randomSlot()]
		if v, ok := slot.visitPop(); ok {
			return v, true
		}
	}
	return 0, false
}

func (e *EliminationBackoffMPMC) randomSlot() int {
	x := e.ticket.Add(0x9e3779b97f4a7c15)
	x ^= x >> 30
	x *= 0xbf58476d1ce4e5b9
	x ^= x >> 27
	x *= 0x94d049bb133111eb
	x ^= x >> 31
	return int(x % uint64(len(e.elimination)))
}

func (s *eliminationSlot) visitPush(v int) bool {
	mine := &eliminationOp{kind: eliminationPush}
	mine.value.Store(int64(v))

	for {
		curr := s.op.Load()
		switch {
		case curr == nil:
			if s.op.CompareAndSwap(nil, mine) {
				return s.waitForPushMatch(mine)
			}
		case curr.kind == eliminationPop:
			if s.op.CompareAndSwap(curr, nil) {
				curr.value.Store(int64(v))
				curr.done.Store(true)
				return true
			}
		default:
			return false
		}
	}
}

func (s *eliminationSlot) visitPop() (int, bool) {
	mine := &eliminationOp{kind: eliminationPop}

	for {
		curr := s.op.Load()
		switch {
		case curr == nil:
			if s.op.CompareAndSwap(nil, mine) {
				return s.waitForPopMatch(mine)
			}
		case curr.kind == eliminationPush:
			if s.op.CompareAndSwap(curr, nil) {
				v := int(curr.value.Load())
				curr.done.Store(true)
				return v, true
			}
		default:
			return 0, false
		}
	}
}

func (s *eliminationSlot) waitForPushMatch(op *eliminationOp) bool {
	if waitForExchange(op) {
		return true
	}
	if s.op.CompareAndSwap(op, nil) {
		return false
	}
	waitForExchange(op)
	return true
}

func (s *eliminationSlot) waitForPopMatch(op *eliminationOp) (int, bool) {
	if waitForExchange(op) {
		return int(op.value.Load()), true
	}
	if s.op.CompareAndSwap(op, nil) {
		return 0, false
	}
	waitForExchange(op)
	return int(op.value.Load()), true
}

func waitForExchange(op *eliminationOp) bool {
	for range eliminationWaitSpins {
		if op.done.Load() {
			return true
		}
		runtime.Gosched()
	}
	return op.done.Load()
}
