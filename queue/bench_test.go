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

// BenchmarkSPSCQueue runs the single-producer / single-consumer scenario:
// exactly one goroutine owns Enqueue, one owns Dequeue. This is the only
// regime the dedicated SPSCQueue supports, and the one its minimal
// synchronization (plain atomic load/store on head and tail, no CAS) is
// built for. The general-purpose lock-free queues and the mutex queue are
// driven through the same 1+1 setup as baselines.
//
// Design notes:
//
//  1. One producer, one consumer — no contention levels. Adding workers
//     would break the SPSC contract (tail is producer-private, head is
//     consumer-private), so there is nothing to oversubscribe; the whole
//     point is the uncontended hand-off cost.
//
//  2. Hot spin retry (no Gosched) — both sides spin until success, so the
//     two hot cache lines (head, tail) stay pinned and what we measure is
//     the producer→consumer hand-off latency rather than scheduler noise.
//
//  3. Pre-fill to half capacity so neither side starves at the start.
//
//  4. Latency buffers are allocated before ResetTimer and the timer is
//     stopped before merging, so B/op and allocs/op reflect only what the
//     queue itself allocates in the hot loop (≈0 for these array-backed
//     queues) — not the measurement scaffolding.
//
//  5. Every op samples time.Now (~30ns overhead); all targets pay it
//     equally. Reports enq/deq p50/p99/p999 plus B/op, allocs/op, GC
//     count and total GC pause.
func BenchmarkSPSCQueue(b *testing.B) {
	targets := []struct {
		name string
		make func() Queue[int]
	}{
		// MutexMPSC is byte-identical to MutexMPMC; both are listed so the
		// matrix literally covers every queue implementation in the package.
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
			benchSPSCQ(b, t.make)
		})
	}
}

func benchSPSCQ(b *testing.B, makeQ func() Queue[int]) {
	q := makeQ()
	// Pre-fill to half capacity so neither side starves immediately.
	for i := range BoundedQueueSize / 2 {
		q.Enqueue(i)
	}

	// Allocated outside the timed region so they do not pollute B/op.
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
			start := time.Now()
			for !q.Enqueue(k) {
			}
			enqLat[k] = time.Since(start).Nanoseconds()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		for k := range b.N {
			start := time.Now()
			for {
				if _, ok := q.Dequeue(); ok {
					break
				}
			}
			deqLat[k] = time.Since(start).Nanoseconds()
		}
	}()

	wg.Wait()
	b.StopTimer()

	runtime.ReadMemStats(&memAfter)

	reportSPSCLatency(b, enqLat, deqLat, &memBefore, &memAfter)
}

// reportSPSCLatency emits the package-standard 1-producer/1-consumer
// columns: enq/deq p50/p99/p999, GC count and pause (B/op + allocs/op come
// from the caller's ReportAllocs). Shared so every SPSC benchmark reports
// an identical, comparable column set.
func reportSPSCLatency(b *testing.B, enqLat, deqLat []int64, memBefore, memAfter *runtime.MemStats) {
	slices.Sort(enqLat)
	slices.Sort(deqLat)
	b.ReportMetric(float64(qPercentile(enqLat, 0.50)), "enq-p50")
	b.ReportMetric(float64(qPercentile(enqLat, 0.99)), "enq-p99")
	b.ReportMetric(float64(qPercentile(enqLat, 0.999)), "enq-p999")
	b.ReportMetric(float64(qPercentile(deqLat, 0.50)), "deq-p50")
	b.ReportMetric(float64(qPercentile(deqLat, 0.99)), "deq-p99")
	b.ReportMetric(float64(qPercentile(deqLat, 0.999)), "deq-p999")
	b.ReportMetric(float64(memAfter.NumGC-memBefore.NumGC), "gc-count")
	b.ReportMetric(float64(memAfter.PauseTotalNs-memBefore.PauseTotalNs)/1e6, "gc-pause-ms")
}

// BenchmarkSPSCThroughput measures raw producer→consumer throughput with
// NO per-op time.Now() sampling.
//
// BenchmarkSPSCQueue calls time.Now() twice per op to record latency. On
// Apple silicon each call is ~30-40ns, so ~60-80ns of clock overhead lands
// on every op — larger than the queue operation itself (~20-40ns). That
// instrument is coarser than the thing it measures, so it cannot resolve
// which queue is faster (note the p50s quantize to 42 vs 83 ns, right at
// the timer floor). Here ns/op is wall-time/b.N over a tight enqueue/
// dequeue loop, so it reflects the queue, not the clock. No prefill: the
// consumer simply spins until the producer gets ahead.
func BenchmarkSPSCThroughput(b *testing.B) {
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
			q := t.make()
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

			for range b.N {
				for {
					if _, ok := q.Dequeue(); ok {
						break
					}
				}
			}

			wg.Wait()
			b.StopTimer()
		})
	}
}

// spscWorkSink keeps simulated work from being optimized away. Benchmarks
// run sequentially, so a plain global (no atomic) is fine.
var spscWorkSink int

// spscSimulateWork burns roughly `iters` units of CPU, standing in for the
// per-item processing a real consumer does (matching, risk checks, ...).
// The dependent multiply/xorshift chain costs ~2ns/iter on Apple silicon
// (measured), which sets the Work= labels below.
//
//go:noinline
func spscSimulateWork(iters int) int {
	x := 1
	for range iters {
		x = x*1103515245 + 12345
		x ^= x >> 7
	}
	return x
}

// BenchmarkSPSC1to1Uniform models ONE producer feeding ONE consumer over a
// uniform (steady, one-item-at-a-time) arrival stream — the SPSC topology
// an exchange feed-handler → matching-engine hot path actually uses. The
// name states the workload: 1to1 = single producer/single consumer,
// Uniform = steady arrival.
//
// The realism lever is the Work= axis: a real consumer does work per item
// (matching, risk checks), not a bare Dequeue. As that work grows it
// becomes the bottleneck, a backlog forms, and the hand-off cost stops
// dominating.
//
// Reports the package-standard columns (enq/deq p50/p99/p999, gc, B/op,
// allocs/op) so it lines up with BenchmarkSPSCQueue. Like every latency-
// sampling benchmark here it pays ~30ns per time.Now(); at Work=None that
// dominates and the percentiles quantize to the timer floor, but once
// Work>0 the per-item cost is what the percentiles actually measure. For a
// clock-free ns/op number, see BenchmarkSPSCThroughput.
//
// Reading it: at Work=None the queue IS the cost and MPMC's slot design
// wins; by Work=Light/Heavy the work dominates and the queues converge —
// at which point you choose a queue for tail determinism + zero GC (and to
// dodge MPMC's multi-producer CAS contention), not for raw ns/op.
func BenchmarkSPSC1to1Uniform(b *testing.B) {
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
		{"Work=None", 0},    // bare queue op, no processing
		{"Work=Light", 48},  // ~100ns/item
		{"Work=Heavy", 240}, // ~500ns/item
	}

	for _, impl := range impls {
		b.Run(impl.name, func(b *testing.B) {
			for _, w := range workLevels {
				b.Run(w.name, func(b *testing.B) {
					benchSPSC1to1(b, impl.make, w.iters)
				})
			}
		})
	}
}

func benchSPSC1to1(b *testing.B, makeQ func() Queue[int], workIters int) {
	q := makeQ()
	// Allocated outside the timed region so they do not pollute B/op.
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
			start := time.Now()
			for !q.Enqueue(k) {
			}
			enqLat[k] = time.Since(start).Nanoseconds()
		}
	}()

	// Consumer on the main goroutine: time only the Dequeue hand-off, then
	// do the per-item work outside the timed region.
	for k := range b.N {
		start := time.Now()
		for {
			if _, ok := q.Dequeue(); ok {
				break
			}
		}
		deqLat[k] = time.Since(start).Nanoseconds()
		sink += spscSimulateWork(workIters)
	}

	wg.Wait()
	b.StopTimer()
	runtime.ReadMemStats(&memAfter)
	spscWorkSink = sink
	reportSPSCLatency(b, enqLat, deqLat, &memBefore, &memAfter)
}
