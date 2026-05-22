package stack

import "sync/atomic"

// Deliberately incorrect stacks used to verify the concurrent tests
// actually detect races.

// buggyDupPopStack: Pop reads head with Load+Store instead of CAS, so
// two concurrent Pops can return the same value.
type buggyDupPopStack struct {
	head atomic.Pointer[node[int]]
}

func newBuggyDupPopStack() buggyDupPopStack { return buggyDupPopStack{} }

func (s *buggyDupPopStack) Push(v int) {
	n := &node[int]{v: v}
	for {
		prev := s.head.Load()
		n.next = prev
		if s.head.CompareAndSwap(prev, n) {
			return
		}
	}
}

func (s *buggyDupPopStack) Pop() (int, bool) {
	prev := s.head.Load()
	if prev == nil {
		return 0, false
	}
	s.head.Store(prev.next) // BUG: not atomic with Load
	return prev.v, true
}

// buggyLostPushStack: Push uses Load+Store instead of a CAS loop, so a
// concurrent Push from another goroutine can be overwritten.
type buggyLostPushStack struct {
	head atomic.Pointer[node[int]]
}

func newBuggyLostPushStack() buggyLostPushStack { return buggyLostPushStack{} }

func (s *buggyLostPushStack) Push(v int) {
	n := &node[int]{v: v}
	n.next = s.head.Load()
	s.head.Store(n) // BUG: missing CAS loop
}

func (s *buggyLostPushStack) Pop() (int, bool) {
	for {
		prev := s.head.Load()
		if prev == nil {
			return 0, false
		}
		if s.head.CompareAndSwap(prev, prev.next) {
			return prev.v, true
		}
	}
}

var buggyStackFactories = map[string]func() Stack[int]{
	"BuggyDupPop":   func() Stack[int] { s := newBuggyDupPopStack(); return &s },
	"BuggyLostPush": func() Stack[int] { s := newBuggyLostPushStack(); return &s },
}
