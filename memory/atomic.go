package memory

import "sync/atomic"

// AtomicCell is a single-location atomic load/store over atomic.Pointer[T].
// Substrate: sync/atomic. Linearization point: the atomic call itself.
// Progress: wait-free (single atomic op, no loop).
//
// It publishes the cell pointer only — not unrelated fields the stored
// value transitively reaches. For that, pair this with an acquire/release
// flag (see AcquireReleasePairing). Store snapshots T by value, allocating
// a fresh *T per call, so the cell is never an alias of the caller's
// variable.
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
