package memory

import "sync/atomic"

type AtomicCell[T any] struct {
	obj atomic.Pointer[T]
}

func (c *AtomicCell[T]) Store(v T) {
	c.obj.Store(&v)
}

func (c *AtomicCell[T]) Load() T {
	if r := c.obj.Load(); r != nil {
		return *r
	}

	return *new(T)
}
