package syncx

import (
	"context"
	"sync"

	"github.com/hashicorp/go-uuid"
)

const (
	PENDING int32 = iota
	COMPLETE
)

type runnable interface {
	Run()
	Signature() string
}

type node struct {
	next *node
	prev *node
	t    runnable
}

type orderedMap struct {
	head     *node
	tail     *node
	size     int
	taskMap  map[string]*node
	mu       sync.Mutex
	notEmpty *sync.Cond
	closed   bool
}

func newOrderedMap() *orderedMap {
	head, tail := &node{}, &node{}
	head.next = tail
	tail.prev = head
	o := &orderedMap{
		taskMap: make(map[string]*node),
		head:    head,
		tail:    tail,
	}
	o.notEmpty = sync.NewCond(&o.mu)
	return o
}

func (o *orderedMap) put(t runnable) {
	o.mu.Lock()
	defer o.mu.Unlock()

	n := &node{
		t: t,
	}
	o.taskMap[t.Signature()] = n

	o.tail.prev.next = n
	n.prev = o.tail.prev

	o.tail.prev = n
	n.next = o.tail

	o.size++
	o.notEmpty.Signal()
}

func (o *orderedMap) pollWait() (runnable, bool) {
	o.mu.Lock()
	defer o.mu.Unlock()

	for o.size == 0 && !o.closed {
		o.notEmpty.Wait()
	}

	n := o.head.next
	if n == o.tail {
		return nil, false // closed and drained
	}
	o.head.next = n.next
	o.head.next.prev = o.head
	delete(o.taskMap, n.t.Signature())
	o.size--

	return n.t, true
}

func (o *orderedMap) close() {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.closed = true
	o.notEmpty.Broadcast()
}

func (o *orderedMap) delete(key string) {
	o.mu.Lock()
	defer o.mu.Unlock()

	n, ok := o.taskMap[key]
	if !ok {
		return
	}
	delete(o.taskMap, key)
	n.prev.next = n.next
	n.next.prev = n.prev
	o.size--
}

var (
	workerPool *WorkerPool
	wpOnce     sync.Once
)

type WorkerPool struct {
	ctx     context.Context
	wpMu    sync.Mutex
	queue   *orderedMap
	taskMap map[string]runnable
}

func (w *WorkerPool) submit(r runnable) {
	w.wpMu.Lock()
	defer w.wpMu.Unlock()

	w.taskMap[r.Signature()] = r
	w.queue.put(r)
}

func (w *WorkerPool) get(signature string) (runnable, bool) {
	w.wpMu.Lock()
	defer w.wpMu.Unlock()

	t, ok := w.taskMap[signature]
	return t, ok
}

func newWorkerPool(ctx context.Context) *WorkerPool {
	wp := &WorkerPool{
		ctx:     ctx,
		queue:   newOrderedMap(),
		taskMap: make(map[string]runnable),
	}

	return wp
}

func (w *WorkerPool) start(workers int) {
	// When the context is cancelled, close the queue so parked workers wake and exit.
	go func() {
		<-w.ctx.Done()
		w.queue.close()
	}()

	for range workers {
		go func() {
			for {
				task, ok := w.queue.pollWait()
				if !ok {
					return // queue closed and drained
				}
				task.Run()
			}
		}()
	}
}

func NewWorkerPool(ctx context.Context, workers int) {
	wpOnce.Do(func() {
		workerPool = newWorkerPool(ctx)
		workerPool.start(workers)
	})
}

func getWorkerPool() *WorkerPool {
	if workerPool == nil {
		panic("worker pool is not initialized")
	}
	return workerPool
}

type task[T any] struct {
	fn        func() T
	v         T
	err       error
	signature string
	doneCh    chan struct{}
}

func (t *task[T]) Run() {
	t.v = t.fn()
	close(t.doneCh)
}

func (t *task[T]) Signature() string {
	return t.signature
}

type Promise[T any] struct {
	signature string
}

func NewPromise[T any]() (p *Promise[T], f *Future[T]) {
	if workerPool == nil {
		panic("worker pool is not initialized")
	}
	id, _ := uuid.GenerateUUID()
	return &Promise[T]{signature: id}, &Future[T]{signature: id}
}

func (p *Promise[T]) Resolve(fn func() T) {
	t := task[T]{
		fn:        fn,
		signature: p.signature,
		doneCh:    make(chan struct{}),
	}
	getWorkerPool().submit(&t)
}

type Future[T any] struct {
	signature string
}

func (f *Future[T]) Poll() (T, bool) {
	r, ok := getWorkerPool().get(f.signature)
	if !ok {
		panic("cannot find task")
	}

	t, _ := r.(*task[T])
	select {
	case <-t.doneCh:
		return t.v, true
	default:
		var zero T
		return zero, false
	}
}

func (f *Future[T]) Get() T {
	r, ok := getWorkerPool().get(f.signature)
	if !ok {
		panic("cannot find task")
	}

	t, _ := r.(*task[T])
	select {
	case <-t.doneCh:
		return t.v
	case <-workerPool.ctx.Done():
		panic("Worker pool canceled")
	}
}
