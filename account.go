package sdk

import (
	"context"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	accounttypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	grpc "github.com/cosmos/gogoproto/grpc"
	grpcoptions "google.golang.org/grpc"
)

var queryCodec *codec.ProtoCodec

// init initializes the codec for the account module
func init() {
	reg := cdctypes.NewInterfaceRegistry()
	accounttypes.RegisterInterfaces(reg)
	cryptocodec.RegisterInterfaces(reg)
	queryCodec = codec.NewProtoCodec(reg)
}

// AccountClient is used to interact with the account module.
//
// For example, it can be used to get the public key corresponding to an address.
type AccountClient struct {
	PoktNodeAccountFetcher
}

// GetPubKeyFromAddress returns the public key of the account with the given address.
// It queries the account module using the gRPC query client.
func (ac *AccountClient) GetPubKeyFromAddress(
	ctx context.Context,
	address string,
) (pubKey cryptotypes.PubKey, err error) {
	req := &accounttypes.QueryAccountRequest{Address: address}
	res, err := ac.PoktNodeAccountFetcher.Account(ctx, req)
	if err != nil {
		return nil, err
	}

	var fetchedAccount types.AccountI
	if err = queryCodec.UnpackAny(res.Account, &fetchedAccount); err != nil {
		return nil, err
	}

	return fetchedAccount.GetPubKey(), nil
}

// NewPoktNodeAccountFetcher returns the default implementation of the PoktNodeAccountFetcher interfce.
// It connects to a POKT full node, through the account module's query client, to get account data.
func NewPoktNodeAccountFetcher(grpcConn grpc.ClientConn) PoktNodeAccountFetcher {
	return accounttypes.NewQueryClient(grpcConn)
}

// PoktNodeAccountFetcher is used by the AccountClient to fetch accounts using
// poktroll request/response types.
//
// Most users can rely on the default implementation provided by NewPoktNodeAccountFetcher function.
// A custom implementation of this interface can be used to gain more granular
// control over the interactions of the AccountClient with the POKT full node.
type PoktNodeAccountFetcher interface {
	Account(
		context.Context,
		*accounttypes.QueryAccountRequest,
		...grpcoptions.CallOption,
	) (*accounttypes.QueryAccountResponse, error)
}
