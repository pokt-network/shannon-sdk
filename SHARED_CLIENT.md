# SharedClient Implementation

## Overview

This document describes the implementation of the `SharedClient` in the Shannon SDK, which provides access to shared module parameters from the Pocket Network blockchain.

## Files Added

### 1. `shared.go`
- **SharedClient struct**: Main client for interacting with shared module
- **PoktNodeSharedFetcher interface**: Interface for fetching shared parameters
- **NewPoktNodeSharedFetcher()**: Constructor function following SDK patterns
- **GetParams()**: Method to fetch shared module parameters

### 2. `shared_test.go`  
- **Unit tests**: Test coverage for SharedClient functionality
- **Mock implementation**: mockSharedFetcher for testing
- **Test cases**: Success and error scenarios

## Usage

### Basic Usage

```go
import (
    sdk "github.com/pokt-network/shannon-sdk"
    sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
)

// Create a gRPC connection
conn, err := grpc.Dial("localhost:9090", grpc.WithInsecure())
if err != nil {
    panic(err)
}

// Create shared client
sharedClient := &sdk.SharedClient{
    QueryClient: sharedtypes.NewQueryClient(conn),
}

// Get shared parameters
ctx := context.Background()
params, err := sharedClient.GetParams(ctx)
if err != nil {
    panic(err)
}

// Access parameters
fmt.Printf("Grace Period: %d blocks\n", params.GracePeriodEndOffsetBlocks)
fmt.Printf("Session Length: %d blocks\n", params.NumBlocksPerSession)
```

### Integration with PATH

The SharedClient integrates seamlessly with PATH's grace period logic:

```go
// In PATH's FullNode interface
func (lfn *LazyFullNode) GetSharedParams(ctx context.Context) (*sharedtypes.Params, error) {
    params, err := lfn.sharedClient.GetParams(ctx)
    if err != nil {
        return nil, fmt.Errorf("GetSharedParams: error getting shared module parameters: %w", err)
    }
    return &params, nil
}
```

## Architecture

### Follows Shannon SDK Patterns

1. **Interface-based design**: Uses `PoktNodeSharedFetcher` interface
2. **Context support**: All methods accept `context.Context`
3. **Error handling**: Consistent error wrapping and propagation
4. **Async pattern**: Uses goroutines with done channels for timeout handling
5. **Type safety**: Leverages strong typing from poktroll shared types

### Parameters Available

The SharedClient provides access to all shared module parameters:

- `NumBlocksPerSession` - Session length in blocks
- `GracePeriodEndOffsetBlocks` - Grace period after session end
- `ClaimWindowOpenOffsetBlocks` - When claim window opens
- `ClaimWindowCloseOffsetBlocks` - When claim window closes  
- `ProofWindowOpenOffsetBlocks` - When proof window opens
- `ProofWindowCloseOffsetBlocks` - When proof window closes
- `SupplierUnbondingPeriodSessions` - Supplier unbonding period
- `ApplicationUnbondingPeriodSessions` - Application unbonding period
- `GatewayUnbondingPeriodSessions` - Gateway unbonding period
- `ComputeUnitsToTokensMultiplier` - Economic conversion factor

## Benefits

### 1. **Protocol Alignment**
- Queries actual on-chain parameters instead of hardcoded values
- Ensures PATH stays synchronized with protocol upgrades
- Eliminates drift between code and blockchain state

### 2. **Session Management**
- Enables precise grace period calculations
- Supports dynamic session length changes
- Facilitates proper session overlap handling

### 3. **Maintainability**
- Consistent with other Shannon SDK clients
- Well-tested with comprehensive unit tests
- Clear interface separation for future enhancements

## Testing

Run tests with:
```bash
cd /Users/olshansky/workspace/pocket/shannon-sdk
go test -v ./... -run TestSharedClient
```

The test suite covers:
- Successful parameter fetching
- Error handling scenarios
- Constructor function validation
- Mock implementation for isolated testing

## Integration Points

### PATH Integration
- Used in `LazyFullNode.GetSharedParams()`
- Used in `CachingFullNode.GetSharedParams()`
- Enables grace period logic in session management

### Future Extensions
- Could be extended for parameter caching
- Could support parameter change notifications
- Could add filtering for specific parameter subsets

This implementation provides the foundation for PATH's session grace period functionality while maintaining consistency with the Shannon SDK architecture.