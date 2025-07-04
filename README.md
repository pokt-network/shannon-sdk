# ShannonSDK <!-- omit in toc -->

ShannonSDK is a Go-based toolkit for interacting with the POKT Network, designed for developers building `Gateway`s and sovereign `Application`s.

## Table of Contents <!-- omit in toc -->

- [TechDebt: Updating the ShannonSDK](#techdebt-updating-the-shannonsdk)
- [Overview](#overview)
- [Key Features](#key-features)
- [Complete working integration example](#complete-working-integration-example)
- [Core Components](#core-components)
- [Installation](#installation)
- [Quick Start](#quick-start)
  - [Relay Request Workflow](#relay-request-workflow)
  - [Example Code](#example-code)
- [API Reference](#api-reference)

## TechDebt: Updating the ShannonSDK

As of 07/2025, the two primary repositories dependant on ShannonSDK are:

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

## Overview

ShannonSDK streamlines:

- Constructing POKT-compatible `RelayRequest`s
- Verifying `RelayResponse`s from `RelayMiner`s
- Building `Suppliers` co-processors
- Abstracting protocol-specific details

The SDK provides an intuitive interface to manage `Sessions`, `Applications`, and `RelayRequests`.

## Key Features

- Secure request signing using ring signatures
- Endpoint selection based on customizable filters
- Robust error handling and validation
- Protocol-specific serialization for HTTP requests/responses
- Go-idiomatic API design

## Complete working integration example

A complete and working example of how to use the ShannonSDK can be found in `PATH`'s
implementation of the `signer`. See [signer.go](https://github.com/buildwithgrove/path/blob/53d0f84cc0321c25d1e28b2ffb9b70714918870b/protocol/shannon/signer.go#L9).

## Core Components

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

## Installation

```bash
go get github.com/pokt-network/shannon-sdk
```

## Quick Start

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
  signer := sdk.Signer{PrivateKeyHex: "YOUR_PRIVATE_KEY"}
  signedRelayReq, err := signer.Sign(context.Background(), relayReq, ring)
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

## API Reference

| Component             | Description                                       | Key Method                                                                         |
| --------------------- | ------------------------------------------------- | ---------------------------------------------------------------------------------- |
| **AccountClient**     | Fetches account information including public keys | `GetPubKeyFromAddress()`                                                           |
| **ApplicationClient** | Manages application operations and queries        | `GetApplication()`, `GetAllApplications()`, `GetApplicationsDelegatingToGateway()` |
| **ApplicationRing**   | Handles gateway delegations and ring signatures   | `GetRing()`                                                                        |
| **BlockClient**       | Retrieves blockchain information                  | `LatestBlockHeight()`                                                              |
| **SessionClient**     | Manages session operations                        | `GetSession()`                                                                     |
| **SharedClient**      | Interops with the onchain shared module           | `GetParams()`                                                                      |
| **SessionFilter**     | Filters and selects supplier endpoints            | `AllEndpoints()`, `FilteredEndpoints()`                                            |
| **Signer**            | Signs relay requests with private keys            | `Sign()`                                                                           |
| **Relayer Functions** | Builds and validates relay requests/responses     | `BuildRelayRequest()`, `ValidateRelayResponse()`                                   |
