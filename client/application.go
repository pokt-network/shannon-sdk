package client

import (
	"context"

	"github.com/cosmos/gogoproto/grpc"
	"github.com/pokt-network/poktroll/x/application/types"

	"github.com/pokt-network/shannon-sdk/sdk"
)

var _ sdk.ApplicationClient = (*applicationClient)(nil)

// applicationClient is an ApplicationClient implementation that uses the gRPC query client
// of the application module.
// It is a wrapper around the Poktroll generated application QueryClient.
type applicationClient struct {
	queryClient types.QueryClient
}

// NewApplicationClient creates a new application client with the provided gRPC connection.
func NewApplicationClient(grpcConn grpc.ClientConn) (sdk.ApplicationClient, error) {
	return &applicationClient{
		queryClient: types.NewQueryClient(grpcConn),
	}, nil
}

// GetAllApplications returns all applications in the network.
func (ac *applicationClient) GetAllApplications(
	ctx context.Context,
) ([]types.Application, error) {
	req := &types.QueryAllApplicationsRequest{}
	res, err := ac.queryClient.AllApplications(ctx, req)
	if err != nil {
		return []types.Application{}, err
	}

	return res.Applications, nil
}

// GetApplication returns the details of the application with the given address.
func (ac *applicationClient) GetApplication(
	ctx context.Context,
	appAddress string,
) (types.Application, error) {
	req := &types.QueryGetApplicationRequest{Address: appAddress}
	res, err := ac.queryClient.Application(ctx, req)
	if err != nil {
		return types.Application{}, err
	}

	return res.Application, nil
}
