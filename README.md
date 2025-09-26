# ShannonSDK <!-- omit in toc -->

ShannonSDK is a Go-based toolkit for interacting with the POKT Network, designed for developers building `Gateway`s and sovereign `Application`s.

## Table of Contents <!-- omit in toc -->

- [TODO_TECHDEBT: Updating the ShannonSDK](#todo_techdebt-updating-the-shannonsdk)
- [Installation](#installation)
- [Quick Start](#quick-start)
  - [Complete working integration example](#complete-working-integration-example)
  - [Relay Request Workflow](#relay-request-workflow)
  - [Example Code](#example-code)
- [Shannon SDK Usage](#shannon-sdk-usage)
  - [Shannon SDK Core Components](#shannon-sdk-core-components)
- [Crypto Backends](#crypto-backends)
  - [Comparison](#comparison)
  - [Benchmarking Crypto Backends](#benchmarking-crypto-backends)

## TODO_TECHDEBT: Updating the ShannonSDK

As of 09/2025, the two primary repositories dependant on ShannonSDK are:

- [Grove's Path](https://github.com/buildwithgrove/path)
- [Pocket's Poktroll](https://github.com/pokt-network/poktroll)

Since the protobufs are shared between the `poktroll` and `shannon-sdk` repositories,
but are housed in the `poktroll` repository, [poktroll/go.mod] has the following piece of techdebt:

```go
// TODO_TECHDEBT: Whenever we update a protobuf in the `poktroll` repo, we need to:
// 1. Merge in the update PR (and it's generated outputs) into `poktroll` main.
// 2. Update the `poktroll` sha in the `shannon-sdk` to reflect the new dependency.
// 3. Update the `shannon-sdk` sha in the `poktroll` repo (here).
// This is creating a circular dependency whereby exporting the protobufs into a separate
// repo is the first obvious idea, but has to be carefully considered, automated, and is not
// a hard blocker.
github.com/pokt-network/shannon-sdk v0.0.0-20250603210336-969a825fddd5
```

To update the `poktroll` repository in the `shannon-sdk` repository, simply run:

```bash
git checkout -b bump_poktroll_version
# Optional: go clean -v -modcache
go get github.com/pokt-network/poktroll@main
make proto_regen
make go_lint
go mod tidy
make test_all
git commit -am "[Poktroll] update poktroll go dependency in the shannon-sdk"
git push
# Merge the PR after the CI is green
```

## Installation

```bash
go get github.com/pokt-network/shannon-sdk
```

## Quick Start

### Complete working integration example

A complete and working example of how to use the ShannonSDK can be found in `PATH`'s
implementation of the `signer`. See [signer.go](https://github.com/buildwithgrove/path/blob/53d0f84cc0321c25d1e28b2ffb9b70714918870b/protocol/shannon/signer.go#L9).

### Relay Request Workflow

The standard workflow for using ShannonSDK:

1. Get the current block height
2. Get the session corresponding to the block height
3. Select a supplier endpoint from the session
4. Build a relay request
5. Sign the relay request
6. Send the relay request to the endpoint
7. Validate the received relay response

### Example Code

<details>
<summary>Example Code</summary>

```go
package main

import (
  "bytes"
  "context"
  "fmt"
  "io"
  "net/http"
  "net/url"

  sdk "github.com/pokt-network/shannon-sdk"
  grpc "github.com/cosmos/gogoproto/grpc"
)

func main() {
  // 1. Create a connection to the POKT full node
  // Replace with your POKT node URL
  var grpcConn grpc.ClientConn
  // Initialize your gRPC connection here...

  // 2. Get the latest block height
  blockClient := sdk.BlockClient{
    PoktNodeStatusFetcher: sdk.NewPoktNodeStatusFetcher("http://pokt-node-url"),
  }
  blockHeight, err := blockClient.LatestBlockHeight(context.Background())
  if err != nil {
    fmt.Printf("Error fetching block height: %v\n", err)
    return
  }

  // 3. Get the current session
  sessionClient := sdk.SessionClient{
    PoktNodeSessionFetcher: sdk.NewPoktNodeSessionFetcher(grpcConn),
  }
  session, err := sessionClient.GetSession(
    context.Background(),
    "YOUR_APP_ADDRESS",
    "SERVICE_ID",
    blockHeight,
  )
  if err != nil {
    fmt.Printf("Error fetching session: %v\n", err)
    return
  }

  // 4. Select an endpoint from the session
  sessionFilter := sdk.SessionFilter{
    Session:         session,
    EndpointFilters: []sdk.EndpointFilter{},
  }
  endpoints, err := sessionFilter.FilteredEndpoints()
  if err != nil {
    fmt.Printf("Error filtering endpoints: %v\n", err)
    return
  }
  if len(endpoints) == 0 {
    fmt.Println("No endpoints available")
    return
  }

  // 5. Build a relay request
  relayReq, err := sdk.BuildRelayRequest(endpoints[0], []byte("your-relay-payload"))
  if err != nil {
    fmt.Printf("Error building relay request: %v\n", err)
    return
  }

  // 6. Create an account client for fetching public keys
  accountClient := sdk.AccountClient{
    PoktNodeAccountFetcher: sdk.NewPoktNodeAccountFetcher(grpcConn),
  }

  // 7. Create an application client to get application details
  appClient := sdk.ApplicationClient{
    QueryClient: nil, // Initialize with your query client
  }
  app, err := appClient.GetApplication(context.Background(), "YOUR_APP_ADDRESS")
  if err != nil {
    fmt.Printf("Error fetching application: %v\n", err)
    return
  }

  // 8. Create an application ring for signing
  ring := sdk.ApplicationRing{
    Application:      app,
    PublicKeyFetcher: &accountClient,
  }

  // 9. Sign the relay request
  signer, err := sdk.NewSignerFromHex("YOUR_PRIVATE_KEY")
  if err != nil {
    fmt.Printf("Error creating signer: %v\n", err)
    return
  }
  signedRelayReq, err := signer.Sign(context.Background(), relayReq, &ring)
  if err != nil {
    fmt.Printf("Error signing relay request: %v\n", err)
    return
  }

  // 10. Send the relay request to the endpoint
  relayReqBz, err := signedRelayReq.Marshal()
  if err != nil {
    fmt.Printf("Error marshaling relay request: %v\n", err)
    return
  }

  reqUrl, err := url.Parse(endpoints[0].Endpoint().Url)
  if err != nil {
    fmt.Printf("Error parsing endpoint URL: %v\n", err)
    return
  }

  httpReq := &http.Request{
    Method: http.MethodPost,
    URL:    reqUrl,
    Body:   io.NopCloser(bytes.NewReader(relayReqBz)),
  }

  // Send the request
  httpResp, err := http.DefaultClient.Do(httpReq)
  if err != nil {
    fmt.Printf("Error sending relay request: %v\n", err)
    return
  }
  defer httpResp.Body.Close()

  // 11. Read the response
  respBz, err := io.ReadAll(httpResp.Body)
  if err != nil {
    fmt.Printf("Error reading response: %v\n", err)
    return
  }

  // 12. Validate the relay response
  validatedResp, err := sdk.ValidateRelayResponse(
    context.Background(),
    sdk.SupplierAddress(signedRelayReq.Meta.SupplierOperatorAddress),
    respBz,
    &accountClient,
  )
  if err != nil {
    fmt.Printf("Error validating response: %v\n", err)
    return
  }

  fmt.Printf("Relay successful: %v\n", validatedResp.Result)
}
```

</details>

## Shannon SDK Usage

ShannonSDK streamlines:

- Constructing POKT-compatible `RelayRequest`s
- Verifying `RelayResponse`s from `RelayMiner`s
- Building `Suppliers` co-processors
- Abstracting protocol-specific details

The SDK provides an intuitive interface to manage `Sessions`, `Applications`, and `RelayRequests`.

### Shannon SDK Core Components

| Component          | Description                                    |
| ------------------ | ---------------------------------------------- |
| Account Client     | Query and manage account information           |
| Application Client | Handle application operations and queries      |
| ApplicationRing    | Manage gateway delegations and ring signatures |
| Block Client       | Retrieve blockchain information                |
| Session Client     | Manage session operations                      |
| Shared Client      | Interops with the onchain shared module        |
| Session Filter     | Select supplier endpoints based on criteria    |
| Signer             | Sign relay requests securely                   |
| Relayer            | Build and validate relay requests/responses    |

## Crypto Backends

⚠️The crypto backend is a BUILD-TIME configuration ⚠️

Shannon SDK uses ring signatures implemented in [ring-go](https://github.com/pokt-network/ring-go), which in turn
uses [go-dleq](https://github.com/pokt-network/go-dleq) and a `secp256k1` library. Those repos select fast vs portable
implementations via build tags. This SDK follows their selection — there’s nothing to toggle at runtime.

### Comparison

| Backend                | Build Tags / Env                             | Dependencies                  | Performance                              | Portability                |
| ---------------------- | -------------------------------------------- | ----------------------------- | ---------------------------------------- | -------------------------- |
| **Portable (Pure Go)** | `CGO_ENABLED=0` (default)                    | None                          | Excellent (Decred secp256k1 via ring-go) | Runs anywhere              |
| **Ethereum secp256k1** | `CGO_ENABLED=1` + `-tags=ethereum_secp256k1` | `gcc`, `libsecp256k1` headers | Faster signing/verification              | Requires CGO + system libs |

To use the faster backend locally (copy/paste):

```bash
# Build
CGO_ENABLED=1 go build -tags=ethereum_secp256k1 ./...

# Run tests/benchmarks
CGO_ENABLED=1 go test -tags=ethereum_secp256k1 -bench=. -benchmem ./...
```

NOTE: You may need to add other flags to enable CGO, such as `-ldflags "-linkshared"` depending on your system.

### Benchmarking Crypto Backends

To see the benchmarks, run this command:

```bash
make benchmark_report
```

<details>
<summary>Benchmark Output</summary>

```bash
📊 Shannon SDK Benchmarks (portable vs ethereum)
🔬 Shannon SDK Benchmark Comparison
====================================

📊 Testing Portable Backend (Pure Go)

📊 Testing Ethereum Backend (libsecp256k1)

=== SDK Performance Comparison ===

## Benchmark Portable Ethereum Improvement Δ Time Δ %

BenchmarkSign 1.7 ms 766 μs 🚀 2.2x faster - 915 μs -54.4%
BenchmarkSignWithCachedPrivateKey 1.8 ms 745 μs 🚀 2.4x faster - 1.0 ms -58.2%
BenchmarkSignLargePayload 1.8 ms 776 μs 🚀 2.3x faster - 980 μs -55.8%
BenchmarkSerializeSignature 42 μs 399 ns 🚀 104.6x faster - 41 μs -99.0%
BenchmarkSignCore 1.5 ms 631 μs 🚀 2.4x faster - 890 μs -58.5%
BenchmarkSignReuseRing 1.7 ms 784 μs 🚀 2.1x faster - 888 μs -53.1%
BenchmarkVerifyRingSignature 1.0 ms 458 μs 🚀 2.2x faster - 553 μs -54.7%
BenchmarkDeserializeAndVerifyRingSignature 1.1 ms 473 μs 🚀 2.3x faster - 597 μs -55.8%
BenchmarkVerifyRingSignature5 2.5 ms 1.1 ms 🚀 2.3x faster - 1.4 ms -55.7%
BenchmarkVerifyRingSignature10 5.1 ms 2.1 ms 🚀 2.4x faster - 3.0 ms -58.6%
BenchmarkVerifyRingSignature20 9.9 ms 4.2 ms 🚀 2.4x faster - 5.7 ms -57.8%
BenchmarkVerifyRingSignature40 20.6 ms 8.2 ms 🚀 2.5x faster - 12.4 ms -60.2%

Summary
Benchmarks: 12
Avg speedup: 10.83x
Median speedup: 2.32x
Avg Δ%%: -60.1%

=== Memory Usage Comparison ===

## Benchmark Port B/op Port Allocs Eth B/op Eth Allocs Δ B/op Δ Allocs Δ% B Δ% Allocs

BenchmarkSign 9073 150 43108 1030 +34035 +880 +375.1% +586.7%
BenchmarkSignWithCachedPrivateKey 8425 137 39030 898 +30605 +761 +363.3% +555.5%
BenchmarkSignLargePayload 19827 150 54090 1058 +34263 +908 +172.8% +605.3%
BenchmarkSerializeSignature 968 11 968 11 +0 +0 +0.0% +0.0%
BenchmarkSignCore 5977 95 23070 376 +17093 +281 +286.0% +295.8%
BenchmarkSignReuseRing 9073 150 43448 1072 +34375 +922 +378.9% +614.7%
BenchmarkVerifyRingSignature 3392 53 19172 299 +15780 +246 +465.2% +464.2%
BenchmarkDeserializeAndVerifyRingSignature 4512 77 30335 676 +25823 +599 +572.3% +777.9%
BenchmarkVerifyRingSignature5 8480 131 47730 744 +39250 +613 +462.9% +467.9%
BenchmarkVerifyRingSignature10 16961 261 95588 1489 +78627 +1228 +463.6% +470.5%
BenchmarkVerifyRingSignature20 33920 521 191145 2976 +157225 +2455 +463.5% +471.2%
BenchmarkVerifyRingSignature40 67904 1041 382960 5959 +315056 +4918 +464.0% +472.4%
```

</details>
