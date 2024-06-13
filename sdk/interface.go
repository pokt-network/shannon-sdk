package sdk

import (
	"context"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
)

// AccountClient is the interface to interact with the on-chain account module.
//
// For example, it can be used to get the public key of an account from its address.
//
// The implementations of this interface could leverage caching to avoid querying
// the blockchain for the same account multiple times.
type AccountClient interface {
	// GetPubKeyFromAddress returns the public key of the account with the given address.
	GetPubKeyFromAddress(
		ctx context.Context,
		address string,
	) (pubKey cryptotypes.PubKey, err error)
}

// SharedParamsClient is the interface to interact with the on-chain shared module.
//
// For example, it can be used to get the number of blocks per session.
//
// The implementations of this interface could leverage caching to avoid querying
// the blockchain for the same data multiple times but need to invalidate it by
// listening to the relevant events.
type SharedParamsClient interface {
	GetParams(ctx context.Context) (params *sharedtypes.Params, err error)
}

// BlockClient is the interface to interact with the on-chain block module.
//
// For example, it can be used to get the latest block height.
//
// The implementations of this interface could leverage websockets to get notified
// about new blocks and cache the latest block height to avoid querying the blockchain
// for it every time.
type BlockClient interface {
	// GetLatestBlockHeight returns the height of the latest block.
	GetLatestBlockHeight(ctx context.Context) (height int64, err error)
}

// RelayClient is the interface used to send Relays to suppliers.
//
// It is transport agnostic and could be implemented using the required protocols.
//
// The implementations of this interface could leverage mechanisms such as retries,
// rate limiting, circuit breaking etc. to ensure quality of service.
type RelayClient interface {
	// SendRequest sends a relay request to the supplier at the given URL.
	// It accepts the relay request bytes to avoid relying on protocol specific
	// request objects such as http.Request.
	SendRequest(
		ctx context.Context,
		supplierUrl string,
		relayRequestBz []byte,
	) (relayResponseBz []byte, err error)
}

// SessionClient is the interface to interact with the on-chain session module.

// For example, it can be used to get the current session for a given application
// and service id at a given height.
type SessionClient interface {
	// GetSession returns the current session for the given application address
	// and service id at the given height.
	GetSession(
		ctx context.Context,
		appAddress string,
		serviceId string,
		height int64,
	) (session *sessiontypes.Session, err error)
}

// Signer is the interface used to interact with private keys.
//
// For example, it can be used to retrieve the private key of the account that's
// used to sign the relay requests.
//
// The implementations of this interface could leverage hardware wallets, secure
// enclaves etc. to store the private key securely.
type Signer interface {
	// GetPrivateKeyHex returns the private key bytes of the account in hex format.
	GetPrivateKeyHex() string
}
