package session

import (
	"context"

	"github.com/cosmos/gogoproto/grpc"
	"github.com/pokt-network/poktroll/x/session/types"
	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
)

var _ SessionClient = (*sessionClient)(nil)

type sessionClient struct {
	queryClient types.QueryClient
}

func NewSessionClient(grpcConn grpc.ClientConn) SessionClient {
	return &sessionClient{
		queryClient: types.NewQueryClient(grpcConn),
	}
}

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
