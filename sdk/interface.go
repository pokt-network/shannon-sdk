package sdk

import (
	"context"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
)

type AccountClient interface {
	GetPubKeyFromAddress(
		ctx context.Context,
		address string,
	) (pubKey cryptotypes.PubKey, err error)
}

type ApplicationClient interface {
	GetAllApplications(
		ctx context.Context,
	) ([]apptypes.Application, error)

	GetApplication(
		ctx context.Context,
		appAddress string,
	) (apptypes.Application, error)
}

type BlockClient interface {
	GetLatestBlockHeight(ctx context.Context) (height int64, err error)
}

type RelayClient interface {
	Do(
		ctx context.Context,
		supplierUrl string,
		relayRequestBz []byte,
		method string,
		requestHeaders map[string][]string,
	) (relayResponseBz []byte, err error)
}

type SessionClient interface {
	GetSession(
		ctx context.Context,
		appAddress string,
		serviceId string,
		height int64,
	) (session *sessiontypes.Session, err error)
}

type Signer interface {
	GetPrivateKeyHex() string
}
