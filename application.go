package sdk

import (
	"context"
	"errors"
	"fmt"
	"slices"

	ring_secp256k1 "github.com/athanorlabs/go-dleq/secp256k1"
	ringtypes "github.com/athanorlabs/go-dleq/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/pokt-network/ring-go"

	"github.com/cosmos/gogoproto/grpc"
	"github.com/pokt-network/poktroll/x/application/types"
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
	types.QueryClient
}

// NewApplicationClient creates a new application client with the provided gRPC connection.
func NewApplicationClient(grpcConn grpc.ClientConn) *ApplicationClient {
	return &ApplicationClient{
		QueryClient: types.NewQueryClient(grpcConn),
	}
}

// GetAllApplications returns all applications in the network.
// TODO_TECHDEBT: Add filtering options to this method once they are supported by the on-chain module.
func (ac *ApplicationClient) GetAllApplications(
	ctx context.Context,
) ([]types.Application, error) {
	req := &types.QueryAllApplicationsRequest{}
	res, err := ac.QueryClient.AllApplications(ctx, req)
	if err != nil {
		return []types.Application{}, err
	}

	return res.Applications, nil
}

// GetApplication returns the details of the application with the given address.
func (ac *ApplicationClient) GetApplication(
	ctx context.Context,
	appAddress string,
) (types.Application, error) {
	req := &types.QueryGetApplicationRequest{Address: appAddress}
	res, err := ac.QueryClient.Application(ctx, req)
	if err != nil {
		return types.Application{}, err
	}

	return res.Application, nil
}

// GetApplicationsDelegatingToGateway returns the application addresses that are
// delegating to the given gateway address.
func (ac *ApplicationClient) GetApplicationsDelegatingToGateway(
	ctx context.Context,
	gatewayAddress string,
	queryHeight uint64,
) ([]string, error) {
	allApplications, err := ac.GetAllApplications(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetApplicationsDelegatingToGateway: error getting all applications: %w", err)
	}

	gatewayDelegatingApplications := make([]string, 0)
	for _, application := range allApplications {
		appRing := ApplicationRing{Application: application, SessionEndBlock: queryHeight}
		// Get the gateways that are delegated to the application
		// at the query height and check if the given gateway address is in the list.
		gatewaysDelegatedTo := appRing.ringAddressesAtBlock()
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
	SessionEndBlock uint64
}

// GetRing returns the ring for the application.
// The ring is created using the application's public key and the public keys of
// the gateways that are currently delegated from the application.
func (a ApplicationRing) GetRing(
	ctx context.Context,
) (addressRing *ring.Ring, err error) {
	if a.PublicKeyFetcher == nil {
		return nil, errors.New("GetRing: Public Key Fetcher not set")
	}

	if a.SessionEndBlock <= 0 {
		return nil, errors.New("GetRing: Current Height not set")
	}

	// Get the gateway addresses that are delegated from the application
	// at the query height.
	currentGatewayAddresses := a.ringAddressesAtBlock()

	ringAddresses := make([]string, 0)
	ringAddresses = append(ringAddresses, a.Application.Address)

	// If there are no current gateway addresses, use the application address as the ring address.
	if len(currentGatewayAddresses) == 0 {
		ringAddresses = append(ringAddresses, a.Application.Address)
	} else {
		ringAddresses = append(ringAddresses, currentGatewayAddresses...)
	}

	curve := ring_secp256k1.NewCurve()
	ringPoints := make([]ringtypes.Point, 0, len(ringAddresses))

	// Create a ring point for each address.
	for _, address := range ringAddresses {
		pubKey, err := a.PublicKeyFetcher.GetPubKeyFromAddress(ctx, address)
		if err != nil {
			return nil, err
		}

		point, err := curve.DecodeToPoint(pubKey.Bytes())
		if err != nil {
			return nil, err
		}

		ringPoints = append(ringPoints, point)
	}

	return ring.NewFixedKeyRingFromPublicKeys(ring_secp256k1.NewCurve(), ringPoints)
}

func (a ApplicationRing) ringAddressesAtBlock() []string {
	// Get the current active delegations for the application and use them as a base.
	activeDelegationsAtHeight := a.Application.DelegateeGatewayAddresses

	// Use a map to keep track of the gateways addresses that have been added to
	// the active delegations slice to avoid duplicates.
	addedDelegations := make(map[string]struct{})

	// Iterate over the pending undelegations recorded at their respective block
	// height and check whether to add them back as active delegations.
	for pendingUndelegationHeight, undelegatedGateways := range a.Application.PendingUndelegations {
		// If the pending undelegation happened BEFORE the target session end height, skip it.
		// The gateway is pending undelegation and simply has not been pruned yet.
		// It will be pruned in the near future.
		// TODO_DISCUSS: should we use the session's ending height instead?
		if pendingUndelegationHeight < a.SessionEndBlock {
			continue
		}
		// Add back any gateway address  that was undelegated after the target session
		// end height, as we consider it not happening yet relative to the target height.
		for _, gatewayAddress := range undelegatedGateways.GatewayAddresses {
			if _, ok := addedDelegations[gatewayAddress]; ok {
				continue
			}

			activeDelegationsAtHeight = append(activeDelegationsAtHeight, gatewayAddress)
			// Mark the gateway address as added to avoid duplicates.
			addedDelegations[gatewayAddress] = struct{}{}
		}

	}

	return activeDelegationsAtHeight
}

// PublicKeyFetcher specifies an interface that allows getting the public key corresponding to an address.
// It is used by the ApplicationRing struct to construct the Application's Ring for signing relay requests.
// The AccountClient struct provides an implementation of this interface.
type PublicKeyFetcher interface {
	GetPubKeyFromAddress(context.Context, string) (cryptotypes.PubKey, error)
}
