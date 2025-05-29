package sdk

import (
	"context"

	query "github.com/cosmos/cosmos-sdk/types/query"
	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
	"github.com/pokt-network/poktroll/x/supplier/types"
)

type SupplierRing struct {
	sharedtypes.Supplier
	PublicKeyFetcher
}

// SupplierClient is the interface to interact with the on-chain Supplier-module.
//
// - Used to get the list of Suppliers and the details of a specific Supplier
// - Uses the gRPC query client of the Supplier module
// - QueryClient is public for future interface abstraction (see: https://go.dev/wiki/CodeReviewComments#interfaces)
// - Could be extended to use caching, but cache must be invalidated by events (e.g. MsgStakeSupplier, MsgUnstakeSupplier)
//
// Implements sdk.SupplierClient interface.
type SupplierClient struct {
	// TODO_TECHDEBT: Replace QueryClient with a PoktNodeAccountFetcher interface.
	types.QueryClient
}

// ------------------------- Methods -------------------------

// GetAllSuppliers returns all Suppliers in the network.
//
// - Returns error if context deadline is exceeded
// - TODO_TECHDEBT(@adshmh): Support pagination if/when onchain Supplier count grows
// - TODO_TECHDEBT: Add filtering options when supported by on-chain module
func (ac *SupplierClient) GetAllSuppliers(
	ctx context.Context,
) ([]sharedtypes.Supplier, error) {
	var (
		fetchedSuppliers []sharedtypes.Supplier
		fetchErr         error
		doneCh           = make(chan struct{}) // Signals completion of Supplier fetch
	)

	go func() {
		defer close(doneCh)
		req := &types.QueryAllSuppliersRequest{
			Pagination: &query.PageRequest{
				Limit: query.PaginationMaxLimit,
			},
		}
		res, err := ac.QueryClient.AllSuppliers(ctx, req)
		if err != nil {
			fetchErr = err
			return
		}
		fetchedSuppliers = res.Supplier
	}()

	select {
	case <-doneCh:
		return fetchedSuppliers, fetchErr
	case <-ctx.Done():
		return []sharedtypes.Supplier{}, ctx.Err()
	}
}

// GetSupplier returns the details of the Supplier with the given address.
//
// - Returns error if context deadline is exceeded
func (ac *SupplierClient) GetSupplier(
	ctx context.Context,
	SupplierAddress string,
) (sharedtypes.Supplier, error) {
	var (
		fetchedSupplier sharedtypes.Supplier
		fetchErr        error
		doneCh          = make(chan struct{}) // Signals completion of Supplier fetch
	)

	go func() {
		defer close(doneCh)
		req := &types.QueryGetSupplierRequest{OperatorAddress: SupplierAddress}
		// TODO_TECHDEBT(@adshmh): Consider increasing default response size (e.g. grpc MaxCallRecvMsgSize)
		res, err := ac.QueryClient.Supplier(ctx, req)
		if err != nil {
			fetchErr = err
			return
		}
		fetchedSupplier = res.Supplier
	}()

	select {
	case <-doneCh:
		return fetchedSupplier, fetchErr
	case <-ctx.Done():
		return sharedtypes.Supplier{}, ctx.Err()
	}
}
