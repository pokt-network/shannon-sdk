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
benchmark_report: ## Report SDK-level ring-go performance (Decred vs Ethereum backends)
	@echo "\n📊 SDK-LEVEL PERFORMANCE (Ring Signatures via ring-go)"
	@echo "-----------------------------------------------------------"
	@echo "\033[1mBackend         Time/op      Memory/op    Allocs/op    Iterations     \033[0m"
	@echo "-------         --------     ---------    ---------    ----------     "
	@printf "%-15s " "Decred"; go test github.com/pokt-network/ring-go -bench=BenchmarkSign2_Secp256k1 -benchmem -run=^$$ -benchtime=1s 2>/dev/null | grep "ns/op" | head -1 | awk '{printf "\033[32m%-12s\033[0m \033[34m%-12s\033[0m \033[33m%-12s\033[0m \033[36m%-15s\033[0m", ($$3/1000000 >= 1 ? sprintf("%.1f ms", $$3/1000000) : sprintf("%.0f μs", $$3/1000)), $$5 " " $$6, $$7, ($$2/1000000 >= 1 ? sprintf("%.1fM", $$2/1000000) : sprintf("%.1fK", $$2/1000))}'; echo ""
	@printf "%-15s " "Ethereum"; env CGO_ENABLED=1 go test github.com/pokt-network/ring-go -tags=ethereum_secp256k1 -bench=BenchmarkSign2_Secp256k1 -benchmem -run=^$$ -benchtime=1s 2>/dev/null | grep "ns/op" | head -1 | awk '{printf "\033[32m%-12s\033[0m \033[34m%-12s\033[0m \033[33m%-12s\033[0m \033[36m%-15s\033[0m", ($$3/1000000 >= 1 ? sprintf("%.1f ms", $$3/1000000) : sprintf("%.0f μs", $$3/1000)), $$5 " " $$6, $$7, ($$2/1000000 >= 1 ? sprintf("%.1fM", $$2/1000000) : sprintf("%.1fK", $$2/1000))}'; echo ""
	@echo ""
	@echo "=================================================================="
