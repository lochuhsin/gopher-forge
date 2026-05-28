package queue

import "sync/atomic"

type Sequence struct {
	_     [56]byte
	value atomic.Int64
	_     [56]byte
}

func NewSequence(initial int64) *Sequence {
	seq := &Sequence{}
	seq.value.Store(initial)
	return seq
}

func (s *Sequence) Get() int64 {
	return s.value.Load()
}

func (s *Sequence) Set(value int64) {
	s.value.Store(value)
}

func (s *Sequence) IncrementAndGet() int64 {
	return s.value.Add(1)
}

func (s *Sequence) AddAndGet(delta int64) int64 {
	return s.value.Add(delta)
}

func (s *Sequence) CompareAndSet(expected, next int64) bool {
	return s.value.CompareAndSwap(expected, next)
}

type WaitStrategy interface {
	WaitFor(seq int64, cursor *Sequence) int64
	SignalAllWhenBlocking()
}

type BlockingWaitStrategy struct{}

func (b *BlockingWaitStrategy) WaitFor(seq int64, cursor *Sequence) int64 {
	return 0
}

func (b *BlockingWaitStrategy) SignalAllWhenBlocking() {
}
