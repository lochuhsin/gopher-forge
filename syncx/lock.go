package syncx

import (
	"sync/atomic"
)

type SpinLock struct {
	state atomic.Bool
}

func (s *SpinLock) Lock() {
	for {
		if s.state.CompareAndSwap(false, true) {
			return
		}
	}
}

func (s *SpinLock) Unlock() {
	s.state.Store(false)
}
