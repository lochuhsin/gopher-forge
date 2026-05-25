package syncx

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type testLocker interface {
	Lock()
	Unlock()
}

var lockImpls = []struct {
	name string
	new  func() testLocker
}{
	{"SpinLock", func() testLocker { return &SpinLock{} }},
	{"TicketLock", func() testLocker { return &TicketLock{} }},
	{"MCSLock", func() testLocker { return &MCSLock{} }},
	{"RWMutexLock", func() testLocker { return &RWMutexLock{} }},
}

func TestLocksBasicLockUnlock(t *testing.T) {
	for _, impl := range lockImpls {
		t.Run(impl.name, func(t *testing.T) {
			lock := impl.new()
			lock.Lock()
			lock.Unlock()
			lock.Lock()
			lock.Unlock()
		})
	}
}

func TestLocksBlockWaitersUntilUnlock(t *testing.T) {
	for _, impl := range lockImpls {
		t.Run(impl.name, func(t *testing.T) {
			lock := impl.new()
			lock.Lock()

			ready := make(chan struct{})
			acquired := make(chan struct{})
			go func() {
				close(ready)
				lock.Lock()
				close(acquired)
				lock.Unlock()
			}()

			<-ready
			select {
			case <-acquired:
				t.Fatal("waiter acquired lock before Unlock")
			case <-time.After(20 * time.Millisecond):
			}

			lock.Unlock()
			select {
			case <-acquired:
			case <-time.After(time.Second):
				t.Fatal("waiter did not acquire lock after Unlock")
			}
		})
	}
}

func TestLocksSerializeCriticalSections(t *testing.T) {
	for _, impl := range lockImpls {
		t.Run(impl.name, func(t *testing.T) {
			lock := impl.new()

			const (
				workers    = 8
				iterations = 32
			)

			var inCritical atomic.Int64
			var violations atomic.Int64
			start := make(chan struct{})
			done := make(chan struct{})
			var wg sync.WaitGroup

			for range workers {
				wg.Add(1)
				go func() {
					defer wg.Done()
					<-start
					for range iterations {
						lock.Lock()
						if got := inCritical.Add(1); got != 1 {
							violations.Add(1)
						}
						doLockTestWork()
						inCritical.Add(-1)
						lock.Unlock()
						runtime.Gosched()
					}
				}()
			}

			close(start)
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("workers did not finish")
			}

			if got := violations.Load(); got != 0 {
				t.Fatalf("critical section overlapped %d times", got)
			}
		})
	}
}

var lockTestSink atomic.Uint64

func doLockTestWork() {
	var x uint64
	for i := range 32 {
		x += uint64(i) + 0x9e3779b97f4a7c15
		x ^= x >> 7
	}
	lockTestSink.Store(x)
}

func TestRWMutexLockReadersShareAndWritersExclude(t *testing.T) {
	var lock RWMutexLock
	lock.RLock()

	readerAcquired := make(chan struct{})
	go func() {
		lock.RLock()
		close(readerAcquired)
		lock.RUnlock()
	}()

	select {
	case <-readerAcquired:
	case <-time.After(time.Second):
		t.Fatal("second reader did not acquire while first reader held RLock")
	}

	writerAcquired := make(chan struct{})
	go func() {
		lock.Lock()
		close(writerAcquired)
		lock.Unlock()
	}()

	select {
	case <-writerAcquired:
		t.Fatal("writer acquired while reader held RLock")
	case <-time.After(20 * time.Millisecond):
	}

	lock.RUnlock()
	select {
	case <-writerAcquired:
	case <-time.After(time.Second):
		t.Fatal("writer did not acquire after reader released RLock")
	}
}
