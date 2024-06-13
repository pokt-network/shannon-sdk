package sdk

import (
	"context"

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
