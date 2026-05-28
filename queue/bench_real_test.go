// Benchmarks in this file mirror the latency benches in bench_test.go but
// replace time.Now() with runtime.nanotime(). On Apple silicon time.Now
// costs ~30-40ns per call (called twice per op = ~60-80ns measurement
// overhead); nanotime is ~10ns. When the queue operation itself is
// 3-15ns, the time.Now floor quantises everything to a single bucket and
// hides the real differences — this file removes that floor.
//
// Run via: make bench-spsc-queue-real
package queue

import (
	"runtime"
	"slices"
	"sync"
	"testing"
	_ "unsafe"
)

// nanotime returns monotonic nanoseconds via the Go runtime. On macOS
// this delegates to mach_absolute_time(); on Linux to
// clock_gettime(CLOCK_MONOTONIC). Both skip the wallclock/timezone path
// that time.Now() walks, dropping per-call cost from ~30-40ns to ~10ns
// on Apple silicon.
//
//go:linkname nanotime runtime.nanotime
func nanotime() int64

const spscRealBatchOps = 256

func spscRealBatchSampleCount(totalOps, opsPerBatch int) int {
	if totalOps <= 0 || opsPerBatch <= 0 {
		return 0
	}
	return (totalOps + opsPerBatch - 1) / opsPerBatch
}

func spscRealBatchSize(totalOps, opsPerBatch, sample int) int {
	if totalOps <= 0 || opsPerBatch <= 0 {
		return 0
	}
	start := sample * opsPerBatch
	if start >= totalOps {
		return 0
	}
	if remaining := totalOps - start; remaining < opsPerBatch {
		return remaining
	}
	return opsPerBatch
}

func qPercentileFloat64(sorted []float64, p float64) float64 {
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// BenchmarkSPSCQueueReal mirrors BenchmarkSPSCQueue (1-producer /
// 1-consumer, hot spin, no per-item work) but times every op with
// nanotime instead of time.Now. With the timer floor cut from ~42ns to
// ~14ns, fast queues (SPSC-Cached ~3ns, MPMC-Padded ~9ns) become
// resolvable at p50 instead of all quantising to one bucket.
func BenchmarkSPSCQueueReal(b *testing.B) {
	targets := []struct {
		name string
		make func() Queue[int]
	}{
		{"MutexMPMC", func() Queue[int] { return NewMutexMPMC[int]() }},
		{"MutexMPSC", func() Queue[int] { return NewMutexMPSC[int]() }},
		{"MPMC-Unpadded", func() Queue[int] { return NewLockFreeMPMC[int]() }},
		{"MPMC-Padded", func() Queue[int] { return NewLockFreePaddedMPMC[int]() }},
		{"MPSC", func() Queue[int] { return NewLockFreeMPSC[int]() }},
		{"SPSC", func() Queue[int] { return NewSPSCQueue[int]() }},
		{"SPSC-Cached", func() Queue[int] { return NewCachedSPSCQueue[int]() }},
	}

	for _, t := range targets {
		b.Run(t.name, func(b *testing.B) {
			benchSPSCQReal(b, t.make)
		})
	}
}

// BenchmarkSPSCQueueRealBatched is the "extreme" SPSC/MPMC comparison.
// It samples one latency point per 256 successful consumer handoffs and then
// divides by 256, so the nanotime/read/write sampling cost is amortized below
// one nanosecond per queue op. The default ns/op remains total wall time per
// item; the batch-pXX-ns/op columns show the p50/p99/p999 windowed cost.
func BenchmarkSPSCQueueRealBatched(b *testing.B) {
	targets := []struct {
		name string
		make func() Queue[int]
	}{
		{"MPMC-Padded", func() Queue[int] { return NewLockFreePaddedMPMC[int]() }},
		{"SPSC", func() Queue[int] { return NewSPSCQueue[int]() }},
		{"SPSC-Cached", func() Queue[int] { return NewCachedSPSCQueue[int]() }},
	}

	for _, t := range targets {
		b.Run(t.name, func(b *testing.B) {
			benchSPSCQRealBatched(b, t.make, spscRealBatchOps)
		})
	}
}

func benchSPSCQRealBatched(b *testing.B, makeQ func() Queue[int], opsPerBatch int) {
	q := makeQ()
	for i := range BoundedQueueSize / 2 {
		q.Enqueue(i)
	}

	samples := spscRealBatchSampleCount(b.N, opsPerBatch)
	batchLat := make([]float64, samples)

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	var wg sync.WaitGroup
	b.ReportAllocs()
	b.ResetTimer()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for k := range b.N {
			for !q.Enqueue(k) {
			}
		}
	}()

	for sample := range samples {
		batchSize := spscRealBatchSize(b.N, opsPerBatch, sample)
		start := nanotime()
		for range batchSize {
			for {
				if _, ok := q.Dequeue(); ok {
					break
				}
			}
		}
		batchLat[sample] = float64(nanotime()-start) / float64(batchSize)
	}

	wg.Wait()
	b.StopTimer()
	runtime.ReadMemStats(&memAfter)
	reportSPSCBatchedLatency(b, batchLat, opsPerBatch, &memBefore, &memAfter)
}

func reportSPSCBatchedLatency(b *testing.B, batchLat []float64, opsPerBatch int, memBefore, memAfter *runtime.MemStats) {
	if len(batchLat) == 0 {
		return
	}
	slices.Sort(batchLat)
	b.ReportMetric(float64(opsPerBatch), "batch-ops")
	b.ReportMetric(qPercentileFloat64(batchLat, 0.50), "batch-p50-ns/op")
	b.ReportMetric(qPercentileFloat64(batchLat, 0.99), "batch-p99-ns/op")
	b.ReportMetric(qPercentileFloat64(batchLat, 0.999), "batch-p999-ns/op")
	b.ReportMetric(float64(memAfter.NumGC-memBefore.NumGC), "gc-count")
	b.ReportMetric(float64(memAfter.PauseTotalNs-memBefore.PauseTotalNs)/1e6, "gc-pause-ms")
}

func benchSPSCQReal(b *testing.B, makeQ func() Queue[int]) {
	q := makeQ()
	for i := range BoundedQueueSize / 2 {
		q.Enqueue(i)
	}

	enqLat := make([]int64, b.N)
	deqLat := make([]int64, b.N)

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	var wg sync.WaitGroup
	b.ReportAllocs()
	b.ResetTimer()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for k := range b.N {
			start := nanotime()
			for !q.Enqueue(k) {
			}
			enqLat[k] = nanotime() - start
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for k := range b.N {
			start := nanotime()
			for {
				if _, ok := q.Dequeue(); ok {
					break
				}
			}
			deqLat[k] = nanotime() - start
		}
	}()

	wg.Wait()
	b.StopTimer()
	runtime.ReadMemStats(&memAfter)

	reportSPSCLatency(b, enqLat, deqLat, &memBefore, &memAfter)
}

// BenchmarkSPSC1to1UniformReal mirrors BenchmarkSPSC1to1Uniform (steady
// 1-to-1 stream with optional per-item work) but uses nanotime for both
// producer and consumer timing.
func BenchmarkSPSC1to1UniformReal(b *testing.B) {
	impls := []struct {
		name string
		make func() Queue[int]
	}{
		{"MPMC-Padded", func() Queue[int] { return NewLockFreePaddedMPMC[int]() }},
		{"SPSC", func() Queue[int] { return NewSPSCQueue[int]() }},
		{"SPSC-Cached", func() Queue[int] { return NewCachedSPSCQueue[int]() }},
	}
	workLevels := []struct {
		name  string
		iters int
	}{
		{"Work=None", 0},
		{"Work=Light", 48},
		{"Work=Heavy", 240},
	}

	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			for _, w := range workLevels {
				b.Run(w.name, func(b *testing.B) {
					benchSPSC1to1Real(b, impl.make, w.iters)
				})
			}
		})
	}
}

func benchSPSC1to1Real(b *testing.B, makeQ func() Queue[int], workIters int) {
	q := makeQ()
	enqLat := make([]int64, b.N)
	deqLat := make([]int64, b.N)
	sink := 0

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	var wg sync.WaitGroup
	b.ReportAllocs()
	b.ResetTimer()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for k := range b.N {
			start := nanotime()
			for !q.Enqueue(k) {
			}
			enqLat[k] = nanotime() - start
		}
	}()

	for k := range b.N {
		start := nanotime()
		for {
			if _, ok := q.Dequeue(); ok {
				break
			}
		}
		deqLat[k] = nanotime() - start
		sink += spscSimulateWork(workIters)
	}

	wg.Wait()
	b.StopTimer()
	runtime.ReadMemStats(&memAfter)
	spscWorkSink = sink
	reportSPSCLatency(b, enqLat, deqLat, &memBefore, &memAfter)
}
