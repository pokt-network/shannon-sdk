# ShannonSDK
ShannonSDK is a software development kit designed to facilitate interaction with
the Poktroll network for both `Gateway` and sovereign `Application` developers.
It streamlines the process of building protocol-correct `RelayRequests` and
verifying the correctness of `RelayResponses` received from `Suppliers`' RelayMiners.

## Overview
ShannonSDK encapsulates various clients and a signer necessary for interacting
with the `Poktroll` network. It provides an intuitive interface to manage `Sessions`,
and `RelayRequests`.
This document outlines the key components and functionalities of the SDK, along
with detailed usage instructions.

## Key Components
The SDK consists of the following core components:

* ApplicationClient: Handles interactions related to applications on the network.
* SessionClient: Manages session-related operations.
* AccountClient: Deals with account-related queries and operations.
* BlockClient: Fetches information about blocks on the network.
* RelayClient: Sends relay requests to the network.
* Signer: Signs relay requests to ensure authenticity and integrity.

## Initialization
To create a new instance of ShannonSDK, you need to provide implementations for
the required clients and signer. Here is an example of how to initialize the SDK:

```go
applicationClient := NewApplicationClient(grpcConn)
sessionClient := NewSessionClient(grpcConn)
accountClient := NewAccountClient(grpcConn)
blockClient := NewBlockClient(poktrollRPCURL)
relayClient := NewRelayClient()
signer := NewSigner(privateKeyHex)

sdk, err := NewShannonSDK(
  applicationClient,
  sessionClient,
  accountClient,
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
The `GetGatewayDelegatingApplications` method returns the `Application`s that are
delegating to a given `Gateway` address.

```go
gatewayAddress := "your-gateway-address"

delegatingApps, err := sdk.GetGatewayDelegatingApplications(ctx, gatewayAddress)
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
    ...

    // Get the session supplier endpoints
    sessionSupplierEndpoints := sdk.GetSessionSupplierEndpoints(ctx, appAddress, serviceId)

    // Chose a supplier endpoint
    selectedSupplier := sessionSupplierEndpoints.SuppliersEndpoints[0]

    // Read the request body
    requestBodyBz, err := io.ReadAll(request.Body)
    if err != nil {
        // Handle error
    }
    request.Body.Close()

    // Forward the request to the selected supplier endpoint by using the same
    // method and headers.
    // The SDK will take care of signing the request and verifying the response.
    relayResponse, err := sdk.SendRelay(
      ctx,
      selectedSupplier,
      requestBody,
      request.Method,
      request.Header,
    )
    if err != nil {
        // Handle error
    }

    // Send back the relay response to the client
    if _, err := writer.Write(relayResponse.Payload); err != nil {
        // Handle error
    }
}
```

## Implementation Details
Dependencies
ShannonSDK relies on interfaces for its dependencies, which must be implemented
by the developer. This allows flexibility in how network access is handled,
whether data is cached, and other implementation specifics.

## Error Handling
The SDK does not define any custom error types. It relies on the errors returned
by its dependencies. This design choice simplifies error handling by ensuring
that errors are propagated directly from the underlying implementations.

## Dependencies implementation

`./client` package contains simple implementations of the clients required by
the SDK. These implementations are based on the `grpc` and `http` packages in
Go, and they can be used as a reference for building more complex ones.
