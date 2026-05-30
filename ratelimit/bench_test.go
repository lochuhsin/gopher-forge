package ratelimit

import (
	"context"
	"fmt"
	"runtime"
	"slices"
	"sync"
	"testing"
	"time"
)

// Mirrors queue/bench_test.go conventions: latency sampled with time.Now()
// (~30ns floor on Apple silicon), reports p50/p99/p999 + B/op + allocs/op
// + GC count + GC pause.
//
// Matrix:
//
//	BenchmarkTokenBucketAllow:       <Scenario>/SingleThread
//	                                 <Scenario>/MultiThread/<ContentionLevel>
//	BenchmarkTokenBucketAllowN:      N=1,4,16,64
//	BenchmarkTokenBucketWait:        Rate=1000,10000,100000
//	BenchmarkTokenBucket1to1Uniform: Work=None,Light,Heavy

type rlScenario struct {
	name string
	make func() *TokenBucket
}

var rlScenarios = []rlScenario{
	{"Unlimited", func() *TokenBucket { return NewTokenBucket(1_000_000_000, 1_000_000_000) }},
	{"Throttled", func() *TokenBucket { return NewTokenBucket(1000, 1_000_000) }},
	{"Saturated", func() *TokenBucket { return NewTokenBucket(10, 10) }},
}

type rlContention struct {
	name    string
	workers int
}

func rlContentionLevels() []rlContention {
	procs := runtime.GOMAXPROCS(0)
	return []rlContention{
		{"LowContention", max(2, procs/2)},
		{"HighContention", procs * 8},
	}
}

func BenchmarkTokenBucketAllow(b *testing.B) {
	for _, s := range rlScenarios {
		b.Run(s.name, func(b *testing.B) {
			b.Run("SingleThread", func(b *testing.B) {
				benchRLAllowSingle(b, s.make)
			})
			b.Run("MultiThread", func(b *testing.B) {
				for _, c := range rlContentionLevels() {
					b.Run(c.name, func(b *testing.B) {
						benchRLAllowConcurrent(b, s.make, c.workers)
					})
				}
			})
		})
	}
}

func benchRLAllowSingle(b *testing.B, makeBucket func() *TokenBucket) {
	tb := makeBucket()
	latencies := make([]int64, b.N)

	var memBefore, memAfter runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&memBefore)

	b.ReportAllocs()
	b.ResetTimer()
	for i := range b.N {
		start := time.Now()
		tb.Allow()
		latencies[i] = time.Since(start).Nanoseconds()
	}
	b.StopTimer()

	runtime.ReadMemStats(&memAfter)
	reportRLMetrics(b, latencies, &memBefore, &memAfter)
}

func benchRLAllowConcurrent(b *testing.B, makeBucket func() *TokenBucket, workers int) {
	tb := makeBucket()
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
				tb.Allow()
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
	reportRLMetrics(b, all, &memBefore, &memAfter)
}

func BenchmarkTokenBucketAllowN(b *testing.B) {
	ns := []uint64{1, 4, 16, 64}
	for _, n := range ns {
		b.Run(fmt.Sprintf("N=%d", n), func(b *testing.B) {
			tb := NewTokenBucket(1_000_000, 1_000_000_000)
			latencies := make([]int64, b.N)

			var memBefore, memAfter runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memBefore)

			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				start := time.Now()
				tb.AllowN(n)
				latencies[i] = time.Since(start).Nanoseconds()
			}
			b.StopTimer()

			runtime.ReadMemStats(&memAfter)
			reportRLMetrics(b, latencies, &memBefore, &memAfter)
		})
	}
}

func BenchmarkTokenBucketWait(b *testing.B) {
	targets := []struct {
		name string
		rate uint64
	}{
		{"Rate=1000", 1000},
		{"Rate=10000", 10000},
		{"Rate=100000", 100_000},
	}

	for _, t := range targets {
		b.Run(t.name, func(b *testing.B) {
			tb := NewTokenBucket(1, t.rate)
			tb.Allow()
			ctx := context.Background()
			latencies := make([]int64, b.N)

			var memBefore, memAfter runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memBefore)

			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				start := time.Now()
				_ = tb.Wait(ctx)
				latencies[i] = time.Since(start).Nanoseconds()
			}
			b.StopTimer()

			runtime.ReadMemStats(&memAfter)
			reportRLMetrics(b, latencies, &memBefore, &memAfter)
		})
	}
}

var rlWorkSink int

//go:noinline
func rlSimulateWork(iters int) int {
	x := 1
	for range iters {
		x = x*1103515245 + 12345
		x ^= x >> 7
	}
	return x
}

func BenchmarkTokenBucket1to1Uniform(b *testing.B) {
	workLevels := []struct {
		name  string
		iters int
	}{
		{"Work=None", 0},
		{"Work=Light", 48},
		{"Work=Heavy", 240},
	}

	for _, w := range workLevels {
		b.Run(w.name, func(b *testing.B) {
			tb := NewTokenBucket(1_000_000, 1_000_000_000)
			latencies := make([]int64, b.N)
			sink := 0

			var memBefore, memAfter runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&memBefore)

			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				start := time.Now()
				tb.Allow()
				latencies[i] = time.Since(start).Nanoseconds()
				sink += rlSimulateWork(w.iters)
			}
			b.StopTimer()
			rlWorkSink = sink

			runtime.ReadMemStats(&memAfter)
			reportRLMetrics(b, latencies, &memBefore, &memAfter)
		})
	}
}

func reportRLMetrics(b *testing.B, latencies []int64, memBefore, memAfter *runtime.MemStats) {
	if len(latencies) == 0 {
		return
	}
	slices.Sort(latencies)
	b.ReportMetric(float64(rlPercentile(latencies, 0.50)), "p50-ns/op")
	b.ReportMetric(float64(rlPercentile(latencies, 0.99)), "p99-ns/op")
	b.ReportMetric(float64(rlPercentile(latencies, 0.999)), "p999-ns/op")
	b.ReportMetric(float64(memAfter.NumGC-memBefore.NumGC), "gc-count")
	b.ReportMetric(float64(memAfter.PauseTotalNs-memBefore.PauseTotalNs)/1e6, "gc-pause-ms")
}

func rlPercentile(sorted []int64, p float64) int64 {
	idx := int(float64(len(sorted)-1) * p)
	return sorted[idx]
}
