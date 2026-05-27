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

func TestSeqLockInitialStateIsEvenAndValidates(t *testing.T) {
	var lk SeqLock
	s := lk.ReadBegin()
	if s != 0 {
		t.Fatalf("initial seq = %d, want 0", s)
	}
	if !lk.ReadValidate(s) {
		t.Fatal("ReadValidate(0) on fresh SeqLock = false, want true")
	}
}

func TestSeqLockWriteLockMakesSeqOdd(t *testing.T) {
	var lk SeqLock
	lk.WriteLock()
	s := lk.ReadBegin()
	if s&1 == 0 {
		t.Fatalf("seq after WriteLock = %d, want odd", s)
	}
	if lk.ReadValidate(s) {
		t.Fatal("ReadValidate of odd start returned true; must reject mid-write reads")
	}
	lk.WriteUnlock()
}

func TestSeqLockWriteUnlockRestoresEven(t *testing.T) {
	var lk SeqLock
	lk.WriteLock()
	lk.WriteUnlock()
	s := lk.ReadBegin()
	if s&1 != 0 {
		t.Fatalf("seq after WriteLock+WriteUnlock = %d, want even", s)
	}
	if !lk.ReadValidate(s) {
		t.Fatal("ReadValidate(even, unchanged) = false, want true")
	}
}

func TestSeqLockDetectsConcurrentWriteStart(t *testing.T) {
	var lk SeqLock
	start := lk.ReadBegin()
	lk.WriteLock()
	if lk.ReadValidate(start) {
		t.Fatal("validate returned true while writer holds the lock")
	}
	lk.WriteUnlock()
}

func TestSeqLockDetectsCompletedWriteBetween(t *testing.T) {
	var lk SeqLock
	start := lk.ReadBegin()
	lk.WriteLock()
	lk.WriteUnlock()
	if lk.ReadValidate(start) {
		t.Fatal("validate returned true after an intervening write cycle")
	}
}

func TestSeqLockSequenceProgressesByTwoPerCycle(t *testing.T) {
	var lk SeqLock
	const cycles = 100
	for range cycles {
		lk.WriteLock()
		lk.WriteUnlock()
	}
	if got := lk.ReadBegin(); got != 2*cycles {
		t.Fatalf("seq after %d cycles = %d, want %d", cycles, got, 2*cycles)
	}
}

// TestSeqLockReaderNeverObservesTornWrite is the canonical SeqLock concurrency
// test. Invariant: protected fields a and b must always be equal. The writer
// updates both between WriteLock and WriteUnlock; a reader that does NOT use
// seq validation can observe a torn pair (a != b). A correct SeqLock reader
// must never accept a torn snapshot after ReadValidate returns true.
//
// Protected fields are atomic.Uint64 so the test runs cleanly under -race.
// The SeqLock still adds real value: per-field atomicity does not guarantee
// multi-field consistency.
func TestSeqLockReaderNeverObservesTornWrite(t *testing.T) {
	var lk SeqLock
	var a, b atomic.Uint64

	const readers = 8
	duration := 200 * time.Millisecond
	if testing.Short() {
		duration = 50 * time.Millisecond
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			lk.WriteLock()
			v := a.Load() + 1
			a.Store(v)
			// Widen the tear window so a broken validate is more likely caught.
			runtime.Gosched()
			b.Store(v)
			lk.WriteUnlock()
		}
	})

	var tears atomic.Int64
	for range readers {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				var av, bv uint64
				for {
					s := lk.ReadBegin()
					av = a.Load()
					bv = b.Load()
					if lk.ReadValidate(s) {
						break
					}
				}
				if av != bv {
					tears.Add(1)
				}
			}
		})
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()

	if got := tears.Load(); got != 0 {
		t.Fatalf("readers observed %d torn snapshots; SeqLock failed to provide consistency", got)
	}
}

func TestSeqLockReadersMakeProgressUnderContention(t *testing.T) {
	var lk SeqLock
	var data atomic.Uint64

	duration := 100 * time.Millisecond
	if testing.Short() {
		duration = 25 * time.Millisecond
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			lk.WriteLock()
			data.Add(1)
			lk.WriteUnlock()
			time.Sleep(time.Microsecond)
		}
	})

	var successes atomic.Int64
	const readers = 4
	for range readers {
		wg.Go(func() {
			for {
				select {
				case <-stop:
					return
				default:
				}
				s := lk.ReadBegin()
				_ = data.Load()
				if lk.ReadValidate(s) {
					successes.Add(1)
				}
			}
		})
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()

	if got := successes.Load(); got == 0 {
		t.Fatal("no reader ever validated successfully; readers may be livelocking")
	}
}

func TestSeqLockSnapshotMonotonic(t *testing.T) {
	var lk SeqLock
	var data atomic.Uint64

	duration := 100 * time.Millisecond
	if testing.Short() {
		duration = 25 * time.Millisecond
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup

	wg.Go(func() {
		for {
			select {
			case <-stop:
				return
			default:
			}
			lk.WriteLock()
			data.Add(1)
			lk.WriteUnlock()
		}
	})

	var regressions atomic.Int64
	const readers = 4
	for range readers {
		wg.Go(func() {
			var last uint64
			for {
				select {
				case <-stop:
					return
				default:
				}
				var v uint64
				for {
					s := lk.ReadBegin()
					v = data.Load()
					if lk.ReadValidate(s) {
						break
					}
				}
				if v < last {
					regressions.Add(1)
				}
				last = v
			}
		})
	}

	time.Sleep(duration)
	close(stop)
	wg.Wait()

	if got := regressions.Load(); got != 0 {
		t.Fatalf("validated snapshot went backwards %d times", got)
	}
}

func TestSeqLockNoFailuresWithoutWrites(t *testing.T) {
	var lk SeqLock

	const readers = 8
	const iterations = 10_000

	var failures atomic.Int64
	var wg sync.WaitGroup
	for range readers {
		wg.Go(func() {
			for range iterations {
				s := lk.ReadBegin()
				if !lk.ReadValidate(s) {
					failures.Add(1)
				}
			}
		})
	}
	wg.Wait()

	if got := failures.Load(); got != 0 {
		t.Fatalf("ReadValidate failed %d times with no concurrent writers", got)
	}
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
