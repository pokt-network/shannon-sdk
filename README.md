# ShannonSDK <!-- omit in toc -->

ShannonSDK is a Software Development Kit designed to facilitate interaction with
the POKT Network for developers of both `Gateway`s and sovereign `Application`s.
It streamlines the process of building POKT compatible `RelayRequests`, verifying
`RelayResponses` from RelayMiners, `Suppliers` co-processors, and abstracting out
other protocol-specific details.

To learn more about any of the actors or components mentioned above, please refer
to [dev.poktroll.com/category/actors](https://dev.poktroll.com/category/actors).

- [Overview](#overview)
- [Key Components](#key-components)
- [Initialization](#initialization)
- [Usage](#usage)
  - [Get Session Supplier Endpoints](#get-session-supplier-endpoints)
  - [Get Gateway Delegating Applications](#get-gateway-delegating-applications)
  - [Send Relay](#send-relay)
  - [Helper functions](#helper-functions)
- [ShannonSDK Internals \& Design (for developers only)](#shannonsdk-internals--design-for-developers-only)
  - [Code Organization](#code-organization)
    - [Interface Design](#interface-design)
    - [Exposed Concrete Types](#exposed-concrete-types)
    - [sdk.go](#sdkgo)
    - [Public vs Private Fields](#public-vs-private-fields)
  - [Implementation Details](#implementation-details)
  - [Error Handling](#error-handling)
  - [Dependencies implementation](#dependencies-implementation)
  - [Poktroll dependencies](#poktroll-dependencies)

## Overview

ShannonSDK encapsulates various clients and a signer necessary for interacting
with the `Poktroll` network. It provides an intuitive interface to manage `Sessions`,
and `RelayRequests`.

This document outlines the key components and functionalities of the SDK, along
with detailed usage instructions.

## Key Components

The SDK consists of the following core components:

- **ApplicationClient**: Handles interactions related to applications on the network.
- **SessionClient**: Manages session-related operations.
- **AccountClient**: Deals with account-related queries and operations.
- **SharedParamsClient**: Provides shared parameters such as various governance params to the SDK.
- **BlockClient**: Fetches information about blocks on the network.
- **RelayClient**: Sends relay requests to the network.
- **Signer**: Signs relay requests to ensure authenticity and integrity.

## Initialization

To create a new instance of ShannonSDK, you need to provide the implementations for
the required clients and signer. Here is an example of how to initialize the SDK:

```go
applicationClient := NewApplicationClient(grpcConn)
sessionClient := NewSessionClient(grpcConn)
accountClient := NewAccountClient(grpcConn)
sharedParamsClient := NewSharedParamsClient(grpcConn)
blockClient := NewBlockClient(poktrollRPCURL)
relayClient := NewRelayClient()
signer := NewSigner(privateKeyHex)

sdk, err := NewShannonSDK(
  applicationClient,
  sessionClient,
  accountClient,
  sharedParamsClient,
  blockClient,
  relayClient,
  signer,
)
if err != nil {
    log.Fatalf("failed to create ShannonSDK: %v", err)
}
```

## Usage

### Get Session Supplier Endpoints

The `GetSessionSupplierEndpoints` method retrieves the current `Session` and its
assigned `Suppliers`' endpoints for a given `Application` address and `serviceId`.

```go
ctx := context.Background()
appAddress := "your-app-address"
serviceId := "your-service-id"

sessionSuppliers, err := sdk.GetSessionSupplierEndpoints(ctx, appAddress, serviceId)
if err != nil {
    log.Fatalf("failed to get session supplier endpoints: %v", err)
}

for _, endpoint := range sessionSuppliers.SuppliersEndpoints {
    fmt.Printf("Supplier: %s, URL: %s\n", endpoint.SupplierAddress, endpoint.Url)
}
```

### Get Gateway Delegating Applications

The `GetApplicationsDelegatingToGateway` method returns the `Application`s that are
delegating to a given `Gateway` address.

```go
gatewayAddress := "your-gateway-address"

delegatingApps, err := sdk.GetApplicationsDelegatingToGateway(ctx, gatewayAddress)
if err != nil {
    log.Fatalf("failed to get gateway delegating applications: %v", err)
}

for _, app := range delegatingApps {
    fmt.Println("Delegating Application:", app)
}
```

### Send Relay

The `SendRelay` method signs and sends a `RelayRequest` to the given `Supplier` endpoint,
and verifies the `Supplier`'s signature on the `RelayResponse`.

```go
func (s *server) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
    ctx := r.Context()
    // Example the appAddress and serviceId retrieval from the request
    serviceId := request.URL.Query().Get("serviceId")
    appAddress := request.URL.Query().Get("appAddress")

    // Get the session supplier endpoints
    sessionSupplierEndpoints, err := sdk.GetSessionSupplierEndpoints(ctx, appAddress, serviceId)
    if err != nil {
        panic("TODO: handle error")
    }

    // Chose a supplier endpoint
    selectedSupplier := sessionSupplierEndpoints.SuppliersEndpoints[0]

    // Serialize the whole upstream request
    _, requestBz, err := types.SerializeHTTPRequest(request)

    // Forward the request to the selected supplier endpoint by using the same
    // method and headers.
    // The SDK will take care of signing the request and verifying the response.
    relayResponse, err := sdk.SendRelay(ctx, selectedSupplier, requestBz)
    if err != nil {
        panic("TODO: handle error")
    }

    // Deserialize the http response from the relay response payload
    httpResponse, err := types.DeserializeHTTPResponse(relayResponse.Payload)
    if err != nil {
        panic("TODO: handle error")
    }

    // Set the response headers
    httpResponse.CopyToHTTPHeader(writer.Header())

    // Set the response status code
    writer.WriteHeader(int(httpResponse.StatusCode))

    // Send back the response body to the client
    if _, err := writer.Write(httpResponse.BodyBz); err != nil {
        panic("TODO: handle error")
    }
}
```

### Helper functions

In order to transparently relay requests and responses between `Gateway`s/`Application`s
and the `RelayMiner`s, the full request and response components must be transferred
between the parties. This includes the request's method, headers, body, and the response's
status code, headers, and body.

Since the `http.Request` and `http.Response` types are not serializable, the SDK provides
helper functions that return serializable representations of these types.

SDK consumers can use them to serialize upstream requests and embed them in `RelayRequest`
payloads, and deserialize `RelayResponse` payloads to obtain the original responses

```go
// Parse the http.Request to get the request components that will be sent
// to the RelayMiner.
poktHTTPRequest, requestBz, err := sdktypes.SerializeHTTPRequest(request)

// SendRelay

// Parse the RelayResponse payload to get the serviceResponse that will
// be forwarded to the client.
serviceResponse, err := sdktypes.DeserializeHTTPResponse(relayResponse.Payload)
```

## ShannonSDK Internals & Design (for developers only)

### Code Organization

The following is the top-level structure the SDK repo is moving towards:

```bash
application.go
block.go
relay.go
session.go
sign.go
```

_TODO_DOCUMENT: Add the output of `tree -L 2` once the above structure is implemented._
_TODO_DOCUMENT: Add a mermaid diagram of the exposed types once complete._

#### Interface Design

The `SDK` **IS NOT DESIGNED** to provide interfaces to the consumer.

The `SDK` **IS DESIGNED** to consume functionality from other packages via interfaces.

This follows Golang's best practices for interfaces as described [here](https://go.dev/wiki/CodeReviewComments#interfaces).

#### Exposed Concrete Types

Each file (in the top level directory) will have a client implemented and returned
as a concrete struct.

For example, `ApplicationClient` is a `struct` that will be returned by `application.go`
rather than an interface.

#### sdk.go

**NOTE: If you are reading this and the documentation is outdated, please update it!**

The `sdk.go` is a **TEMPORARY** that needs to be split file needs to be split into
`application.go`, `supplier.go`, etc...

A `ShannonSDK` struct was defined initially but is non-ideal. It forces the
user/developer to construct the entire struct even if they need a small fraction
of the functionality.

The following is an example of using a small subset of the SDK:

```go
session, err := sessionClient.CurrentSession()
if err != nil {
   return nil, err
}

endpoints := sdk.Endpoints(session, serviceID)
```

#### Public vs Private Fields

The goal of this `SDK` is to make all fields of concrete types public to the user
if there is a potential need for the user to set them directly.

**IT SHOULD** be possible for the user to initialize any component of the SDK by
creating a struct and setting the bare minimum necessary fields.

For example, the SDK biases towards the following design:

```go
c := SessionClient {
     HttpClient:  myCustomHttpTransport
}
```

Instead of the following design:

```go
c := NewSessionClient(nil, nil, myCustomHttpTransport, nil, nil)
```

### Implementation Details

ShannonSDK relies on interfaces for its dependencies, which must be implemented
by the developer. This allows flexibility in how network access is handled,
whether data is cached, and other implementation specifics.

### Error Handling

The SDK does not define any custom error types. It relies on the errors returned
by its dependencies. This design choice simplifies error handling by ensuring
that errors are propagated directly from the underlying implementations.

### Dependencies implementation

`./client` package contains example implementations of the clients required by
the SDK. These implementations are based on the `grpc` and `http` packages in
Go, and they can be used as a reference for building more complex ones.

### Poktroll dependencies

The SDK relies on the `poktroll` repository for the `types` package, which
acts as a single source of truth for the data structures used by the SDK.
This design choice ensures consistency across the various components of the
POKT ecosystem.
