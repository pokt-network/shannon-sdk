# Shannon SDK Crypto Backends <!-- omit in toc -->

The Shannon SDK supports multiple secp256k1 crypto backends that can be selected at build time for optimal performance vs portability trade-offs.

## Table of Contents <!-- omit in toc -->

- [Quick Start](#quick-start)
- [Available Backends](#available-backends)
- [Performance Comparison](#performance-comparison)
- [Build Options](#build-options)
- [Usage Examples](#usage-examples)
- [Backend Selection Guide](#backend-selection-guide)
- [Migration Guide](#migration-guide)
- [Benchmarking](#benchmarking)
- [Troubleshooting](#troubleshooting)

## Quick Start

```bash
# Fast build (requires CGO, ~50% faster)
make build_fast

# Portable build (pure Go, works everywhere) 
make build_portable

# Auto-select best for your platform
make build_auto
```

```go
// Application code remains the same regardless of backend
signer, err := sdk.NewSignerFromHex("your-private-key-hex")
if err != nil {
    log.Fatal(err)
}

// See which backend is being used
sdk.LogBackendInfo(signer.GetCryptoSigner())
```

## Available Backends

### 🥇 Ethereum Backend (Fastest)

**Build Tag:** `ethereum_secp256k1`  
**Requirements:** CGO, libsecp256k1  
**Performance:** ~20.5μs signing, ~23.8μs verification

Uses Ethereum's wrapper around Bitcoin Core's libsecp256k1 C library. This is the same cryptographic implementation used by Bitcoin and Ethereum networks.

**Pros:**
- 🚀 Maximum performance (~50% faster signing, ~80% faster verification)
- 📦 Zero memory allocations during verification
- ✅ Battle-tested in production (Bitcoin Core since 2015)
- 🔒 Hardware-optimized assembly implementations

**Cons:**
- ⚙️  Requires CGO and C compiler
- 📋 Platform-specific build dependencies
- 🏗️  More complex deployment requirements

### 🥈 Decred Backend (Default)

**Build Tag:** None (default)  
**Requirements:** Pure Go  
**Performance:** ~37.6μs signing, ~129.8μs verification

Pure Go implementation by the Decred project, offering excellent performance without any C dependencies.

**Pros:**
- 🌍 Maximum portability (pure Go)
- 📦 Simple deployment (single binary)
- 🔧 Easy cross-compilation  
- ⚡ Still excellent performance (2nd fastest)
- 🛡️  No CGO security surface

**Cons:**
- 🐌 ~45% slower than Ethereum backend
- 💾 More memory allocations during crypto operations

## Performance Comparison

Based on benchmark results (Apple M1 Max):

| Backend | Signing Speed | Verification Speed | Memory/Op | Allocations | CGO Required |
|---------|---------------|-------------------|-----------|-------------|--------------|
| Ethereum | 20.5μs 🥇 | 23.8μs 🥇 | 164B | 3 | ✅ |
| Decred | 37.6μs 🥈 | 129.8μs 🥈 | 1.6KB | 32 | ❌ |

**Performance Impact:**
- **High-throughput applications:** Ethereum backend provides significant benefits
- **Standard applications:** Both backends offer excellent performance  
- **Resource-constrained environments:** Decred backend uses less memory

## Build Options

### Make Targets

```bash
# Development builds
make build_fast           # Ethereum backend (fastest)
make build_portable       # Decred backend (portable)  
make build_auto           # Auto-select best option
make build_all            # Build both variants

# Benchmarking
make benchmark_secp256k1_report_fast    # Quick comparison
make benchmark_secp256k1_report         # Full comparison
make benchmark_secp256k1_report_no_cgo  # CGO-free only

# Cleanup
make clean_builds         # Remove built binaries
```

### Manual Builds

```bash
# Ethereum backend
go build -tags="ethereum_secp256k1" -o shannon-sdk-fast

# Decred backend (default)
go build -o shannon-sdk-portable

# Decred backend (explicit CGO disable)
CGO_ENABLED=0 go build -o shannon-sdk-portable
```

### Docker Builds

```dockerfile
# Multi-stage build for both variants
FROM golang:1.21-alpine AS builder

# Install CGO dependencies
RUN apk add --no-cache gcc musl-dev

WORKDIR /app
COPY . .

# Build fast version
RUN go build -tags="ethereum_secp256k1" -o shannon-sdk-fast

# Build portable version  
RUN CGO_ENABLED=0 go build -o shannon-sdk-portable

# Final image
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/

# Choose which binary to include
COPY --from=builder /app/shannon-sdk-fast .
# OR: COPY --from=builder /app/shannon-sdk-portable .
```

## Usage Examples

### Basic Usage

```go
package main

import (
    "context"
    "log"
    
    sdk "github.com/pokt-network/shannon-sdk"
)

func main() {
    // Create signer (backend selected at build time)
    signer, err := sdk.NewSignerFromHex("1234567890abcdef...")
    if err != nil {
        log.Fatal("Failed to create signer:", err)
    }
    
    // Log backend information
    info := signer.GetBackendInfo()
    log.Printf("Using %s backend: %s", info.Name, info.PerformanceLevel)
    
    // Use signer normally - API is identical regardless of backend
    signedRequest, err := signer.Sign(ctx, relayRequest, appRing)
    if err != nil {
        log.Fatal("Signing failed:", err)
    }
}
```

### Advanced Usage

```go
// Check available backends
backends := sdk.GetAvailableBackends()
for _, backend := range backends {
    fmt.Printf("Available: %s\n", backend)
}

// Access underlying crypto signer
cryptoSigner := signer.GetCryptoSigner()
backendInfo := cryptoSigner.GetBackendInfo()

// Custom private key operations
privateKey, err := cryptoSigner.DecodePrivateKey(keyHex)
if err != nil {
    return err
}

hash := sha256.Sum256([]byte("test message"))
signature, err := privateKey.Sign(hash[:])
if err != nil {
    return err  
}
```

### Testing Both Backends

```go
// Test with build tags
// go test -tags="ethereum_secp256k1" ./...  # Test Ethereum
// go test ./...                             # Test Decred

func TestSigningCompatibility(t *testing.T) {
    signer, err := sdk.NewSignerFromHex(testPrivateKey)
    require.NoError(t, err)
    
    // Test should pass with either backend
    result, err := signer.Sign(ctx, testRequest, testRing)
    require.NoError(t, err)
    require.NotNil(t, result)
    
    t.Logf("Tested with %s backend", signer.GetBackendInfo().Name)
}
```

## Backend Selection Guide

### Choose Ethereum Backend When:

✅ **Maximum performance is critical**  
✅ **High transaction throughput applications**  
✅ **CGO dependencies are acceptable**  
✅ **Deployment environment is controlled**  
✅ **Building for specific platforms**

### Choose Decred Backend When:

✅ **Simple deployment is priority**  
✅ **Cross-platform compatibility needed**  
✅ **CGO dependencies are problematic**  
✅ **Static binary distribution required**  
✅ **Performance requirements are moderate**

### Platform Considerations

| Platform | Recommended | Notes |
|----------|-------------|--------|
| **Linux Servers** | Ethereum | Easy CGO setup, performance critical |
| **Docker Containers** | Ethereum | Controlled environment |
| **Cloud Functions** | Decred | Cold start optimization, simple deployment |
| **Edge/IoT Devices** | Decred | Resource constraints, simple deployment |
| **CI/CD Pipelines** | Decred | Faster builds, fewer dependencies |
| **Cross-compilation** | Decred | CGO complicates cross-compilation |

## Migration Guide

### From Original Signer

The new crypto backends maintain full API compatibility:

```go
// Before (still works)
signer := &sdk.Signer{PrivateKeyHex: "..."}
result, err := signer.Sign(ctx, request, ring)

// After (recommended)
signer, err := sdk.NewSignerFromHex("...")
if err != nil {
    return err
}
result, err := signer.Sign(ctx, request, ring)
```

### Gradual Migration

1. **Phase 1:** Use new `NewSignerFromHex()` constructor
2. **Phase 2:** Add backend logging/monitoring
3. **Phase 3:** Benchmark your specific workload
4. **Phase 4:** Choose optimal backend for production

## Benchmarking

### Run Benchmarks

```bash
# Quick comparison (30 seconds)
make benchmark_secp256k1_report_fast

# Full comparison (3 minutes)
make benchmark_secp256k1_report

# CGO-free only
make benchmark_secp256k1_report_no_cgo_fast
```

### Benchmark Your Workload

```go
func BenchmarkYourWorkload(b *testing.B) {
    signer, _ := sdk.NewSignerFromHex(testKey)
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := signer.Sign(ctx, relayRequest, appRing)
        if err != nil {
            b.Fatal(err)
        }
    }
}
```

Run with both backends:
```bash
go test -bench=BenchmarkYourWorkload -tags="ethereum_secp256k1"  # Ethereum
go test -bench=BenchmarkYourWorkload                             # Decred
```

## Troubleshooting

### Build Issues

**CGO errors with Ethereum backend:**
```bash
# Install CGO dependencies
# Ubuntu/Debian:
sudo apt-get install build-essential

# macOS:
xcode-select --install

# Alpine:
apk add gcc musl-dev
```

**"undefined: ethsecp256k1" errors:**
- Missing `ethereum_secp256k1` build tag
- CGO disabled when Ethereum backend expected

**Solution:**
```bash
# Ensure CGO is enabled and build tag is set
CGO_ENABLED=1 go build -tags="ethereum_secp256k1"
```

### Runtime Issues

**"unknown backend" errors:**
- Signer not properly initialized
- Call `NewSignerFromHex()` instead of direct struct creation

**Performance not as expected:**
- Verify correct backend with `signer.GetBackendInfo()`
- Run benchmarks to confirm setup: `make benchmark_secp256k1_report_fast`

### Testing Issues

**Tests fail with specific backend:**
```bash
# Test both backends
go test -tags="ethereum_secp256k1" ./...  # Test Ethereum
go test ./...                             # Test Decred
```

**Compatibility issues:**
- All backends should produce identical results
- Run compatibility tests: `go test -run TestCompatibility`

### Debug Information

```go
// Add debug logging
signer, err := sdk.NewSignerFromHex(privateKey)
if err != nil {
    log.Fatal(err)
}

info := signer.GetBackendInfo()
log.Printf("Backend: %s", info.Name)
log.Printf("CGO Required: %v", info.CGORequired)
log.Printf("Performance: %s", info.PerformanceLevel)
log.Printf("Signing Speed: %.1fμs", info.SigningSpeedUs)
log.Printf("Verification Speed: %.1fμs", info.VerificationSpeedUs)
```

---

## Contributing

When adding new crypto backends:

1. Implement the `CryptoSigner` interface
2. Use appropriate build tags
3. Add performance benchmarks  
4. Update documentation
5. Test compatibility with existing backends