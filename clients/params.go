package client

import (
	"context"

	"github.com/cosmos/gogoproto/grpc"
	"github.com/pokt-network/poktroll/x/shared/types"
)

// sharedParamsClient is a SharedParamsClient implementation that uses the gRPC
// query client of the on-chain shared module.
type SharedParamsClient struct {
	queryClient types.QueryClient
}

// NewSharedParamsClient creates a new share params client with the provided gRPC connection.
func NewSharedParamsClient(grpcConn grpc.ClientConn) (*SharedParamsClient, error) {
	return &SharedParamsClient{
		queryClient: types.NewQueryClient(grpcConn),
	}, nil
}

// GetParams returns the params of the poktroll on-chain shared module.
func (pc *SharedParamsClient) GetParams(
	ctx context.Context,
) (*types.Params, error) {
	req := &types.QueryParamsRequest{}
	res, err := pc.queryClient.Params(ctx, req)
	if err != nil {
		return nil, err
	}

	return &res.Params, nil
}
