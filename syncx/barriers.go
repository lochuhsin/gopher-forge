package syncx

import (
	"sync/atomic"
)

// Centralized counting barrier. One shot.
type CountingBarrier struct {
	seats *atomic.Int32
}

func NewCountingBarrier(n int32) *CountingBarrier {
	if n <= 0 {
		panic("barrier size must be positive")
	}
	seats := atomic.Int32{}
	seats.Store(n)
	return &CountingBarrier{
		seats: &seats,
	}
}

func (b *CountingBarrier) Wait() {
	b.seats.Add(-1)
	for b.seats.Load() > 0 {
		// spin
	}
}

type SenseReversingBarrier struct {
	release *atomic.Uint32
	seats   *atomic.Int32
	count   int32
}

func NewSenseReversingBarrier(n int32) *SenseReversingBarrier {
	if n <= 0 {
		panic("barrier size must be positive")
	}
	arrival := atomic.Int32{}
	arrival.Store(n)
	return &SenseReversingBarrier{
		release: &atomic.Uint32{},
		seats:   &arrival,
		count:   n,
	}
}

/*
The key point is that the path of the last goroutine is different than
normal one.

「state preparation → publication → observation」這三件事的時間順序不能亂。
Publisher 必須在準備好 state 之後才發 → Observer 必須在 publish 之前就開始觀察。
*/
func (s *SenseReversingBarrier) Wait() {
	release := s.release.Load()

	if s.seats.Add(-1) == 0 {
		s.seats.Store(s.count)
		s.release.Add(1) //release-after-prepare ordering
		return
	}

	for s.release.Load() == release {
		// Spin
	}
}
