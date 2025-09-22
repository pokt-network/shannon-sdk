####################
### Benchmarking ###
####################

.PHONY: benchmark_all
benchmark_all: ## Run all benchmarks (tests both Decred and Ethereum backends)
	@echo "🔬 Running benchmarks with Decred backend (Pure Go)..."
	@echo "=================================================="
	go test -v -bench=. -benchmem -run=^$$ ./...
	@echo ""
	@echo "🔬 Running benchmarks with Ethereum backend (CGO + libsecp256k1)..."
	@echo "=================================================================="
	go test -tags=ethereum_secp256k1 -v -bench=. -benchmem -run=^$$ ./...

.PHONY: benchmark_report
benchmark_report: ## Compare secp256k1 implementations with formatted report
	@echo "🔬 Benchmarking secp256k1 implementations..."
	@echo "=================================================================="
	@timeout 60s \
		go test ./crypto \
			-bench="BenchmarkKeyGeneration|BenchmarkSigning|BenchmarkVerification" \
			-benchmem \
			-run=^$$ \
			-benchtime=3s \
			2>/dev/null | \
		python3 format_benchmark.py \
		|| ( \
			echo "⚠️  Benchmark timed out or failed. Trying without Ethernet library..." && \
			CGO_ENABLED=0 go test ./crypto \
				-bench="BenchmarkKeyGenerationNoCgo|BenchmarkSigningNoCgo|BenchmarkVerificationNoCgo" \
				-benchmem \
				-run=^$$ \
				-benchtime=3s \
				2>/dev/null | \
			python3 format_benchmark.py \
		)
	@echo "=================================================================="
	@echo "💡 Key Insights:"
	@echo "   🥇 = Fastest    🥈 = Second fastest    🥉 = Third fastest"
	@echo ""
	@echo "   • Ethernet (libsecp256k1) is fastest but requires CGO"
	@echo "   • Decred offers best CGO-free performance"
	@echo "   • CosmosSDK has most memory allocations"
	@echo "   • BTCSuite does extensive validation during key generation"
	@echo "=================================================================="