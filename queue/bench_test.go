package queue

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
)

var mpmcQueueFactories = map[string]func() Queue[int]{
	"MutexMPMC":          func() Queue[int] { return NewMutexMPMC[int]() },
	"LockFreeMPMC":       func() Queue[int] { return NewLockFreeMPMC[int]() },
	"LockFreePaddedMPMC": func() Queue[int] { return NewLockFreePaddedMPMC[int]() },
}

var mpscQueueFactories = map[string]func() Queue[int]{
	"MutexMPSC":    func() Queue[int] { return NewMutexMPSC[int]() },
	"LockFreeMPSC": func() Queue[int] { return NewLockFreeMPSC[int]() },
}

// Note: every op samples time.Now() to record latency, adding ~30ns of
// measurement overhead. All implementations pay this equally, so the
// relative comparison still holds.
//
// On a bounded queue (cap=128), a full Enqueue returns false. This
// benchmark does not retry — failed Enqueues still count as ops, so
// EnqueueHeavy throughput is inflated by "free" no-ops.

const qPrefillSize = 64

type qWorkload struct {
	name         string
	enqueueRatio int // 0-100, remainder is dequeue
}

var qWorkloads = []qWorkload{
	{"EnqueueHeavy", 90},
	{"Balanced", 50},
	{"DequeueHeavy", 10},
}

type qContention struct {
	name    string
	workers int
}

func qContentionLevels() []qContention {
	procs := runtime.GOMAXPROCS(0)
	return []qContention{
		{"LowContention", max(2, procs/2)},
		{"HighContention", procs * 8},
	}
}

// BenchmarkQueue runs the full matrix:
//
//	<QueueName>/SingleThread/<Workload>
//	<QueueName>/MultiThread/<ContentionLevel>/<Workload>
//
// Reports ns/op, B/op, allocs/op, p50/p99 latency, GC count, and GC pause.
func BenchmarkQueue(b *testing.B) {
	names := make([]string, 0, len(mpmcQueueFactories))
	for n := range mpmcQueueFactories {
		names = append(names, n)
	}
	slices.Sort(names)

	for _, name := range names {
		factory := mpmcQueueFactories[name]
		b.Run(name, func(b *testing.B) {
			b.Run("SingleThread", func(b *testing.B) {
				for _, w := range qWorkloads {
					b.Run(w.name, func(b *testing.B) {
						benchQSingleThread(b, factory, w.enqueueRatio)
					})
				}
			})

			b.Run("MultiThread", func(b *testing.B) {
				for _, c := range qContentionLevels() {
					b.Run(c.name, func(b *testing.B) {
						for _, w := range qWorkloads {
							b.Run(w.name, func(b *testing.B) {
								benchQConcurrent(b, factory, w.enqueueRatio, c.workers)
							})
						}
					})
				}
			})
		})
	}
}

func benchQSingleThread(b *testing.B, factory func() Queue[int], enqueueRatio int) {
	q := factory()
	if enqueueRatio < 50 {
		// dequeue-heavy: pre-fill so Dequeue is not just hitting empty.
		for i := range qPrefillSize {
			q.Enqueue(i)
		}
	}

	r := rand.New(rand.NewPCG(1, 2))
	ops := make([]bool, b.N)
	for i := range ops {
		ops[i] = r.IntN(100) < enqueueRatio
	}
	latencies := make([]int64, b.N)

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		start := time.Now()
		if ops[i] {
			q.Enqueue(i)
		} else {
			q.Dequeue()
		}
		latencies[i] = time.Since(start).Nanoseconds()
	}
	b.StopTimer()

	runtime.ReadMemStats(&memAfter)
	reportQMetrics(b, latencies, &memBefore, &memAfter)
}

func benchQConcurrent(b *testing.B, factory func() Queue[int], enqueueRatio, workers int) {
	q := factory()
	if enqueueRatio < 50 {
		for i := range qPrefillSize {
			q.Enqueue(i)
		}
	}

	iterPerWorker := b.N / workers
	if iterPerWorker == 0 {
		iterPerWorker = 1
	}
	totalIters := iterPerWorker * workers

	workerOps := make([][]bool, workers)
	for w := range workers {
		r := rand.New(rand.NewPCG(uint64(w)+1, 42))
		ops := make([]bool, iterPerWorker)
		for i := range ops {
			ops[i] = r.IntN(100) < enqueueRatio
		}
		workerOps[w] = ops
	}
	workerLatencies := make([][]int64, workers)

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	b.ReportAllocs()
	b.ResetTimer()
	var wg sync.WaitGroup
	for w := range workers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ops := workerOps[id]
			local := make([]int64, len(ops))
			for i, isEnqueue := range ops {
				start := time.Now()
				if isEnqueue {
					q.Enqueue(i)
				} else {
					q.Dequeue()
				}
				local[i] = time.Since(start).Nanoseconds()
			}
			workerLatencies[id] = local
		}(w)
	}
	wg.Wait()
	b.StopTimer()

	runtime.ReadMemStats(&memAfter)

	all := make([]int64, 0, totalIters)
	for _, l := range workerLatencies {
		all = append(all, l...)
	}
	reportQMetrics(b, all, &memBefore, &memAfter)
}

func reportQMetrics(b *testing.B, latencies []int64, memBefore, memAfter *runtime.MemStats) {
	if len(latencies) == 0 {
		return
	}
	slices.Sort(latencies)
	b.ReportMetric(float64(qPercentile(latencies, 0.50)), "p50-ns/op")
	b.ReportMetric(float64(qPercentile(latencies, 0.99)), "p99-ns/op")
	b.ReportMetric(float64(memAfter.NumGC-memBefore.NumGC), "gc-count")
	b.ReportMetric(float64(memAfter.PauseTotalNs-memBefore.PauseTotalNs)/1e6, "gc-pause-ms")
}

func qPercentile(sorted []int64, p float64) int64 {
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// BenchmarkMPMCQueue isolates false sharing by running padded and
// unpadded MPMC queues side by side. Design notes:
//
//  1. Producer/consumer split — half the workers Enqueue only, the
//     other half Dequeue only. tail is hot for producers, head for
//     consumers. In the unpadded queue both atomics share a cache
//     line, so each write invalidates the other side; the padded
//     version puts them on independent lines.
//
//  2. Hot spin retry (no Gosched) — producers and consumers spin
//     until success, keeping cache-line contention pinned.
//
//  3. Latency is still sampled with time.Now (~30ns overhead);
//     padded and unpadded both pay it, so the ratio is meaningful
//     even if absolute ns/op is inflated.
//
//  4. Oversubscribe — workers = 1×, 2×, 4×, 8× GOMAXPROCS. At 8×
//     the scheduler shuffles goroutines aggressively, multiplying
//     L1 misses and cache-line invalidations.
func BenchmarkMPMCQueue(b *testing.B) {
	procs := runtime.GOMAXPROCS(0)
	targets := []struct {
		name string
		make func() Queue[int]
	}{
		{"Mutex", func() Queue[int] { return NewMutexMPMC[int]() }},
		{"Unpadded", func() Queue[int] {
			q := &LockFreeMPMC[int]{}
			for i := range BoundedQueueSize {
				q.arr[i].seq.Store(uint64(i))
			}
			return q
		}},
		{"Padded", func() Queue[int] { return NewLockFreePaddedMPMC[int]() }},
	}
	multipliers := []int{1, 2, 8}

	for _, t := range targets {
		b.Run(t.name, func(b *testing.B) {
			for _, m := range multipliers {
				half := max((procs*m)/2, 1)
				producers, consumers := half, half
				b.Run(fmt.Sprintf("Workers=%d", producers+consumers), func(b *testing.B) {
					benchMPMCQ(b, t.make, producers, consumers)
				})
			}
		})
	}
}

func benchMPMCQ(b *testing.B, makeQ func() Queue[int], producers, consumers int) {
	q := makeQ()
	// Pre-fill to half capacity so neither side starves immediately.
	for i := range BoundedQueueSize / 2 {
		q.Enqueue(i)
	}

	total := producers + consumers
	iters := b.N / total
	if iters == 0 {
		iters = 1
	}

	var wg sync.WaitGroup

	// Each worker owns its latency slice; merging happens after the
	// hot loop so the timed section stays lock-free.
	pLat := make([][]int64, producers)
	cLat := make([][]int64, consumers)

	b.ReportAllocs()
	b.ResetTimer()

	for id := range producers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			local := make([]int64, iters)
			for k := range iters {
				start := time.Now()
				for !q.Enqueue(k) {
				}
				local[k] = time.Since(start).Nanoseconds()
			}
			pLat[id] = local
		}(id)
	}

	for id := range consumers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			local := make([]int64, iters)
			for k := range iters {
				start := time.Now()
				for {
					if _, ok := q.Dequeue(); ok {
						break
					}
				}
				local[k] = time.Since(start).Nanoseconds()
			}
			cLat[id] = local
		}(id)
	}

	wg.Wait()
	b.StopTimer()

	pAll := make([]int64, 0, producers*iters)
	for _, l := range pLat {
		pAll = append(pAll, l...)
	}
	slices.Sort(pAll)

	cAll := make([]int64, 0, consumers*iters)
	for _, l := range cLat {
		cAll = append(cAll, l...)
	}
	slices.Sort(cAll)

	b.ReportMetric(float64(qPercentile(pAll, 0.50)), "enq-p50")
	b.ReportMetric(float64(qPercentile(pAll, 0.99)), "enq-p99")
	b.ReportMetric(float64(qPercentile(pAll, 0.999)), "enq-p999")
	b.ReportMetric(float64(qPercentile(cAll, 0.50)), "deq-p50")
	b.ReportMetric(float64(qPercentile(cAll, 0.99)), "deq-p99")
	b.ReportMetric(float64(qPercentile(cAll, 0.999)), "deq-p999")
}

func BenchmarkMPSCQueue(b *testing.B) {
	procs := runtime.GOMAXPROCS(0)
	targets := []struct {
		name string
		make func() Queue[int]
	}{
		{"Mutex", func() Queue[int] { return NewMutexMPMC[int]() }},
		{"MPMC-Unpadded", func() Queue[int] { return NewLockFreeMPMC[int]() }},
		{"MPMC-Padded", func() Queue[int] { return NewLockFreePaddedMPMC[int]() }},
		{"MPSC", func() Queue[int] { return NewLockFreeMPSC[int]() }},
	}
	multipliers := []int{1, 2, 8}

	for _, t := range targets {
		b.Run(t.name, func(b *testing.B) {
			for _, m := range multipliers {
				producers := max(procs*m-1, 1)
				b.Run(fmt.Sprintf("Producers=%d", producers), func(b *testing.B) {
					benchQMPSC(b, t.make, producers)
				})
			}
		})
	}
}

func benchQMPSC(b *testing.B, makeQ func() Queue[int], producers int) {
	q := makeQ()
	for i := range BoundedQueueSize / 2 {
		q.Enqueue(i)
	}

	iters := b.N / (2 * producers)
	if iters == 0 {
		iters = 1
	}
	totalProducerOps := producers * iters

	var wg sync.WaitGroup
	pLat := make([][]int64, producers)
	cLat := make([]int64, totalProducerOps)

	b.ReportAllocs()
	b.ResetTimer()

	for id := range producers {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			local := make([]int64, iters)
			for k := range iters {
				start := time.Now()
				for !q.Enqueue(k) {
				}
				local[k] = time.Since(start).Nanoseconds()
			}
			pLat[id] = local
		}(id)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for k := range totalProducerOps {
			start := time.Now()
			for {
				if _, ok := q.Dequeue(); ok {
					break
				}
			}
			cLat[k] = time.Since(start).Nanoseconds()
		}
	}()

	wg.Wait()
	b.StopTimer()

	pAll := make([]int64, 0, totalProducerOps)
	for _, l := range pLat {
		pAll = append(pAll, l...)
	}
	slices.Sort(pAll)
	slices.Sort(cLat)

	b.ReportMetric(float64(qPercentile(pAll, 0.50)), "enq-p50")
	b.ReportMetric(float64(qPercentile(pAll, 0.99)), "enq-p99")
	b.ReportMetric(float64(qPercentile(pAll, 0.999)), "enq-p999")
	b.ReportMetric(float64(qPercentile(cLat, 0.50)), "deq-p50")
	b.ReportMetric(float64(qPercentile(cLat, 0.99)), "deq-p99")
	b.ReportMetric(float64(qPercentile(cLat, 0.999)), "deq-p999")
}
