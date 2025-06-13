package sdk

import (
	"context"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

// PublicKeyFetcher specifies an interface that allows getting the public
// key corresponding to an address.
//
// - Used by the ApplicationRing struct to construct the Application's Ring for signing relay requests
//
// Implements sdk.PublicKeyFetcher interface.
type PublicKeyFetcher interface {
	GetAccountPubKey(context.Context, string) (cryptotypes.PubKey, error)
}
