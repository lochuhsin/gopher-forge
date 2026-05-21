package stack

import (
	"math/rand/v2"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
)

var stackFactories = map[string]func() Stack{
	"MutexSliceMPMC": func() Stack {
		return NewMutexSliceMPMC()
	},
	"MutexLinkedMPMC": func() Stack {
		return NewMutexLinkedMPMC()
	},
	"LockFreeMPMC": func() Stack {
		return NewLockFreeMPMC()
	},
	"EliminationBackoffMPMC": func() Stack {
		return NewEliminationBackoffMPMC()
	},
}

const prefillSize = 1024

type workload struct {
	name      string
	pushRatio int // 0-100, remainder is pop
}

var workloads = []workload{
	{"WriteHeavy", 90},
	{"Balanced", 50},
	{"ReadHeavy", 10},
}

type contention struct {
	name    string
	workers int
}

func contentionLevels() []contention {
	procs := runtime.GOMAXPROCS(0)
	return []contention{
		{"LowContention", max(2, procs/2)},
		{"HighContention", procs * 8},
	}
}

// BenchmarkStack runs the full matrix:
//
//	<StackName>/SingleThread/<Workload>
//	<StackName>/MultiThread/<ContentionLevel>/<Workload>
//
// Reports ns/op, B/op, allocs/op, p50/p99 latency, GC count, and GC pause.
func BenchmarkStack(b *testing.B) {
	names := make([]string, 0, len(stackFactories))
	for n := range stackFactories {
		names = append(names, n)
	}
	slices.Sort(names)

	for _, name := range names {
		factory := stackFactories[name]
		b.Run(name, func(b *testing.B) {
			b.Run("SingleThread", func(b *testing.B) {
				for _, w := range workloads {
					b.Run(w.name, func(b *testing.B) {
						benchSingleThread(b, factory, w.pushRatio)
					})
				}
			})

			b.Run("MultiThread", func(b *testing.B) {
				for _, c := range contentionLevels() {
					b.Run(c.name, func(b *testing.B) {
						for _, w := range workloads {
							b.Run(w.name, func(b *testing.B) {
								benchConcurrent(b, factory, w.pushRatio, c.workers)
							})
						}
					})
				}
			})
		})
	}
}

func benchSingleThread(b *testing.B, factory func() Stack, pushRatio int) {
	s := factory()
	if pushRatio < 50 {
		// read-heavy: pre-fill so Pop is not just hitting empty.
		for i := range prefillSize {
			s.Push(i)
		}
	}

	r := rand.New(rand.NewPCG(1, 2))
	ops := make([]bool, b.N)
	for i := range ops {
		ops[i] = r.IntN(100) < pushRatio
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
			s.Push(i)
		} else {
			s.Pop()
		}
		latencies[i] = time.Since(start).Nanoseconds()
	}
	b.StopTimer()

	runtime.ReadMemStats(&memAfter)
	reportMetrics(b, latencies, &memBefore, &memAfter)
}

func benchConcurrent(b *testing.B, factory func() Stack, pushRatio, workers int) {
	s := factory()
	if pushRatio < 50 {
		for i := range prefillSize {
			s.Push(i)
		}
	}

	iterPerWorker := b.N / workers
	if iterPerWorker == 0 {
		iterPerWorker = 1
	}
	totalIters := iterPerWorker * workers

	// Pre-generate each worker's op sequence so the timed loop doesn't
	// include rand work.
	workerOps := make([][]bool, workers)
	for w := range workers {
		r := rand.New(rand.NewPCG(uint64(w)+1, 42))
		ops := make([]bool, iterPerWorker)
		for i := range ops {
			ops[i] = r.IntN(100) < pushRatio
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
			for i, isPush := range ops {
				start := time.Now()
				if isPush {
					s.Push(i)
				} else {
					s.Pop()
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
	reportMetrics(b, all, &memBefore, &memAfter)
}

func reportMetrics(b *testing.B, latencies []int64, memBefore, memAfter *runtime.MemStats) {
	if len(latencies) == 0 {
		return
	}
	slices.Sort(latencies)
	b.ReportMetric(float64(percentile(latencies, 0.50)), "p50-ns/op")
	b.ReportMetric(float64(percentile(latencies, 0.99)), "p99-ns/op")
	b.ReportMetric(float64(memAfter.NumGC-memBefore.NumGC), "gc-count")
	b.ReportMetric(float64(memAfter.PauseTotalNs-memBefore.PauseTotalNs)/1e6, "gc-pause-ms")
}

func percentile(sorted []int64, p float64) int64 {
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
