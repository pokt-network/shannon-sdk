package account

import (
	"context"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

type AccountClient interface {
	GetPubKeyFromAddress(
		ctx context.Context,
		address string,
	) (pubKey cryptotypes.PubKey, err error)
}
