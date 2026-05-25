package syncx

import (
	"fmt"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
)

var lockFactories = map[string]func() testLocker{
	"SpinLock":    func() testLocker { return &SpinLock{} },
	"TicketLock":  func() testLocker { return &TicketLock{} },
	"MCSLock":     func() testLocker { return &MCSLock{} },
	"RWMutexLock": func() testLocker { return &RWMutexLock{} },
}

// Note: every measured op samples time.Now() to record latency, adding
// measurement overhead. All implementations pay this equally, so the
// relative comparison still holds.

type lockMeasureMode int

const (
	lockMeasurePair lockMeasureMode = iota
	lockMeasureAcquire
	lockMeasureRelease
)

type lockWorkload struct {
	name         string
	mode         lockMeasureMode
	criticalWork int
}

var lockWorkloads = []lockWorkload{
	{"Uniform", lockMeasurePair, 0},
	{"LockContention", lockMeasureAcquire, 128},
	{"UnlockContention", lockMeasureRelease, 128},
}

type lockContention struct {
	name    string
	workers int
}

func lockContentionLevels() []lockContention {
	procs := runtime.GOMAXPROCS(0)
	return []lockContention{
		{"LowContention", max(2, procs/2)},
		{"HighContention", procs * 8},
	}
}

// BenchmarkLock runs the full matrix:
//
//	<LockName>/SingleThread/Uniform
//	<LockName>/MultiThread/<ContentionLevel>/<Workload>
//
// Reports ns/op, B/op, allocs/op, p50/p99/p999 latency, GC count, and GC pause.
// The latency percentile is the measured operation for each workload:
// Uniform measures Lock+Unlock, LockContention measures Lock, and
// UnlockContention measures Unlock while waiters are expected to be present.
func BenchmarkLock(b *testing.B) {
	names := make([]string, 0, len(lockFactories))
	for n := range lockFactories {
		names = append(names, n)
	}
	slices.Sort(names)

	for _, name := range names {
		factory := lockFactories[name]
		b.Run(name, func(b *testing.B) {
			b.Run("SingleThread", func(b *testing.B) {
				benchLockSingleThread(b, factory)
			})

			b.Run("MultiThread", func(b *testing.B) {
				for _, c := range lockContentionLevels() {
					b.Run(c.name, func(b *testing.B) {
						for _, w := range lockWorkloads {
							b.Run(w.name, func(b *testing.B) {
								benchLockConcurrent(b, factory, w, c.workers)
							})
						}
					})
				}
			})
		})
	}
}

func benchLockSingleThread(b *testing.B, factory func() testLocker) {
	lock := factory()
	latencies := make([]int64, b.N)

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		start := time.Now()
		lock.Lock()
		lock.Unlock()
		latencies[i] = time.Since(start).Nanoseconds()
	}
	b.StopTimer()

	runtime.ReadMemStats(&memAfter)
	reportLockMetrics(b, latencies, &memBefore, &memAfter)
}

func benchLockConcurrent(b *testing.B, factory func() testLocker, workload lockWorkload, workers int) {
	lock := factory()

	iterPerWorker := b.N / workers
	if iterPerWorker == 0 {
		iterPerWorker = 1
	}
	totalIters := iterPerWorker * workers

	workerLatencies := make([][]int64, workers)
	for w := range workers {
		workerLatencies[w] = make([]int64, iterPerWorker)
	}

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	var ready sync.WaitGroup
	var wg sync.WaitGroup
	startLine := make(chan struct{})

	ready.Add(workers)
	wg.Add(workers)

	b.ReportAllocs()
	b.ResetTimer()
	for w := range workers {
		go func(id int) {
			defer wg.Done()
			local := workerLatencies[id]
			ready.Done()
			<-startLine
			for i := range iterPerWorker {
				local[i] = measureLockWorkload(lock, workload)
			}
		}(w)
	}
	ready.Wait()
	close(startLine)
	wg.Wait()
	b.StopTimer()

	runtime.ReadMemStats(&memAfter)

	all := make([]int64, 0, totalIters)
	for _, l := range workerLatencies {
		all = append(all, l...)
	}
	reportLockMetrics(b, all, &memBefore, &memAfter)
}

func measureLockWorkload(lock testLocker, workload lockWorkload) int64 {
	switch workload.mode {
	case lockMeasureAcquire:
		start := time.Now()
		lock.Lock()
		elapsed := time.Since(start).Nanoseconds()
		doLockCriticalWork(workload.criticalWork)
		lock.Unlock()
		return elapsed
	case lockMeasureRelease:
		lock.Lock()
		doLockCriticalWork(workload.criticalWork)
		start := time.Now()
		lock.Unlock()
		return time.Since(start).Nanoseconds()
	default:
		start := time.Now()
		lock.Lock()
		doLockCriticalWork(workload.criticalWork)
		lock.Unlock()
		return time.Since(start).Nanoseconds()
	}
}

// BenchmarkLockScaling isolates handoff behavior under a single shared lock.
// It mirrors the targeted queue benchmarks by sweeping workers at 1x, 2x, and
// 8x GOMAXPROCS. Every worker repeatedly contends for the same lock, performs
// a fixed short critical section, then releases it.
//
// Reports separate acquire/unlock/pair p50/p99/p999 latency. The acquire
// latency captures queueing and cache-line contention, unlock latency captures
// the handoff release path, and pair latency captures the whole critical
// section transaction.
func BenchmarkLockScaling(b *testing.B) {
	procs := runtime.GOMAXPROCS(0)
	multipliers := []int{1, 2, 8}

	names := make([]string, 0, len(lockFactories))
	for n := range lockFactories {
		names = append(names, n)
	}
	slices.Sort(names)

	for _, name := range names {
		factory := lockFactories[name]
		b.Run(name, func(b *testing.B) {
			for _, m := range multipliers {
				workers := max(procs*m, 1)
				b.Run(fmt.Sprintf("Workers=%d", workers), func(b *testing.B) {
					benchLockScaling(b, factory, workers)
				})
			}
		})
	}
}

func benchLockScaling(b *testing.B, factory func() testLocker, workers int) {
	lock := factory()

	iterPerWorker := b.N / workers
	if iterPerWorker == 0 {
		iterPerWorker = 1
	}
	totalIters := iterPerWorker * workers

	acqLat := make([][]int64, workers)
	unlockLat := make([][]int64, workers)
	pairLat := make([][]int64, workers)
	for w := range workers {
		acqLat[w] = make([]int64, iterPerWorker)
		unlockLat[w] = make([]int64, iterPerWorker)
		pairLat[w] = make([]int64, iterPerWorker)
	}

	var ready sync.WaitGroup
	var wg sync.WaitGroup
	startLine := make(chan struct{})

	ready.Add(workers)
	wg.Add(workers)

	b.ReportAllocs()
	b.ResetTimer()
	for w := range workers {
		go func(id int) {
			defer wg.Done()
			localAcq := acqLat[id]
			localUnlock := unlockLat[id]
			localPair := pairLat[id]
			ready.Done()
			<-startLine

			for i := range iterPerWorker {
				pairStart := time.Now()

				acqStart := pairStart
				lock.Lock()
				localAcq[i] = time.Since(acqStart).Nanoseconds()

				doLockCriticalWork(128)

				unlockStart := time.Now()
				lock.Unlock()
				localUnlock[i] = time.Since(unlockStart).Nanoseconds()
				localPair[i] = time.Since(pairStart).Nanoseconds()
			}
		}(w)
	}
	ready.Wait()
	close(startLine)
	wg.Wait()
	b.StopTimer()

	reportLockScalingMetrics(b, acqLat, unlockLat, pairLat, totalIters)
}

func reportLockScalingMetrics(b *testing.B, acqLat, unlockLat, pairLat [][]int64, totalIters int) {
	acqAll := flattenLockLatencies(acqLat, totalIters)
	unlockAll := flattenLockLatencies(unlockLat, totalIters)
	pairAll := flattenLockLatencies(pairLat, totalIters)

	slices.Sort(acqAll)
	slices.Sort(unlockAll)
	slices.Sort(pairAll)

	b.ReportMetric(float64(lockPercentile(acqAll, 0.50)), "acq-p50")
	b.ReportMetric(float64(lockPercentile(acqAll, 0.99)), "acq-p99")
	b.ReportMetric(float64(lockPercentile(acqAll, 0.999)), "acq-p999")
	b.ReportMetric(float64(lockPercentile(unlockAll, 0.50)), "unlock-p50")
	b.ReportMetric(float64(lockPercentile(unlockAll, 0.99)), "unlock-p99")
	b.ReportMetric(float64(lockPercentile(unlockAll, 0.999)), "unlock-p999")
	b.ReportMetric(float64(lockPercentile(pairAll, 0.50)), "pair-p50")
	b.ReportMetric(float64(lockPercentile(pairAll, 0.99)), "pair-p99")
	b.ReportMetric(float64(lockPercentile(pairAll, 0.999)), "pair-p999")
}

func flattenLockLatencies(latencies [][]int64, totalIters int) []int64 {
	all := make([]int64, 0, totalIters)
	for _, l := range latencies {
		all = append(all, l...)
	}
	return all
}

var lockBenchSink uint64

func doLockCriticalWork(iterations int) {
	x := lockBenchSink
	for i := range iterations {
		x += uint64(i) + 0x9e3779b97f4a7c15
		x ^= x >> 7
	}
	lockBenchSink = x
}

func reportLockMetrics(b *testing.B, latencies []int64, memBefore, memAfter *runtime.MemStats) {
	if len(latencies) == 0 {
		return
	}
	slices.Sort(latencies)
	b.ReportMetric(float64(lockPercentile(latencies, 0.50)), "p50-ns/op")
	b.ReportMetric(float64(lockPercentile(latencies, 0.99)), "p99-ns/op")
	b.ReportMetric(float64(lockPercentile(latencies, 0.999)), "p999-ns/op")
	b.ReportMetric(float64(memAfter.NumGC-memBefore.NumGC), "gc-count")
	b.ReportMetric(float64(memAfter.PauseTotalNs-memBefore.PauseTotalNs)/1e6, "gc-pause-ms")
}

func lockPercentile(sorted []int64, p float64) int64 {
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
