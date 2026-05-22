package syncx

import (
	"forge/queue"
	"runtime"
	"sync"
	"sync/atomic"
)

// Semaphore controls concurrent access by issuing at most N permits.
//
//   - Acquire blocks until a permit is available.
//   - Release returns immediately and wakes one waiter (if any).
//   - Calling Release without a prior Acquire panics.
//   - Permits are interchangeable across goroutines (one goroutine may
//     Acquire and another may Release).
type Semaphore interface {
	Acquire()
	Release()
}

type ChannelSemaphore struct {
	sem chan struct{}
}

func NewChannelSemaphore(init int) *ChannelSemaphore {
	return &ChannelSemaphore{
		sem: make(chan struct{}, init),
	}
}

func (c *ChannelSemaphore) Acquire() {
	c.sem <- struct{}{}
}

func (c *ChannelSemaphore) Release() {
	<-c.sem
}

type MutexSemaphore struct {
	mu      sync.Mutex
	waiters []*sync.Mutex // memory leak, probably use ring buffer
	count   int
	max     int
}

func NewMutexSemaphore(init int) *MutexSemaphore {
	return &MutexSemaphore{max: init}
}

func (m *MutexSemaphore) Acquire() {
	m.mu.Lock()

	if m.count < m.max {
		m.count++
		m.mu.Unlock()
		return
	}
	gate := &sync.Mutex{}
	gate.Lock()

	m.waiters = append(m.waiters, gate)

	m.mu.Unlock()
	gate.Lock()
}

func (m *MutexSemaphore) Release() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.waiters) > 0 {
		gate := m.waiters[0]
		m.waiters[0] = nil // gc collect
		m.waiters = m.waiters[1:]
		gate.Unlock()
	}
	m.count--
}

type CondSemaphore struct {
	cond  *sync.Cond
	mu    *sync.Mutex
	count int
	max   int
}

func NewCondSemaphore(init int) *CondSemaphore {
	mu := &sync.Mutex{}
	return &CondSemaphore{
		mu:   mu,
		max:  init,
		cond: sync.NewCond(sync.Locker(mu)),
	}
}

func (c *CondSemaphore) Acquire() {
	c.mu.Lock()

	for c.count >= c.max {
		c.cond.Wait()
	}
	c.count++

	c.mu.Unlock()
}

func (c *CondSemaphore) Release() {
	c.mu.Lock()

	c.count--
	c.cond.Signal()

	c.mu.Unlock()
}

type LockfreeSemaphore struct {
	permits *atomic.Int64
	waiters queue.Queue[*atomic.Bool]
}

func NewLockfreeSemaphore(init int) *LockfreeSemaphore {
	p := atomic.Int64{}
	p.Store(int64(init))

	return &LockfreeSemaphore{
		permits: &p,
		waiters: queue.NewLockFreeMPMC[*atomic.Bool](),
	}
}

func (l *LockfreeSemaphore) Acquire() {
	var val int64

	for {
		val = l.permits.Load()
		if l.permits.CompareAndSwap(val, val-1) {
			if val <= 0 {
				gate := &atomic.Bool{}
				l.waiters.Enqueue(gate)

				// park, cpu burn
				for !gate.Load() {
					runtime.Gosched()
				}
			}
			break
		}
	}
}

func (l *LockfreeSemaphore) Release() {
	var val int64

	for {
		val = l.permits.Load()
		if l.permits.CompareAndSwap(val, val+1) {

			// slow path
			if gate, ok := l.waiters.Dequeue(); ok {
				gate.Store(true)
			}
			break
		}
	}
}
