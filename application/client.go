package application

import (
	"context"

	"github.com/cosmos/gogoproto/grpc"
	"github.com/pokt-network/poktroll/x/application/types"
)

var _ ApplicationClient = (*applicationClient)(nil)

type applicationClient struct {
	queryClient types.QueryClient
}

func NewApplicationClient(grpcConn grpc.ClientConn) (ApplicationClient, error) {
	return &applicationClient{
		queryClient: types.NewQueryClient(grpcConn),
	}, nil
}

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
