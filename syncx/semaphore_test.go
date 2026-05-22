package syncx

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var semaphoreImpls = []struct {
	name string
	new  func(int) Semaphore
}{
	{"Mutex", func(n int) Semaphore { return NewMutexSemaphore(n) }},
	{"Channel", func(n int) Semaphore { return NewChannelSemaphore(n) }},
}

func TestSemaphoreBasicAcquireRelease(t *testing.T) {
	for _, impl := range semaphoreImpls {
		t.Run(impl.name, func(t *testing.T) {
			sem := impl.new(2)
			sem.Acquire()
			sem.Acquire()
			sem.Release()
			sem.Acquire()
			sem.Release()
			sem.Release()
		})
	}
}

//

func TestSemaphoreAcquireBlocksAtMax(t *testing.T) {
	for _, impl := range semaphoreImpls {
		t.Run(impl.name, func(t *testing.T) {
			sem := impl.new(1)
			sem.Acquire()

			blocked := make(chan struct{})
			go func() {
				sem.Acquire()
				close(blocked)
			}()

			select {
			case <-blocked:
				t.Fatal("Acquire did not block when at max permits")
			case <-time.After(100 * time.Millisecond):

			}

			sem.Release()

			select {
			case <-blocked:

			case <-time.After(time.Second):
				t.Fatal("Acquire did not unblock after Release")
			}
			sem.Release()
		})
	}
}

func TestMutexSemaphoreReleaseWithoutAcquirePanics(t *testing.T) {
	sem := NewMutexSemaphore(1)
	defer func() {
		if r := recover(); r == nil {
			t.Error("Release without prior Acquire should panic")
		}
	}()
	sem.Release()
}

//

func TestSemaphoreLimitsConcurrency(t *testing.T) {
	for _, impl := range semaphoreImpls {
		t.Run(impl.name, func(t *testing.T) {
			const (
				maxConcurrent = 5
				workers       = 50
			)
			sem := impl.new(maxConcurrent)

			var (
				current     atomic.Int64
				maxObserved atomic.Int64
				wg          sync.WaitGroup
			)
			for range workers {
				wg.Go(func() {
					sem.Acquire()
					defer sem.Release()

					cur := current.Add(1)
					for {
						prev := maxObserved.Load()
						if cur <= prev {
							break
						}
						if maxObserved.CompareAndSwap(prev, cur) {
							break
						}
					}
					time.Sleep(5 * time.Millisecond)
					current.Add(-1)
				})
			}
			wg.Wait()

			if got := maxObserved.Load(); got > maxConcurrent {
				t.Errorf("max concurrent observed = %d, want <= %d", got, maxConcurrent)
			}
		})
	}
}

func TestSemaphoreCrossGoroutineReleaseAcquire(t *testing.T) {
	for _, impl := range semaphoreImpls {
		t.Run(impl.name, func(t *testing.T) {
			sem := impl.new(1)
			sem.Acquire()

			releaserDone := make(chan struct{})
			go func() {
				defer close(releaserDone)
				time.Sleep(50 * time.Millisecond)
				sem.Release()
			}()

			acquired := make(chan struct{})
			go func() {
				sem.Acquire()
				close(acquired)
			}()

			select {
			case <-acquired:
			case <-time.After(time.Second):
				t.Fatal("Acquire did not unblock from cross-goroutine Release")
			}
			<-releaserDone
			sem.Release()
		})
	}
}

func TestSemaphoreChainedRelease(t *testing.T) {
	for _, impl := range semaphoreImpls {
		t.Run(impl.name, func(t *testing.T) {
			sem := impl.new(1)
			sem.Acquire()

			const waiters = 5
			var wg sync.WaitGroup
			finished := make(chan int, waiters)
			for i := range waiters {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()
					sem.Acquire()
					finished <- id
					sem.Release()
				}(i)
			}

			time.Sleep(100 * time.Millisecond)
			sem.Release()

			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Fatalf("chained release did not complete; only %d/%d finished", len(finished), waiters)
			}
			if got := len(finished); got != waiters {
				t.Errorf("got %d completed waiters, want %d", got, waiters)
			}
		})
	}
}

func TestSemaphoreBinary(t *testing.T) {
	for _, impl := range semaphoreImpls {
		t.Run(impl.name, func(t *testing.T) {
			sem := impl.new(1)

			var counter int
			const iterations = 100
			var wg sync.WaitGroup
			for range iterations {
				wg.Go(func() {
					sem.Acquire()
					defer sem.Release()

					counter++
				})
			}
			wg.Wait()

			if counter != iterations {
				t.Errorf("counter = %d, want %d (lost updates → semaphore failed to serialize)", counter, iterations)
			}
		})
	}
}

func TestSemaphoreStressRace(t *testing.T) {
	if testing.Short() {
		t.Skip("stress test skipped under -short")
	}
	for _, impl := range semaphoreImpls {
		t.Run(impl.name, func(t *testing.T) {
			const (
				permits = 10
				ops     = 50000
			)
			sem := impl.new(permits)

			var wg sync.WaitGroup
			for range ops {
				wg.Go(func() {
					sem.Acquire()
					sem.Release()
				})
			}

			done := make(chan struct{})
			go func() {
				wg.Wait()
				close(done)
			}()

			select {
			case <-done:
			case <-time.After(30 * time.Second):
				t.Fatal("stress test deadlocked")
			}
		})
	}
}
