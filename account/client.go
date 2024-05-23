package account

import (
	"context"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	accounttypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	grpc "github.com/cosmos/gogoproto/grpc"
)

var (
	_          AccountClient = (*accountClient)(nil)
	queryCodec *codec.ProtoCodec
)

func init() {
	reg := codectypes.NewInterfaceRegistry()
	accounttypes.RegisterInterfaces(reg)
	queryCodec = codec.NewProtoCodec(reg)
}

type accountClient struct {
	queryClient accounttypes.QueryClient
}

func NewAccountClient(grpcConn grpc.ClientConn) (AccountClient, error) {
	return &accountClient{
		accounttypes.NewQueryClient(grpcConn),
	}, nil
}

func (ac *accountClient) GetPubKeyFromAddress(
	ctx context.Context,
	address string,
) (pubKey cryptotypes.PubKey, err error) {
	req := &accounttypes.QueryAccountRequest{Address: address}
	res, err := ac.queryClient.Account(ctx, req)
	if err != nil {
		return nil, err
	}

	var fetchedAccount types.AccountI
	if err = queryCodec.UnpackAny(res.Account, &fetchedAccount); err != nil {
		return nil, err
	}

	return fetchedAccount.GetPubKey(), nil
}
