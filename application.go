package sdk

import (
	"context"
	"errors"
	"fmt"
	"slices"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	query "github.com/cosmos/cosmos-sdk/types/query"
	"github.com/pokt-network/poktroll/pkg/crypto/rings"
	"github.com/pokt-network/poktroll/x/application/types"
	"github.com/pokt-network/ring-go"
)

// ApplicationClient is the interface to interact with the on-chain application-module.
//
// For example, it can be used to get the list of applications and the details of a specific application.
//
// The ApplicationClient uses the gRPC query client of the application module.
// QueryClient is made public because it should eventually become an interface, as it is being consumed here.
//
//	More details in the following link: https://go.dev/wiki/CodeReviewComments#interfaces
//
// This implementation could be extended in the future to leverage caching to avoid querying
// the blockchain for the same data multiple times, but such a cache would need to be invalidated by
// listening to the relevant events such as MsgStakeApplication, MsgUnstakeApplication etc...
type ApplicationClient struct {
	// TODO_TECHDEBT: Replace QueryClient with a PoktNodeAccountFetcher interface.
	types.QueryClient
}

// TODO_TECHDEBT(@adshmh): support pagination if/when the number of onchain applications
// grows enough to cause a performance issue with returning all applications at-once.
//
// TODO_TECHDEBT: Add filtering options to this method once they are supported by the on-chain module.
//
// GetAllApplications returns all applications in the network.
// It returns an error if the context deadline is exceeded while fetching the requested session.
func (ac *ApplicationClient) GetAllApplications(
	ctx context.Context,
) ([]types.Application, error) {
	var (
		fetchedApps []types.Application
		fetchErr    error
		// Will be closed to signal the completion of app fetch.
		doneCh = make(chan struct{})
	)

	// Launch AllApplications in goroutine
	go func() {
		// Close the channel to signal completion of fetch.
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

	// Wait for either result or context deadline
	select {
	case <-doneCh:
		return fetchedApps, fetchErr
	case <-ctx.Done():
		return []types.Application{}, ctx.Err()
	}
}

// GetApplication returns the details of the application with the given address.
// It returns an error if the context deadline is exceeded while fetching the requested session.
func (ac *ApplicationClient) GetApplication(
	ctx context.Context,
	appAddress string,
) (types.Application, error) {
	var (
		fetchedApp types.Application
		fetchErr   error
		// Will be closed to signal the completion of app fetch.
		doneCh = make(chan struct{})
	)

	// Launch GetApplication in goroutine
	go func() {
		// Close the channel to signal completion
		defer close(doneCh)

		req := &types.QueryGetApplicationRequest{Address: appAddress}
		// TODO_TECHDEBT(@adshmh): consider increasing the default response size:
		// e.g. using google.golang.org/grpc's MaxCallRecvMsgSize CallOption.
		res, err := ac.QueryClient.Application(ctx, req)
		if err != nil {
			fetchErr = err
			return
		}

		fetchedApp = res.Application
	}()

	// Wait for either completion of fetch or context deadline
	select {
	case <-doneCh:
		return fetchedApp, fetchErr
	case <-ctx.Done():
		return types.Application{}, ctx.Err()
	}
}

// TODO_TECHDEBT: Use a more efficient logic based on a filtering query of onchain applications,
// once the following enhancement on poktroll is implemented:
// https://github.com/pokt-network/poktroll/issues/767
//
// This is an inefficient implementation, as there can be a very large number
// of onchain applications, only a few of which are likely to be delegating to a specific gateway.
// But this can only be fixed once the above proposed enhancement on poktroll is completed.
//
// GetApplicationsDelegatingToGateway returns the application addresses that are
// delegating to the given gateway address.
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
		// Get the gateways that are delegated to the application
		// at the query height and check if the given gateway address is in the list.
		gatewaysDelegatedTo := rings.GetRingAddressesAtSessionEndHeight(&application, sessionEndHeight)
		if slices.Contains(gatewaysDelegatedTo, gatewayAddress) {
			// The application is delegating to the given gateway address, add it to the list.
			gatewayDelegatingApplications = append(gatewayDelegatingApplications, application.Address)
		}
	}

	return gatewayDelegatingApplications, nil
}

type ApplicationRing struct {
	types.Application
	PublicKeyFetcher
}

// GetRing returns the ring for the application until the current session end height.
// The ring is created using the application's public key and the public keys of
// the gateways that are currently delegated from the application.
func (a ApplicationRing) GetRing(
	ctx context.Context,
	sessionEndHeight uint64,
) (addressRing *ring.Ring, err error) {
	if a.PublicKeyFetcher == nil {
		return nil, errors.New("GetRing: Public Key Fetcher not set")
	}

	// Get the gateway addresses that are delegated from the application at the query height.
	currentGatewayAddresses := rings.GetRingAddressesAtSessionEndHeight(&a.Application, sessionEndHeight)

	ringAddresses := make([]string, 0)
	ringAddresses = append(ringAddresses, a.Application.Address)

	// If there are no current gateway addresses, use the application address as the ring address.
	if len(currentGatewayAddresses) == 0 {
		ringAddresses = append(ringAddresses, a.Application.Address)
	} else {
		ringAddresses = append(ringAddresses, currentGatewayAddresses...)
	}

	ringPubKeys := make([]cryptotypes.PubKey, 0, len(ringAddresses))
	for _, address := range ringAddresses {
		pubKey, err := a.PublicKeyFetcher.GetPubKeyFromAddress(ctx, address)
		if err != nil {
			return nil, err
		}
		ringPubKeys = append(ringPubKeys, pubKey)
	}

	return rings.GetRingFromPubKeys(ringPubKeys)
}

// PublicKeyFetcher specifies an interface that allows getting the public
// key corresponding to an address.
// It is used by the ApplicationRing struct to construct the Application's Ring
// for signing relay requests.
// The AccountClient struct provides an implementation of this interface.
type PublicKeyFetcher interface {
	GetPubKeyFromAddress(context.Context, string) (cryptotypes.PubKey, error)
}
