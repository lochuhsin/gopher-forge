package syncx

import (
	"sync/atomic"
)

// Channel Broadcast
type DemoFuture[T any] struct {
	v      T
	err    error
	state  atomic.Bool
	notify chan struct{}
}

func NewDemoFuture[T any](fn func() (T, error)) *DemoFuture[T] {
	future := &DemoFuture[T]{notify: make(chan struct{})}

	go func() {
		future.v, future.err = fn()
		future.state.Store(true)
		close(future.notify)
	}()

	return future
}

func (f *DemoFuture[T]) Get() (T, error) {
	if !f.state.Load() {
		<-f.notify
	}
	return f.v, f.err
}
