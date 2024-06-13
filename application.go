package sdk

import (
	"context"

	"github.com/cosmos/gogoproto/grpc"
	"github.com/pokt-network/poktroll/x/application/types"
)

// ApplicationClient is an ApplicationClient implementation that uses the gRPC
// query client of the application module.
// QueryClient is made public because it should eventually become an interface, as it is being consumed here.
//
//	More details in the following link: https://go.dev/wiki/CodeReviewComments#interfaces
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
