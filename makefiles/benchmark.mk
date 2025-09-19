####################
### Benchmarking ###
####################

.PHONY: benchmark_all
benchmark_all: ## Run all benchmarks
	go test -bench=. -benchmem -run=^$$ ./...

.PHONY: benchmark_signer
benchmark_signer: ## Run signer benchmarks
	go test -bench=. -benchmem -run=^$$ -benchtime=10s

.PHONY: benchmark_compare
benchmark_compare: ## Run benchmarks and save results for comparison (saves to bench_new.txt)
	go test -bench=. -benchmem -run=^$$ ./... | tee bench_new.txt

.PHONY: benchmark_profile
benchmark_profile: ## Run benchmarks with CPU profiling (generates cpu.prof)
	go test -bench=. -benchmem -run=^$$ -cpuprofile=cpu.prof

.PHONY: benchmark_memory
benchmark_memory: ## Run benchmarks with memory profiling (generates mem.prof)
	go test -bench=. -benchmem -run=^$$ -memprofile=mem.prof

.PHONY: benchmark_secp256k1
benchmark_secp256k1: ## Compare different secp256k1 library implementations
	@echo "Benchmarking secp256k1 implementations (CosmosSDK vs BTCSuite vs Decred vs Ethereum)..."
	go test -bench="BenchmarkKeyGeneration|BenchmarkSigning|BenchmarkVerification" -benchmem -run=^$$ -benchtime=3s

.PHONY: benchmark_secp256k1_report
benchmark_secp256k1_report: ## Compare secp256k1 implementations with formatted report
	@echo "🔬 Benchmarking secp256k1 implementations..."
	@echo "=================================================================="
	@timeout 60s go test -bench="BenchmarkKeyGeneration|BenchmarkSigning|BenchmarkVerification" -benchmem -run=^$$ -benchtime=3s 2>/dev/null | python3 format_benchmark.py || (echo "⚠️  Benchmark timed out or failed. Trying without Ethereum library..." && make benchmark_secp256k1_report_no_cgo)
	@echo "=================================================================="
	@echo "💡 Key Insights:"
	@echo "   🥇 = Fastest    🥈 = Second fastest    🥉 = Third fastest"
	@echo ""
	@echo "   • Ethereum (libsecp256k1) is fastest but requires CGO"
	@echo "   • Decred offers best CGO-free performance"
	@echo "   • CosmosSDK has most memory allocations"
	@echo "   • BTCSuite does extensive validation during key generation"
	@echo "=================================================================="

.PHONY: benchmark_secp256k1_report_fast
benchmark_secp256k1_report_fast: ## Quick secp256k1 comparison (1s benchtime)
	@echo "🔬 Quick secp256k1 benchmark (1s each)..."
	@echo "=================================================================="
	@timeout 30s go test -bench="BenchmarkKeyGeneration|BenchmarkSigning|BenchmarkVerification" -benchmem -run=^$$ -benchtime=1s 2>/dev/null | python3 format_benchmark.py || (echo "⚠️  Benchmark timed out. Trying CGO-free only..." && make benchmark_secp256k1_report_no_cgo_fast)
	@echo "=================================================================="
	@echo "💡 This was a quick benchmark. Use 'make benchmark_secp256k1_report' for full results."
	@echo "=================================================================="

.PHONY: benchmark_secp256k1_report_no_cgo
benchmark_secp256k1_report_no_cgo: ## Compare CGO-free secp256k1 implementations only
	@echo "🔬 Benchmarking CGO-free secp256k1 implementations..."
	@echo "=================================================================="
	@CGO_ENABLED=0 go test -bench="BenchmarkKeyGenerationNoCgo|BenchmarkSigningNoCgo|BenchmarkVerificationNoCgo" -benchmem -run=^$$ -benchtime=3s 2>/dev/null | python3 format_benchmark.py
	@echo "=================================================================="
	@echo "💡 CGO-free comparison only (Ethereum libsecp256k1 excluded)"
	@echo "=================================================================="

.PHONY: benchmark_secp256k1_report_no_cgo_fast
benchmark_secp256k1_report_no_cgo_fast: ## Quick CGO-free secp256k1 comparison
	@echo "🔬 Quick CGO-free secp256k1 benchmark..."
	@echo "=================================================================="
	@CGO_ENABLED=0 go test -bench="BenchmarkKeyGenerationNoCgo|BenchmarkSigningNoCgo|BenchmarkVerificationNoCgo" -benchmem -run=^$$ -benchtime=1s 2>/dev/null | python3 format_benchmark.py
	@echo "=================================================================="
	@echo "💡 Quick CGO-free comparison (Ethereum libsecp256k1 excluded)"
	@echo "=================================================================="