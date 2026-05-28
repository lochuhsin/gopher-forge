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

type OnceCellError[T any] struct {
	once Once
	v    T
	err  error
}

func (o *OnceCellError[T]) Do(f func() (T, error)) (T, error) {
	o.once.Do(func() {
		o.v, o.err = f()
	})
	return o.v, o.err
}
