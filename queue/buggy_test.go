package queue

import "sync/atomic"

// Deliberately incorrect Michael-Scott queues used to verify the
// conservation tests actually detect races.

type queueNode struct {
	v    int
	next atomic.Pointer[queueNode]
}

// buggyEnqueueQueue: Enqueue stores into tail.next without CAS, so two
// concurrent Enqueuers can clobber each other.
type buggyEnqueueQueue struct {
	head atomic.Pointer[queueNode]
	tail atomic.Pointer[queueNode]
}

func newBuggyEnqueueQueue() *buggyEnqueueQueue {
	q := &buggyEnqueueQueue{}
	dummy := &queueNode{}
	q.head.Store(dummy)
	q.tail.Store(dummy)
	return q
}

func (q *buggyEnqueueQueue) Enqueue(v int) bool {
	n := &queueNode{v: v}
	tail := q.tail.Load()
	tail.next.Store(n) // BUG: no CAS
	q.tail.Store(n)
	return true
}

func (q *buggyEnqueueQueue) Dequeue() (int, bool) {
	for {
		head := q.head.Load()
		next := head.next.Load()
		if next == nil {
			return 0, false
		}
		if q.head.CompareAndSwap(head, next) {
			return next.v, true
		}
	}
}

// buggyDequeueQueue: Dequeue advances head with Store instead of CAS, so
// two concurrent Dequeuers can return the same value.
type buggyDequeueQueue struct {
	head atomic.Pointer[queueNode]
	tail atomic.Pointer[queueNode]
}

func newBuggyDequeueQueue() *buggyDequeueQueue {
	q := &buggyDequeueQueue{}
	dummy := &queueNode{}
	q.head.Store(dummy)
	q.tail.Store(dummy)
	return q
}

func (q *buggyDequeueQueue) Enqueue(v int) bool {
	n := &queueNode{v: v}
	for {
		tail := q.tail.Load()
		if tail.next.CompareAndSwap(nil, n) {
			q.tail.CompareAndSwap(tail, n)
			return true
		}
		if next := tail.next.Load(); next != nil {
			q.tail.CompareAndSwap(tail, next)
		}
	}
}

func (q *buggyDequeueQueue) Dequeue() (int, bool) {
	head := q.head.Load()
	next := head.next.Load()
	if next == nil {
		return 0, false
	}
	q.head.Store(next) // BUG: no CAS
	return next.v, true
}

var buggyQueueFactories = map[string]func() Queue{
	"BuggyEnqueue": func() Queue { return newBuggyEnqueueQueue() },
	"BuggyDequeue": func() Queue { return newBuggyDequeueQueue() },
}
