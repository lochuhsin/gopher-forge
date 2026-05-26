package queue

import "sync/atomic"

type Queue[T any] interface {
	Dequeue() (T, bool)
	Enqueue(v T) bool
}

const (
	BoundedQueueSize = 1024
	queueMask        = BoundedQueueSize - 1
)

// slot padding assumes sizeof(T) ≤ 8 bytes; larger T breaks cache line alignment.
type slot[T any] struct {
	seq atomic.Uint64
	val T
	_   [112]byte
}

//TODO: SPSC queue + LMAX Disruptor
