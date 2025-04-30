# ShannonSDK

ShannonSDK is a Go-based toolkit for interacting with the POKT Network, designed for developers building `Gateway`s and sovereign `Application`s.

## Table of Contents

- [ShannonSDK](#shannonsdk)
  - [Table of Contents](#table-of-contents)
  - [Overview](#overview)
  - [Key Features](#key-features)
  - [Core Components](#core-components)
  - [Installation](#installation)
  - [Quick Start](#quick-start)
    - [Relay Request Workflow](#relay-request-workflow)
    - [Example Code](#example-code)
  - [API Reference](#api-reference)
    - [Account Client](#account-client)
    - [Application Client](#application-client)
    - [ApplicationRing](#applicationring)
    - [Block Client](#block-client)
    - [Session Client](#session-client)
    - [Session Filter](#session-filter)
    - [Signer](#signer)
    - [Relayer](#relayer)
  - [Project Structure](#project-structure)

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

## Core Components

| Component          | Description                                    |
| ------------------ | ---------------------------------------------- |
| Account Client     | Query and manage account information           |
| Application Client | Handle application operations and queries      |
| ApplicationRing    | Manage gateway delegations and ring signatures |
| Block Client       | Retrieve blockchain information                |
| Session Client     | Manage session operations                      |
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

## API Reference

### Account Client

```go
type AccountClient struct {
    PoktNodeAccountFetcher
}

// GetPubKeyFromAddress returns the public key for a given address
func (ac *AccountClient) GetPubKeyFromAddress(
    ctx context.Context,
    address string,
) (pubKey cryptotypes.PubKey, err error)
```

### Application Client

```go
type ApplicationClient struct {
    QueryClient
}

// GetApplication retrieves a specific application by address
func (ac *ApplicationClient) GetApplication(
    ctx context.Context,
    appAddress string,
) (types.Application, error)

// GetAllApplications returns all applications in the network
func (ac *ApplicationClient) GetAllApplications(
    ctx context.Context,
) ([]types.Application, error)

// GetApplicationsDelegatingToGateway returns applications delegating to a gateway
func (ac *ApplicationClient) GetApplicationsDelegatingToGateway(
    ctx context.Context,
    gatewayAddress string,
    sessionEndHeight uint64,
) ([]string, error)
```

### ApplicationRing

```go
type ApplicationRing struct {
    types.Application
    PublicKeyFetcher
}

// GetRing returns the ring for the application
func (a ApplicationRing) GetRing(
    ctx context.Context,
    sessionEndHeight uint64,
) (addressRing *ring.Ring, err error)
```

### Block Client

```go
type BlockClient struct {
    PoktNodeStatusFetcher
}

// LatestBlockHeight returns the height of the latest block
func (bc *BlockClient) LatestBlockHeight(ctx context.Context) (height int64, err error)
```

### Session Client

```go
type SessionClient struct {
    PoktNodeSessionFetcher
}

// GetSession returns the session for an application, service, and height
func (s *SessionClient) GetSession(
    ctx context.Context,
    appAddress string,
    serviceId string,
    height int64,
) (session *sessiontypes.Session, err error)
```

### Session Filter

```go
type SessionFilter struct {
    *sessiontypes.Session
    EndpointFilters []EndpointFilter
}

// AllEndpoints returns all endpoints in the session
func (f *SessionFilter) AllEndpoints() (map[SupplierAddress][]Endpoint, error)

// FilteredEndpoints returns endpoints that pass all filters
func (f *SessionFilter) FilteredEndpoints() ([]Endpoint, error)
```

### Signer

```go
type Signer struct {
    PrivateKeyHex string
}

// Sign signs a relay request using the application's ring
func (s *Signer) Sign(
    ctx context.Context,
    relayRequest *servicetypes.RelayRequest,
    appRing ApplicationRing,
) (*servicetypes.RelayRequest, error)
```

### Relayer

```go
// BuildRelayRequest creates a relay request from an endpoint and payload
func BuildRelayRequest(
    endpoint Endpoint,
    requestBz []byte,
) (*servicetypes.RelayRequest, error)

// ValidateRelayResponse validates a relay response signature
func ValidateRelayResponse(
    ctx context.Context,
    supplierAddress SupplierAddress,
    relayResponseBz []byte,
    publicKeyFetcher PublicKeyFetcher,
) (*servicetypes.RelayResponse, error)
```

## Project Structure

| File                  | Description                                |
| --------------------- | ------------------------------------------ |
| `account.go`          | Account client implementation              |
| `application.go`      | Application client and ring implementation |
| `block.go`            | Block client implementation                |
| `relay.go`            | Relay request/response utilities           |
| `session.go`          | Session client and filtering               |
| `signer.go`           | Request signing implementation             |
| `types/*.go`          | HTTP/RPC type definitions                  |
| `proto/types/*.proto` | Protocol buffer definitions                |
