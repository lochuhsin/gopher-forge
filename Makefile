.DEFAULT_GOAL := help

BENCHTIME ?= 3s
PKG       := ./...
COMMON    := -benchmem -benchtime=$(BENCHTIME) -run=^$$
PPROF_PORT ?= 8080

.PHONY: help \
        test test-short test-stress test-chaos \
        bench bench-single bench-multi bench-scale bench-mpmc-queue bench-mpsc-queue \
        bench-cpu bench-mem bench-full \
        cpu-prof mem-prof clean

help:
	@echo "Tests (concurrency correctness):"
	@echo "  test           Run all tests with -race (recommended default)"
	@echo "  test-short     Run -race tests, skip slow chaos test"
	@echo "  test-stress    Run -race tests with -count=5 (catch flaky bugs)"
	@echo "  test-chaos     Only the chaos test (multi-GOMAXPROCS × many rounds)"
	@echo ""
	@echo "Benchmarks (performance):"
	@echo "  bench          Run all benchmarks (BENCHTIME=$(BENCHTIME))"
	@echo "  bench-single   Run SingleThread benchmarks only"
	@echo "  bench-multi    Run MultiThread benchmarks only"
	@echo "  bench-scale    Run MultiThread at GOMAXPROCS=1,2,4,8 (scaling curve)"
	@echo "  bench-mpmc-queue  Run MPMC queue benchmark (Mutex vs Unpadded vs Padded)"
	@echo "  bench-mpsc-queue  Run MPSC queue benchmark (single consumer)"
	@echo ""
	@echo "  bench-cpu      Run all benchmarks + write cpu.out"
	@echo "  bench-mem      Run all benchmarks + write mem.out"
	@echo "  bench-full     Run all benchmarks + cpu.out + mem.out"
	@echo ""
	@echo "Profiles:"
	@echo "  cpu-prof       Open cpu.out in pprof web UI (port $(PPROF_PORT))"
	@echo "  mem-prof       Open mem.out in pprof web UI (port $(PPROF_PORT))"
	@echo "  clean          Remove profile + test binaries"
	@echo ""
	@echo "Variables:"
	@echo "  BENCHTIME      Per-benchmark duration   (default: 3s)"
	@echo "  PPROF_PORT     Port for pprof web UI    (default: 8080)"

test:
	go test -race -v $(PKG)

test-short:
	go test -race -short -v $(PKG)

test-stress:
	go test -race -count=5 $(PKG)

test-chaos:
	go test -race -v -run='TestConcurrentChaos' $(PKG)

bench:
	go test -bench=. $(COMMON) $(PKG)

bench-single:
	go test -bench='BenchmarkStack/[^/]+/SingleThread' $(COMMON) $(PKG)

bench-multi:
	go test -bench='BenchmarkStack/[^/]+/MultiThread' $(COMMON) $(PKG)

bench-scale:
	go test -bench='BenchmarkStack/[^/]+/MultiThread' -cpu=1,2,4,8 $(COMMON) $(PKG)

bench-mpmc-queue:
	go test -bench='BenchmarkMPMCQueue' $(COMMON) $(PKG)

bench-mpsc-queue:
	go test -bench='BenchmarkMPSCQueue' $(COMMON) $(PKG)

bench-cpu:
	go test -bench=. -cpuprofile=cpu.out $(COMMON) $(PKG)
	@echo ""
	@echo "CPU profile written to cpu.out. Inspect with: make cpu-prof"

bench-mem:
	go test -bench=. -memprofile=mem.out $(COMMON) $(PKG)
	@echo ""
	@echo "Memory profile written to mem.out. Inspect with: make mem-prof"

bench-full:
	go test -bench=. -cpuprofile=cpu.out -memprofile=mem.out $(COMMON) $(PKG)
	@echo ""
	@echo "Profiles written. Inspect with: make cpu-prof / make mem-prof"

cpu-prof:
	go tool pprof -http=:$(PPROF_PORT) cpu.out

mem-prof:
	go tool pprof -http=:$(PPROF_PORT) mem.out

clean:
	rm -f cpu.out mem.out experiment.test
