package sdk

import (
	"context"

	"github.com/cosmos/gogoproto/grpc"
	"github.com/pokt-network/poktroll/x/session/types"
	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
	grpcoptions "google.golang.org/grpc"
)

// SessionClient is the interface to interact with the on-chain session module.
//
// For example, it can be used to get the current session for a given application
// and service id at a given height.
type SessionClient struct {
	SessionFetcher
}

// SessionFetcher is used by the SessionClient to fetch sessions using poktroll request/response types.
type SessionFetcher interface {
	GetSession(
		context.Context,
		*types.QueryGetSessionRequest,
		...grpcoptions.CallOption,
	) (*types.QueryGetSessionResponse, error)
}

// NewSessionClient creates a new session client with the provided gRPC connection.
//
// It builds an instance of SessionClient that utilizes the gRPC query client of the session module.
func NewSessionClient(grpcConn grpc.ClientConn) *SessionClient {
	return &SessionClient{
		SessionFetcher: types.NewQueryClient(grpcConn),
	}
}

// GetSession returns the session with the given application address, service id and height.
func (s *SessionClient) GetSession(
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
	res, err := s.SessionFetcher.GetSession(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.Session, nil
}
