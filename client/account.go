package client

import (
	"context"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	accounttypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	grpc "github.com/cosmos/gogoproto/grpc"

	"github.com/pokt-network/shannon-sdk/sdk"
)

var (
	_          sdk.AccountClient = (*accountClient)(nil)
	queryCodec *codec.ProtoCodec
)

// init initializes the codec for the account module
func init() {
	reg := codectypes.NewInterfaceRegistry()
	accounttypes.RegisterInterfaces(reg)
	queryCodec = codec.NewProtoCodec(reg)
}

// accountClient is an AccountClient implementation that uses the gRPC query client
// of the account module.
// It is a wrapper around the CosmosSDK account QueryClient.
type accountClient struct {
	queryClient accounttypes.QueryClient
}

// NewAccountClient creates a new account client with the provided gRPC connection.
func NewAccountClient(grpcConn grpc.ClientConn) (sdk.AccountClient, error) {
	return &accountClient{
		accounttypes.NewQueryClient(grpcConn),
	}, nil
}

// GetPubKeyFromAddress returns the public key of the account with the given address.
// It queries the account module using the gRPC query client.
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
