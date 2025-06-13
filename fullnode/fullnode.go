package fullnode

import (
	"context"
	"fmt"
	"net/url"

	"github.com/cosmos/cosmos-sdk/codec"
	cdctypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	accounttypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"

	sdk "github.com/pokt-network/shannon-sdk"
)

// queryCodec is a package-level codec for unmarshaling account data.
// Initialized globally to avoid expensive repeated setup across fullNode instances.
var queryCodec *codec.ProtoCodec

// init initializes the codec for the fullnode module.
func init() {
	reg := cdctypes.NewInterfaceRegistry()
	accounttypes.RegisterInterfaces(reg)
	cryptocodec.RegisterInterfaces(reg)
	queryCodec = codec.NewProtoCodec(reg)
}

// fullNode: default implementation of a full node for the Shannon.
//
// A fullNode queries the onchain data for every data item it needs to do an action (e.g. serve a relay request, etc).
//
// This is done to enable supporting short block times (a few seconds), by avoiding caching
// which can result in failures due to stale data in the cache.
//
// Key differences from a caching full node:
// - Intentionally avoids caching:
//   - Enables support for short block times (e.g. LocalNet)
//   - Use CachingFullNode struct if caching is desired for performance
type fullNode struct {
	// logger polylog.Logger

	appClient     *sdk.ApplicationClient
	sessionClient *sdk.SessionClient
	blockClient   *sdk.BlockClient
	accountClient *sdk.AccountClient
}

// NewFullNode builds and returns a fullNode using the provided configuration.
func NewFullNode(config FullNodeConfig) (*fullNode, error) {
	blockClient, err := newBlockClient(config.RpcURL)
	if err != nil {
		return nil, fmt.Errorf("NewSdk: error creating new Shannon block client at URL %s: %w", config.RpcURL, err)
	}

	config.GRPCConfig = config.GRPCConfig.hydrateDefaults()

	sessionClient, err := newSessionClient(config.GRPCConfig)
	if err != nil {
		return nil, fmt.Errorf("NewSdk: error creating new Shannon session client using URL %s: %w", config.GRPCConfig.HostPort, err)
	}

	appClient, err := newAppClient(config.GRPCConfig)
	if err != nil {
		return nil, fmt.Errorf("NewSdk: error creating new GRPC connection at url %s: %w", config.GRPCConfig.HostPort, err)
	}

	accountClient, err := newAccClient(config.GRPCConfig)
	if err != nil {
		return nil, fmt.Errorf("NewSdk: error creating new account client using url %s: %w", config.GRPCConfig.HostPort, err)
	}

	fullNode := &fullNode{
		sessionClient: sessionClient,
		appClient:     appClient,
		blockClient:   blockClient,
		accountClient: accountClient,
	}

	return fullNode, nil
}

// GetApp:
// - Returns the onchain application matching the supplied application address.
// - Required to fulfill the FullNode interface.
func (lfn *fullNode) GetApp(ctx context.Context, appAddr string) (*apptypes.Application, error) {
	app, err := lfn.appClient.GetApplication(ctx, appAddr)
	return &app, err
}

// GetSession:
// - Uses the Shannon SDK to fetch a session for the (serviceID, appAddr) combination.
// - Required to fulfill the FullNode interface.
func (lfn *fullNode) GetSession(
	ctx context.Context,
	serviceID sdk.ServiceID,
	appAddr string,
) (sessiontypes.Session, error) {
	session, err := lfn.sessionClient.GetSession(
		ctx,
		appAddr,
		string(serviceID),
		0,
	)

	if err != nil {
		return sessiontypes.Session{},
			fmt.Errorf("GetSession: error getting the session for service %s app %s: %w",
				serviceID, appAddr, err,
			)
	}

	if session == nil {
		return sessiontypes.Session{},
			fmt.Errorf("GetSession: got nil session for service %s app %s: %w",
				serviceID, appAddr, err,
			)
	}

	return *session, nil
}

// ValidateRelayResponse validates the RelayResponse and verifies the supplier's signature.
//
// - Returns the RelayResponse, even if basic validation fails (may contain error reason).
// - Verifies supplier's signature with the provided publicKeyFetcher.
func (lfn *fullNode) ValidateRelayResponse(
	ctx context.Context,
	supplierAddress sdk.SupplierAddress,
	relayResponseBz []byte,
) (*servicetypes.RelayResponse, error) {
	relayResponse := &servicetypes.RelayResponse{}
	if err := relayResponse.Unmarshal(relayResponseBz); err != nil {
		return nil, err
	}

	if err := relayResponse.ValidateBasic(); err != nil {
		// Even if the relay response is invalid, return it (may contain failure reason)
		return relayResponse, err
	}

	supplierPubKey, err := lfn.GetAccountPubKey(
		ctx,
		string(supplierAddress),
	)
	if err != nil {
		return nil, err
	}

	// This can happen if a supplier has never been used (e.g. funded) onchain
	if supplierPubKey == nil {
		return nil, fmt.Errorf("ValidateRelayResponse: supplier public key is nil for address %s", string(supplierAddress))
	}

	if signatureErr := relayResponse.VerifySupplierOperatorSignature(supplierPubKey); signatureErr != nil {
		return nil, signatureErr
	}

	return relayResponse, nil
}

// IsHealthy:
// - Always returns true for a fullNode.
// - Required to fulfill the FullNode interface.
func (lfn *fullNode) IsHealthy() bool {
	return true
}

// GetAccountPubKey returns the public key of the account with the given address.
//
// - Queries the account module using the gRPC query client.
func (lfn *fullNode) GetAccountPubKey(
	ctx context.Context,
	address string,
) (pubKey cryptotypes.PubKey, err error) {

	req := &accounttypes.QueryAccountRequest{Address: address}
	res, err := lfn.accountClient.Account(ctx, req)
	if err != nil {
		return nil, err
	}

	var fetchedAccount types.AccountI
	if err = queryCodec.UnpackAny(res.Account, &fetchedAccount); err != nil {
		return nil, err
	}

	return fetchedAccount.GetPubKey(), nil
}

func newSessionClient(config GRPCConfig) (*sdk.SessionClient, error) {
	conn, err := connectGRPC(config)
	if err != nil {
		return nil, fmt.Errorf("could not create new Shannon session client: error establishing grpc connection to %s: %w", config.HostPort, err)
	}

	return &sdk.SessionClient{PoktNodeSessionFetcher: sdk.NewPoktNodeSessionFetcher(conn)}, nil
}

func newBlockClient(fullNodeURL string) (*sdk.BlockClient, error) {
	_, err := url.Parse(fullNodeURL)
	if err != nil {
		return nil, fmt.Errorf("newBlockClient: error parsing url %s: %w", fullNodeURL, err)
	}

	nodeStatusFetcher, err := sdk.NewPoktNodeStatusFetcher(fullNodeURL)
	if err != nil {
		return nil, fmt.Errorf("newBlockClient: error connecting to a full node %s: %w", fullNodeURL, err)
	}

	return &sdk.BlockClient{PoktNodeStatusFetcher: nodeStatusFetcher}, nil
}

func newAppClient(config GRPCConfig) (*sdk.ApplicationClient, error) {
	appConn, err := connectGRPC(config)
	if err != nil {
		return nil, fmt.Errorf("NewSdk: error creating new GRPC connection at url %s: %w", config.HostPort, err)
	}

	return &sdk.ApplicationClient{QueryClient: apptypes.NewQueryClient(appConn)}, nil
}

func newAccClient(config GRPCConfig) (*sdk.AccountClient, error) {
	conn, err := connectGRPC(config)
	if err != nil {
		return nil, fmt.Errorf("newAccClient: error creating new GRPC connection for account client at url %s: %w", config.HostPort, err)
	}

	return &sdk.AccountClient{PoktNodeAccountFetcher: sdk.NewPoktNodeAccountFetcher(conn)}, nil
}
