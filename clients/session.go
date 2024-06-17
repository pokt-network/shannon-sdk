package client

import (
	"context"

	"github.com/cosmos/gogoproto/grpc"
	"github.com/pokt-network/poktroll/x/session/types"
	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"

	"github.com/pokt-network/shannon-sdk/sdk"
)

var _ sdk.SessionClient = (*sessionClient)(nil)

// sessionClient is a SessionClient implementation that uses the gRPC query client
// of the session module.
type sessionClient struct {
	queryClient types.QueryClient
}

// NewSessionClient creates a new session client with the provided gRPC connection.
func NewSessionClient(grpcConn grpc.ClientConn) sdk.SessionClient {
	return &sessionClient{
		queryClient: types.NewQueryClient(grpcConn),
	}
}

// GetSession returns the session with the given application address, service id and height.
func (s *sessionClient) GetSession(
	ctx context.Context,
	appAddress string,
	serviceId string,
	height int64,
) (session *types.Session, err error) {
	req := &types.QueryGetSessionRequest{
		ApplicationAddress: appAddress,
		Service:            &sharedtypes.Service{Id: serviceId},
		BlockHeight:        height,
	}
	res, err := s.queryClient.GetSession(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.Session, nil
}
