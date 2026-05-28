package syncx

import (
	"sync"
	"sync/atomic"
)

type Once struct {
	state atomic.Bool
	mu    sync.Mutex
}

func (o *Once) Do(f func()) {
	// fast path
	old := o.state.Load()
	if old {
		return
	}

	// slow path
	o.mu.Lock()
	defer o.mu.Unlock()
	if !o.state.Load() {
		defer o.state.Store(true)
		f()
	}
}

type OnceCell[T any] struct {
	once Once
	v    T
}

func (o *OnceCell[T]) Do(f func() T) T {
	o.once.Do(func() { o.v = f() })
	return o.v
}

type OnceCells[T any, K any] struct {
	once Once
	v    T
	v2   K
}

func (o *OnceCells[T, K]) Do(f func() (T, K)) (T, K) {
	o.once.Do(func() {
		o.v, o.v2 = f()
	})
	return o.v, o.v2
}
