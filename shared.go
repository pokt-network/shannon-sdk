package sdk

import (
	"context"

	"github.com/cosmos/gogoproto/grpc"
	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
	grpcoptions "google.golang.org/grpc"
)

// --- Interfaces ---

// PoktNodeSharedFetcher is used by SharedClient to fetch shared module parameters using poktroll shared type.
//
// - Most users can rely on the default implementation (see NewPoktNodeSharedFetcher).
// - Custom implementations allow granular control over SharedClient interactions with the POKT full node.
type PoktNodeSharedFetcher interface {
	Params(
		context.Context,
		*sharedtypes.QueryParamsRequest,
		...grpcoptions.CallOption,
	) (*sharedtypes.QueryParamsResponse, error)
}

// --- Structs ---

// SharedClient interacts with the on-chain shared module.
//
// - Used to get shared module parameters such as grace period, session length, etc.
// - QueryClient is public for future interface abstraction (see: https://go.dev/wiki/CodeReviewComments#interfaces)
type SharedClient struct {
	// TODO_TECHDEBT: Replace QueryClient with a PoktNodeSharedFetcher interface.
	sharedtypes.QueryClient
}

// --- Constructor Functions ---

// NewPoktNodeSharedFetcher creates a new PoktNodeSharedFetcher using the provided gRPC connection.
//
// - Most users should use this default implementation.
// - Returns a fetcher that can be used with SharedClient.
func NewPoktNodeSharedFetcher(conn grpc.ClientConn) PoktNodeSharedFetcher {
	return sharedtypes.NewQueryClient(conn)
}

// --- Methods ---

// GetParams returns the shared module parameters from the blockchain.
//
// - Returns the current shared module parameters including grace period, session configuration, etc.
// - Returns error if context deadline is exceeded or query fails
func (sc *SharedClient) GetParams(ctx context.Context) (sharedtypes.Params, error) {
	var (
		fetchedParams sharedtypes.Params
		fetchErr      error
		doneCh        = make(chan struct{}) // Signals completion of params fetch
	)

	go func() {
		defer close(doneCh)
		req := &sharedtypes.QueryParamsRequest{}
		res, err := sc.QueryClient.Params(ctx, req)
		if err != nil {
			fetchErr = err
			return
		}
		fetchedParams = res.Params
	}()

	select {
	case <-doneCh:
		return fetchedParams, fetchErr
	case <-ctx.Done():
		return sharedtypes.Params{}, ctx.Err()
	}
}