####################
### Benchmarking ###
####################

.PHONY: benchmark_all
benchmark_all: ## Run all benchmarks (tests both Decred and Ethereum backends)
	@echo "🔬 Running LOW-LEVEL CRYPTO + SDK benchmarks with Decred backend (Pure Go)..."
	@echo "============================================================================="
	go test -v -bench=. -benchmem -run=^$$ ./...
	@echo ""
	@echo "🔬 Running LOW-LEVEL CRYPTO + SDK benchmarks with Ethereum backend (CGO + libsecp256k1)..."
	@echo "========================================================================================="
	go test -tags=ethereum_secp256k1 -v -bench=. -benchmem -run=^$$ ./...

.PHONY: benchmark_report
benchmark_report: ## Generate SDK benchmark report (portable vs ethereum backends)
	@echo "\n📊 Shannon SDK Benchmarks (portable vs ethereum)"
	@CGO_ENABLED=1 go run cmd/benchmark/main.go -report -duration=2s
