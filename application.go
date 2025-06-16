package sdk

import (
	"context"
	"fmt"
	"slices"

	query "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/pokt-network/poktroll/pkg/crypto/rings"
	"github.com/pokt-network/poktroll/x/application/types"
)

// ApplicationClient is the interface to interact with the on-chain application-module.
//
// - Used to get the list of applications and the details of a specific application
// - Uses the gRPC query client of the application module
// - QueryClient is public for future interface abstraction (see: https://go.dev/wiki/CodeReviewComments#interfaces)
// - Could be extended to use caching, but cache must be invalidated by events (e.g. MsgStakeApplication, MsgUnstakeApplication)
//
// Implements sdk.ApplicationClient interface.
type ApplicationClient struct {
	// TODO_TECHDEBT: Replace QueryClient with a PoktNodeAccountFetcher interface.
	types.QueryClient
}

// ------------------------- Methods -------------------------

// GetAllApplications returns all applications in the network.
//
// - Returns error if context deadline is exceeded
// - TODO_TECHDEBT(@adshmh): Support pagination if/when onchain application count grows
// - TODO_TECHDEBT: Add filtering options when supported by on-chain module
func (ac *ApplicationClient) GetAllApplications(
	ctx context.Context,
) ([]types.Application, error) {
	var (
		fetchedApps []types.Application
		fetchErr    error
		doneCh      = make(chan struct{}) // Signals completion of app fetch
	)

	go func() {
		defer close(doneCh)
		req := &types.QueryAllApplicationsRequest{
			Pagination: &query.PageRequest{
				Limit: query.PaginationMaxLimit,
			},
		}
		res, err := ac.QueryClient.AllApplications(ctx, req)
		if err != nil {
			fetchErr = err
			return
		}
		fetchedApps = res.Applications
	}()

	select {
	case <-doneCh:
		return fetchedApps, fetchErr
	case <-ctx.Done():
		return []types.Application{}, ctx.Err()
	}
}

// GetApplication returns the details of the application with the given address.
//
// - Returns error if context deadline is exceeded
func (ac *ApplicationClient) GetApplication(
	ctx context.Context,
	appAddress string,
) (types.Application, error) {
	var (
		fetchedApp types.Application
		fetchErr   error
		doneCh     = make(chan struct{}) // Signals completion of app fetch
	)

	go func() {
		defer close(doneCh)
		req := &types.QueryGetApplicationRequest{Address: appAddress}
		// TODO_TECHDEBT(@adshmh): Consider increasing default response size (e.g. grpc MaxCallRecvMsgSize)
		res, err := ac.QueryClient.Application(ctx, req)
		if err != nil {
			fetchErr = err
			return
		}
		fetchedApp = res.Application
	}()

	select {
	case <-doneCh:
		return fetchedApp, fetchErr
	case <-ctx.Done():
		return types.Application{}, ctx.Err()
	}
}

// GetApplicationsDelegatingToGateway returns the application addresses that are
// delegating to the given gateway address.
//
// - Inefficient: fetches all applications, then filters by delegation
// - TODO_TECHDEBT: Use filtering query once https://github.com/pokt-network/poktroll/issues/767 is implemented
func (ac *ApplicationClient) GetApplicationsDelegatingToGateway(
	ctx context.Context,
	gatewayAddress string,
	sessionEndHeight uint64,
) ([]string, error) {
	allApplications, err := ac.GetAllApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetApplicationsDelegatingToGateway: error getting all applications: %w", err)
	}

	gatewayDelegatingApplications := make([]string, 0)
	for _, application := range allApplications {
		gatewaysDelegatedTo := rings.GetRingAddressesAtSessionEndHeight(&application, sessionEndHeight)
		if slices.Contains(gatewaysDelegatedTo, gatewayAddress) {
			gatewayDelegatingApplications = append(gatewayDelegatingApplications, application.Address)
		}
	}

	return gatewayDelegatingApplications, nil
}
