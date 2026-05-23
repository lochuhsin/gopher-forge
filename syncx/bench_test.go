package syncx

import (
	"fmt"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
)

var semaphoreFactories = map[string]func(int) Semaphore{
	"Channel":  func(n int) Semaphore { return NewChannelSemaphore(n) },
	"Mutex":    func(n int) Semaphore { return NewMutexSemaphore(n) },
	"Cond":     func(n int) Semaphore { return NewCondSemaphore(n) },
	"Lockfree": func(n int) Semaphore { return NewLockfreeSemaphore(n) },
}

// Note: every op samples time.Now() to record latency, adding ~30ns of
// measurement overhead. All implementations pay this equally, so the
// relative comparison still holds.

// Three points along the permit-contention curve. For the semaphore,
// the analog of stack's push/pop ratio is "how often does Acquire
// actually block" — which is governed by permits-vs-workers.
//
//   - Binary: one permit, every worker serializes through Acquire.
//   - Small:  4 permits, moderate parking/wakeup pressure.
//   - Loose:  permits == workers, every Acquire fast-paths.
type semWorkload struct {
	name    string
	permits int // 0 sentinel: resolve to workers at call site (uncontended)
}

var semWorkloads = []semWorkload{
	{"Binary", 1},
	{"Small", 4},
	{"Loose", 0},
}

type semContention struct {
	name    string
	workers int
}

func semContentionLevels() []semContention {
	procs := runtime.GOMAXPROCS(0)
	return []semContention{
		{"LowContention", max(2, procs/2)},
		{"HighContention", procs * 8},
	}
}

// BenchmarkSemaphore runs the full matrix:
//
//	<Impl>/SingleThread
//	<Impl>/MultiThread/<ContentionLevel>/<Permits>
//
// Reports ns/op, B/op, allocs/op, p50/p99 latency, GC count, and GC pause.
func BenchmarkSemaphore(b *testing.B) {
	names := make([]string, 0, len(semaphoreFactories))
	for n := range semaphoreFactories {
		names = append(names, n)
	}
	slices.Sort(names)

	for _, name := range names {
		factory := semaphoreFactories[name]
		b.Run(name, func(b *testing.B) {
			b.Run("SingleThread", func(b *testing.B) {
				benchSemSingleThread(b, factory)
			})

			b.Run("MultiThread", func(b *testing.B) {
				for _, c := range semContentionLevels() {
					b.Run(c.name, func(b *testing.B) {
						for _, w := range semWorkloads {
							b.Run(w.name, func(b *testing.B) {
								permits := w.permits
								if permits == 0 {
									permits = c.workers
								}
								benchSemConcurrent(b, factory, permits, c.workers)
							})
						}
					})
				}
			})
		})
	}
}

func benchSemSingleThread(b *testing.B, factory func(int) Semaphore) {
	// One goroutine, paired Acquire+Release — permits >= 1 is irrelevant,
	// just measures the fast-path overhead.
	sem := factory(1)
	latencies := make([]int64, b.N)

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		start := time.Now()
		sem.Acquire()
		sem.Release()
		latencies[i] = time.Since(start).Nanoseconds()
	}
	b.StopTimer()

	runtime.ReadMemStats(&memAfter)
	reportSemMetrics(b, latencies, &memBefore, &memAfter)
}

func benchSemConcurrent(b *testing.B, factory func(int) Semaphore, permits, workers int) {
	sem := factory(permits)

	iterPerWorker := b.N / workers
	if iterPerWorker == 0 {
		iterPerWorker = 1
	}
	totalIters := iterPerWorker * workers

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
			local := make([]int64, iterPerWorker)
			for i := range iterPerWorker {
				start := time.Now()
				sem.Acquire()
				sem.Release()
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
	reportSemMetrics(b, all, &memBefore, &memAfter)
}

func reportSemMetrics(b *testing.B, latencies []int64, memBefore, memAfter *runtime.MemStats) {
	if len(latencies) == 0 {
		return
	}
	slices.Sort(latencies)
	b.ReportMetric(float64(semPercentile(latencies, 0.50)), "p50-ns/op")
	b.ReportMetric(float64(semPercentile(latencies, 0.99)), "p99-ns/op")
	b.ReportMetric(float64(memAfter.NumGC-memBefore.NumGC), "gc-count")
	b.ReportMetric(float64(memAfter.PauseTotalNs-memBefore.PauseTotalNs)/1e6, "gc-pause-ms")
}

func semPercentile(sorted []int64, p float64) int64 {
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}

// BenchmarkOrderedChannel mirrors BenchmarkMPSCQueue's shape:
// N producers writing to an in-chan, the transformer in the middle,
// one consumer draining out-chan. Multipliers oversubscribe GOMAXPROCS
// at 1×, 2×, 8× so the scheduler shuffles producers aggressively.
//
// Reports separate send/recv p50/p99/p999 latency. Ordered pays for
// the RingQueue + double-select; Unordered uses a slice fallback —
// the ratio between them isolates the cost of in-order guarantee.
func BenchmarkOrderedChannel(b *testing.B) {
	procs := runtime.GOMAXPROCS(0)
	targets := []struct {
		name string
		make func(<-chan int) <-chan int
	}{
		{"Unordered", UnorderedChannel[int]},
		{"Ordered", OrderedChannel[int]},
	}
	multipliers := []int{1, 2, 8}

	for _, t := range targets {
		b.Run(t.name, func(b *testing.B) {
			for _, m := range multipliers {
				producers := max(procs*m-1, 1)
				b.Run(fmt.Sprintf("Producers=%d", producers), func(b *testing.B) {
					benchChan(b, t.make, producers)
				})
			}
		})
	}
}

func benchChan(b *testing.B, wrap func(<-chan int) <-chan int, producers int) {
	iters := b.N / producers
	if iters == 0 {
		iters = 1
	}
	totalOps := producers * iters

	in := make(chan int, DefaultBuffSize)
	out := wrap(in)

	pLat := make([][]int64, producers)
	cLat := make([]int64, totalOps)

	var pWg, cWg sync.WaitGroup

	b.ReportAllocs()
	b.ResetTimer()

	for id := range producers {
		pWg.Add(1)
		go func(id int) {
			defer pWg.Done()
			local := make([]int64, iters)
			for k := range iters {
				start := time.Now()
				in <- k
				local[k] = time.Since(start).Nanoseconds()
			}
			pLat[id] = local
		}(id)
	}

	cWg.Add(1)
	go func() {
		defer cWg.Done()
		for k := range totalOps {
			start := time.Now()
			<-out
			cLat[k] = time.Since(start).Nanoseconds()
		}
	}()

	pWg.Wait()
	close(in)
	cWg.Wait()
	b.StopTimer()

	pAll := make([]int64, 0, totalOps)
	for _, l := range pLat {
		pAll = append(pAll, l...)
	}
	slices.Sort(pAll)
	slices.Sort(cLat)

	b.ReportMetric(float64(semPercentile(pAll, 0.50)), "send-p50")
	b.ReportMetric(float64(semPercentile(pAll, 0.99)), "send-p99")
	b.ReportMetric(float64(semPercentile(pAll, 0.999)), "send-p999")
	b.ReportMetric(float64(semPercentile(cLat, 0.50)), "recv-p50")
	b.ReportMetric(float64(semPercentile(cLat, 0.99)), "recv-p99")
	b.ReportMetric(float64(semPercentile(cLat, 0.999)), "recv-p999")
}
