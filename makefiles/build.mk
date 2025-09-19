#####################
### Build Targets ###
#####################

.PHONY: build_fast
build_fast: ## Build with Ethereum backend (50% faster signing, 80% faster verification, requires CGO)
	@echo "🚀 Building Shannon SDK with Ethereum secp256k1 backend..."
	@echo "   • Requires CGO and libsecp256k1"
	@echo "   • ~50% faster signing, ~80% faster verification"
	@echo "=================================================================="
	go build -tags="ethereum_secp256k1" -o shannon-sdk-fast ./cmd/...
	@echo "✅ Built: shannon-sdk-fast"

.PHONY: build_portable
build_portable: ## Build with Decred backend (pure Go, maximum portability, no CGO dependencies)
	@echo "🌍 Building Shannon SDK with Decred secp256k1 backend..."
	@echo "   • Pure Go, no CGO dependencies"
	@echo "   • Excellent performance, maximum portability"
	@echo "=================================================================="
	CGO_ENABLED=0 go build -o shannon-sdk-portable ./cmd/...
	@echo "✅ Built: shannon-sdk-portable"

.PHONY: build_auto
build_auto: ## Auto-select optimal backend (Ethereum if CGO available, otherwise Decred)
	@echo "🎯 Auto-selecting optimal crypto backend..."
	@if command -v gcc >/dev/null 2>&1 && [ "$$CGO_ENABLED" != "0" ]; then \
		echo "   • CGO available, building fast version..."; \
		$(MAKE) build_fast; \
	else \
		echo "   • No CGO or CGO disabled, building portable version..."; \
		$(MAKE) build_portable; \
	fi

.PHONY: build_all
build_all: ## Build both Ethereum (fast) and Decred (portable) versions
	@echo "🏗️  Building all Shannon SDK variants..."
	$(MAKE) build_fast
	$(MAKE) build_portable
	@echo "=================================================================="
	@echo "✅ Built all variants:"
	@echo "   • shannon-sdk-fast     (Ethereum backend)"
	@echo "   • shannon-sdk-portable (Decred backend)"
	@ls -la shannon-sdk-*

.PHONY: clean_builds
clean_builds: ## Remove all Shannon SDK built binaries
	@echo "🧹 Cleaning built binaries..."
	rm -f shannon-sdk-fast shannon-sdk-portable
	@echo "✅ Cleaned all builds"