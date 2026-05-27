package queue

import "sync/atomic"

type SPSCQueue[T any] struct {
	arr [BoundedQueueSize]T

	_    [120]byte // 128 - 8
	head atomic.Uint64

	_    [120]byte // 128 - 8
	tail atomic.Uint64
}

func NewSPSCQueue[T any]() *SPSCQueue[T] {
	return &SPSCQueue[T]{}
}

func (s *SPSCQueue[T]) Enqueue(v T) bool {
	t := s.tail.Load()
	diff := t - s.head.Load()

	if diff >= BoundedQueueSize {
		return false
	}

	s.arr[t&queueMask] = v
	s.tail.Store(t + 1)

	return true
}

func (s *SPSCQueue[T]) Dequeue() (T, bool) {
	h := s.head.Load()
	diff := s.tail.Load() - h
	if diff == 0 {
		var zero T
		return zero, false
	}

	v := s.arr[h&queueMask]
	s.head.Store(h + 1)
	return v, true
}

type CachedSPSCQueue[T any] struct {
	arr [BoundedQueueSize]T

	// producerTail and consumerHead are owner-local cursors. The opposite
	// goroutine only observes progress through the published atomics below.
	_            [112]byte // 128 - 16
	producerTail uint64
	cachedHead   uint64

	_            [112]byte // 128 - 16
	consumerHead uint64
	cachedTail   uint64

	_             [120]byte // 128 - 8
	publishedHead atomic.Uint64

	_             [120]byte // 128 - 8
	publishedTail atomic.Uint64
}

func NewCachedSPSCQueue[T any]() *CachedSPSCQueue[T] {
	return &CachedSPSCQueue[T]{}
}

func (s *CachedSPSCQueue[T]) Enqueue(v T) bool {
	t := s.producerTail
	if t-s.cachedHead >= BoundedQueueSize {
		s.cachedHead = s.publishedHead.Load()
		if t-s.cachedHead >= BoundedQueueSize {
			return false
		}
	}
	s.arr[t&queueMask] = v
	s.producerTail = t + 1
	s.publishedTail.Store(t + 1)
	return true
}

func (s *CachedSPSCQueue[T]) Dequeue() (T, bool) {
	h := s.consumerHead

	if s.cachedTail-h == 0 {
		s.cachedTail = s.publishedTail.Load()
		if s.cachedTail-h == 0 {
			var zero T
			return zero, false
		}
	}

	v := s.arr[h&queueMask]
	s.consumerHead = h + 1
	s.publishedHead.Store(h + 1)
	return v, true
}
