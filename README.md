# ShannonSDK <!-- omit in toc -->

ShannonSDK is a Software Development Kit designed to facilitate interaction with
the POKT Network for developers of both `Gateway`s and sovereign `Application`s.
It streamlines the process of constructing POKT compatible RelayRequests, verifying
`RelayResponses` from `RelayMiner`s, bulding `Suppliers` co-processors, and abstracting
out other protocol-specific details.

To learn more about any of the actors or components mentioned above, please refer
to [dev.poktroll.com/category/actors](https://dev.poktroll.com/category/actors).

- [Overview](#overview)
- [Key Components](#key-components)
- [Usage](#usage)
  - [Get session and endpoint selection](#get-session-and-endpoint-selection)
  - [Build and Sign Relay Requests](#build-and-sign-relay-requests)
  - [Complete working integration example](#complete-working-integration-example)
- [ShannonSDK Internals \& Design](#shannonsdk-internals--design)
  - [Code Organization](#code-organization)
  - [Interface Design](#interface-design)
  - [ShannonSDK components](#shannonsdk-components)
    - [Account Client](#account-client)
    - [Application Client](#application-client)
    - [Application Ring](#application-ring)
    - [Block Client](#block-client)
    - [Signer](#signer)
    - [Session Client](#session-client)
    - [Session Filter](#session-filter)
    - [Relayer](#relayer)

## Overview

ShannonSDK encapsulates various structures and a functions necessary for interacting
with the `Poktroll` network. It provides an intuitive interface to manage `Sessions`,
`Applications`, and `RelayRequests`.

## Key Components

The SDK consists of the following core components:

- **Account Client**: Handles account-related queries and operations.
- **Application Client**: Manages application-related operations and queries.
- **Application Ring**: Manages the list of gateways delegations from applications
and handling of ring signatures.
- **Block Client**: Fetches information about blocks on the network.
- **Signer**: Signs relay requests to ensure authenticity and integrity.
- **Session Client**: Manages session-related operations.
- **Relayer**: Building and validating RelayRequests and RelayResponses.

## Usage

For a given request, the main workflow for using the ShannonSDK is the following:
1. Get the current block height.
2. Get the session corresponding to the block height.
3. Select a supplier endpoint from those available in the session.
4. Build a relay request.
5. Sign the relay request.
6. Send the relay request to the selected endpoint.
7. Validate the received relay response.

### Get session and endpoint selection

A full example of how to get a `Session` and select a `Supplier` `Endpoint` to
send a `Relayrequest` can be found in the
[session example test](https://github.com/pokt-network/shannon-sdk/blob/main/session_test.go).

### Build and Sign Relay Requests

An example of how to build, sign, send a `RelayRequest` and validate the `RelayResponse`
can be found in the
[relay example test](https://github.com/pokt-network/shannon-sdk/blob/main/relay_test.go).

### Complete working integration example

The A complete and working example of how to use the ShannonSDK can be found in the
`AppGateServer` implementation in the [`poktroll` repository](https://github.com/pokt-network/poktroll/tree/main/pkg/appgateserver).

The initialization of the SDK can be found in the
[pkg/appgateserver/sdkadapter/sdk.go](https://github.com/pokt-network/poktroll/blob/main/pkg/appgateserver/sdkadapter/sdk.go) package of the `poktroll` repository.

## ShannonSDK Internals & Design

### Code Organization

The SDK is structured into several Go files, each dedicated to a specific aspect
of the Poktroll network:

- `account.go`: Manages account-related operations.
- `application.go`: Handles application-related queries and operations.
- `block.go`: Deals with block information retrieval.
- `relay.go`: Provides utilities for building and validating relay requests/responses.
- `session.go`: Manages session-related operations.
- `signer.go`: Handles the signing of relay requests.

### Interface Design

The SDK interfaces with functionality from other packages through Go interfaces,
following Golang's best practices. For example, the `AccountClient` struct utilizes
the `PoktNodeAccountFetcher` interface.

For more details on Golang's best practices for interfaces, refer to
[go official wiki](https://go.dev/wiki/CodeReviewComments#interfaces).

### ShannonSDK components

<!--
TODO_TECHDEBT: Find a way to integrate godoc comments as documentation for the
component methods
-->

#### Account Client

The `AccountClient` retrieves account information from the Pocket network.
It provides the following method:

- `GetPubKeyFromAddress()`: Retrieves the public key corresponding to a given address.

The `AccountClient` relies on the `PoktNodeAccountFetcher` interface, which mandates 
implementations to fetch account information from the Pocket network.

Refer to [account.go](https://github.com/pokt-network/shannon-sdk/blob/main/account.go)
for detailed information.

#### Application Client

The `ApplicationClient` fetches application information from the Pocket network.

It offers these methods:

- `GetApplication()`: Retrieves application information for a specified application address.
- `GateAllApplications()`: Retrieves all available applications on the network.
- `GetApplicationsDelegatingToGateway()`: Retrieves applications delegating to the gateway.

The `ApplicationClient` depends on the `poktroll` application query client,
which provides methods to fetch corresponding information from the Pocket network.

Refer to [application.go](https://github.com/pokt-network/shannon-sdk/blob/main/application.go)
for detailed information.

#### Application Ring

The `ApplicationRing` retrieves delegated gateways from `Application`s and generates
ring signatures from any actor in the ring.

It offers the following method:

- `GetRing()`: Retrieves the ring associated with the application.

The `ApplicationRing` relies on the `PublicKeyFetcher` interface, which requires
implementations to fetch the public key of the associated application.

**Note**: The `AccountClient` implements the `PublicKeyFetcher` interface and can
be used as a default implementation.

Refer to [application.go](https://github.com/pokt-network/shannon-sdk/blob/main/application.go)
for detailed information.

#### Block Client

The `BlockClient` fetches block information (e.g., block height) from the Pocket network.

It provides the following method:

- `LatestBlockHeight()`: Retrieves the latest block height from the network.

The `BlockClient` depends on the `PoktNodeStatusFetcher` interface, which requires
implementations to fetch the latest block height from the Pocket network.

Refer to [block.go](https://github.com/pokt-network/shannon-sdk/blob/main/block.go)
for detailed information.

The default implementation uses the `CosmosSDK`'s `http.HTTP` client to fetch the
block height.

#### Signer

The `Signer` signs `RelayRequests` to ensure their authenticity and integrity.

It provides the following method:

- `Sign()`: Signs a given `RelayRequest` using the provided `ApplicationRing`.

The `Signer` must set its `PrivateKeyHex` field to the private key of the associated
application or gateway.

Refer to [signer.go](https://github.com/pokt-network/shannon-sdk/blob/main/signer.go)
for detailed information.

#### Session Client

The `SessionClient` retrieves `Session` information from the Pocket network for a specified `Application` address, `Service.Id`, and block height.

It provides the following method:

- `GetSession()`: Retrieves session information for a given `Application` address,
`Service.Id`, and block height.


The `SessionClient` relies on the `PoktNodeSessionFetcher` interface, which requires implementations to fetch session information from the Pocket network.

Refer to [session.go](https://github.com/pokt-network/shannon-sdk/blob/main/session.go)
for detailed information.

#### Session Filter

To select the best supplier endpoint among available options, the SDK offers a
`SessionFilter` struct that filters out endpoints that do not meet specified criteria.

The `SessionFilter` requires an assigned `Session` in its `Session` field.
It supports adding filter functions to its `EndpointFilters` field to select the
best supplier endpoint.

`SessionFilter` provides the following methods:

- `AllEndpoints()`: Retrieves all `Endpoints` from the `Session`, mapped to their
respective `Supplier` addresses, allowing retrieval of all available supplier
endpoints and performing custom filtering.
- `FilteredEndpoints()`: Retrieves filtered endpoints based on specified filter functions.
Returned endpoints must pass all filter functions to be considered valid.

Filtered endpoints adhere to the `Endpoint` interface, which provides:

- `Header()`: Retrieves the `Session` header corresponding to the `Supplier`'s endpoint.
- `Supplier()`: Retrieves the `Supplier` address corresponding to the `Endpoint`.
- `Endpoint()`: Retrieves the `url.URL` of the endpoint.

Refer to [session.go](https://github.com/pokt-network/shannon-sdk/blob/main/session.go)
for detailed information.

#### Relayer

To send a `RelayRequest`, `ShannonSDK` exposes the `BuildRelayRequest` and
`ValidateRelayResponse` functions for building and validating `RelayRequests` and
`RelayResponses`, respectively.

- `BuildRelayRequest()`: Constructs a `RelayRequest` to send to the specified `Endpoint`
using a serialized `http.Request` (body and headers included).

The `Endpoint` interface provides necessary information (`SessionHeader` and
`Supplier` endpoint URL) for constructing the `RelayRequest`. Since the resulting
RelayRequest is unsigned, the consumer must sign it (using `Signer#Sign`) before sending.

SDK consumers can use any suitable HTTP client to send the `RelayRequest`.

- `ValidateRelayResponse()`: Validates a `RelayResponse` byte array against the
selected `Supplier`'s address.

To fetch the public key of the `Supplier`'s address, an implementation of
`PublicKeyFetcher` must be provided. Successful validation returns the verified
`RelayResponse`, which can then be processed to extract response headers and body.

Refer to [relay.go](https://github.com/pokt-network/shannon-sdk/blob/main/relay.go)
for detailed information.