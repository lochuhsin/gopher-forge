package queue

import "sync/atomic"

type Queue interface {
	Dequeue() (int, bool)
	Enqueue(v int) bool
}

const (
	BoundedQueueSize = 128
	queueMask        = BoundedQueueSize - 1
)

type slot struct {
	seq atomic.Uint64
	val int
	_   [112]byte
}
