package syncx

const DefaultBuffSize = 1024

func UnorderedChannel[T any](in <-chan T) <-chan T {
	out := make(chan T)

	go func() {
		buf := make([]T, 0, DefaultBuffSize)

		for v := range in {
			select {
			case out <- v:
			default:
				buf = append(buf, v)
			}
		}

		for _, v := range buf {
			out <- v
		}
		close(out)
	}()

	return out
}

type Queue[T any] interface {
	Len() int
	Push(T)
	Peek() (T, bool)
	Pop() (T, bool)
}

type RingQueue[T any] struct {
	arr  []T
	head int
	tail int
	size int
	mask int
}

func NewRingQueue[T any]() *RingQueue[T] {
	return &RingQueue[T]{
		arr:  make([]T, DefaultBuffSize),
		mask: DefaultBuffSize - 1,
	}
}

func (q *RingQueue[T]) Len() int { return q.size }

func (q *RingQueue[T]) Push(v T) {
	if q.size == len(q.arr) {
		q.grow()
	}
	q.arr[q.tail] = v
	q.tail = (q.tail + 1) & q.mask
	q.size++
}

func (q *RingQueue[T]) Peek() (T, bool) {
	var zero T
	if q.size == 0 {
		return zero, false
	}
	return q.arr[q.head], true
}

func (q *RingQueue[T]) Pop() (T, bool) {
	var zero T
	if q.size == 0 {
		return zero, false
	}
	v := q.arr[q.head]
	q.arr[q.head] = zero
	q.head = (q.head + 1) & q.mask
	q.size--
	return v, true
}

func (q *RingQueue[T]) grow() {
	newCap := len(q.arr) * 2
	newData := make([]T, newCap)
	n := copy(newData, q.arr[q.head:])
	copy(newData[n:], q.arr[:q.tail])
	q.arr = newData
	q.head = 0
	q.tail = q.size
	q.mask = newCap - 1
}

func OrderedChannel[T any](in <-chan T) <-chan T {
	buff := NewRingQueue[T]()
	out := make(chan T)

	go func() {
		defer close(out)

	Outer:
		for {
			var (
				sendCh chan<- T
				first  T
			)
			if f, ok := buff.Peek(); ok {
				sendCh = out
				first = f
			}

			select {
			case v, ok := <-in:
				if !ok {
					break Outer
				}
				buff.Push(v)

			case sendCh <- first:
				buff.Pop()
			}
		}

		for {
			v, ok := buff.Pop()
			if !ok {
				break
			}
			out <- v
		}
	}()

	return out
}
