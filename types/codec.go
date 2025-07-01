package types

import (
	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	accounttypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// QueryCodec is a package-level codec for unmarshaling account data.
// Initialized globally to avoid expensive repeated setup across fullNode instances.
var QueryCodec *codec.ProtoCodec

// init initializes the codec for the full node in the clientpackage.
func init() {
	reg := cdctypes.NewInterfaceRegistry()
	accounttypes.RegisterInterfaces(reg)
	cryptocodec.RegisterInterfaces(reg)
	QueryCodec = codec.NewProtoCodec(reg)
}
