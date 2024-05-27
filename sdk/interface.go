package sdk

import (
	"context"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
)

// AccountClient is the interface that's used by the ShannonSDK to interact with
// the on-chain account module. It's used to get the public key of an account
// from its address.
// The implementations of this interface could leverage caching to avoid querying
// the blockchain for the same account multiple times.
type AccountClient interface {
	// GetPubKeyFromAddress returns the public key of the account with the given address.
	GetPubKeyFromAddress(
		ctx context.Context,
		address string,
	) (pubKey cryptotypes.PubKey, err error)
}

// ApplicationClient is the interface that's used by the ShannonSDK to interact with
// the on-chain application module. It's used to get the list of applications and
// the details of a specific application.
// The implementations of this interface could leverage caching to avoid querying
// the blockchain for the same data multiple times but need to invalidate it by
// listening to the relevant events such as MsgStakeApplication, MsgUnstakeApplication etc...
type ApplicationClient interface {
	// GetAllApplications returns the list of all applications.
	// TODO_TECHDEBT: Add filtering options to this method once they are supported
	// by the on-chain application module.
	GetAllApplications(
		ctx context.Context,
	) ([]apptypes.Application, error)

	// GetApplication returns the details of the application with the given address.
	GetApplication(
		ctx context.Context,
		appAddress string,
	) (apptypes.Application, error)
}

// BlockClient is the interface that's used by the ShannonSDK to interact with the
// on-chain block module. It's used to get the latest block height.
// The implementations of this interface could leverage websockets to get notified
// about new blocks and cache the latest block height to avoid querying the blockchain
// for it every time.
type BlockClient interface {
	// GetLatestBlockHeight returns the height of the latest block.
	// The height of the latest block is used to get the current session which is
	// used to get the suppliers and their endpoints.
	GetLatestBlockHeight(ctx context.Context) (height int64, err error)
}

// RelayClient is the interface that's used by the ShannonSDK to send relay requests
// to the suppliers.
// It is transport agnostic and could be implemented using the required protocols.
// The implementations of this interface could leverage mechanisms such as retries,
// rate limiting, circuit breaking etc... to ensure the quality of service.
type RelayClient interface {
	// SendRequest sends the relay request to the supplier at the given URL.
	// It accepts the relay request bytes to avoid relying on protocol specific
	// request objects such as http.Request.
	// In the case of HTTP, the method is the HTTP method such as GET, POST etc...
	SendRequest(
		ctx context.Context,
		supplierUrl string,
		relayRequestBz []byte,
		method string,
		requestHeaders map[string][]string,
	) (relayResponseBz []byte, err error)
}

// SessionClient is the interface that's used by the ShannonSDK to interact with the
// on-chain session module. It's used to get the current session for a given application
// and service id at a given height.
type SessionClient interface {
	// GetSession returns the current session for the given application address and
	// service id at the given height.
	GetSession(
		ctx context.Context,
		appAddress string,
		serviceId string,
		height int64,
	) (session *sessiontypes.Session, err error)
}

// Signer is the interface that's used by the ShannonSDK to retrieve the private key
// of the account that's used to sign the relay requests.
// The implementations of this interface could leverage hardware wallets, secure
// enclaves etc... to store the private key securely.
type Signer interface {
	// GetPrivateKeyHex returns the private key bytes of the account in hex format.
	GetPrivateKeyHex() string
}
