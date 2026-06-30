package patterns

import (
	"sync"
	"time"
)

type Broadcast[T any] struct {
	sliceMu sync.Mutex

	topics map[string][]chan<- T

	doneCh   chan struct{}
	doneOnce sync.Once
}

func NewBroadcast[T any]() *Broadcast[T] {
	return &Broadcast[T]{
		topics: make(map[string][]chan<- T),
		doneCh: make(chan struct{}),
	}
}

func (b *Broadcast[T]) Register(topic string) chan T {
	b.sliceMu.Lock()
	defer b.sliceMu.Unlock()

	ch := make(chan T)
	b.topics[topic] = append(b.topics[topic], ch)
	return ch
}

func (b *Broadcast[T]) Emit(topic string, value T) {
	select {
	case <-b.doneCh:
		return
	default:
	}

	b.sliceMu.Lock()

	chs := b.topics[topic]
	chCopy := make([]chan<- T, len(chs))

	copy(chCopy, chs)
	b.sliceMu.Unlock()

	// no policy based, slow subscriber

	after := time.After(time.Millisecond * 5)
	for _, ch := range chCopy {
		select {
		case <-b.doneCh:
			return
		case ch <- value:
		case <-after:
			go func() {
				select {
				case ch <- value:
				case <-b.doneCh:
					return
				}
			}()
			after = time.After(time.Millisecond)
		}
	}
}

func (b *Broadcast[T]) Done() <-chan struct{} {
	return b.doneCh
}

func (b *Broadcast[T]) Close() {
	b.doneOnce.Do(func() {
		close(b.doneCh)
	})
}
