package client

import (
	"context"
	"fmt"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	accounttypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/pokt-network/poktroll/pkg/polylog"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"

	sdk "github.com/pokt-network/shannon-sdk"
	sdkTypes "github.com/pokt-network/shannon-sdk/types"
)

// TODO_IN_THIS_PR(@commoddity): start a "micro readme" (just with bullet points) that captures more details about full node implementations?

// fullNode: default implementation of a full node for the Shannon.
//
// A fullNode queries the onchain data for every data item it needs to do an action (e.g. serve a relay request, etc).
//
// This is done to enable supporting short block times (a few seconds), by avoiding caching
// which can result in failures due to stale data in the cache.
//
// Key differences from a caching full node:
//   - Intentionally avoids caching:
//   - Enables support for short block times (e.g. LocalNet)
//   - Use CachingFullNode struct if caching is desired for performance
//
// Implements the FullNode interface.
type fullNode struct {
	logger polylog.Logger

	appClient     *sdk.ApplicationClient
	sessionClient *sdk.SessionClient
	blockClient   *sdk.BlockClient
	accountClient *sdk.AccountClient
}

// newFullNode builds and returns a fullNode using the provided configuration.
func newFullNode(logger polylog.Logger, rpcURL string, fullNodeConfig FullNodeConfig) (*fullNode, error) {
	grpcConn, err := connectGRPC(
		fullNodeConfig.GRPCConfig.HostPort,
		fullNodeConfig.GRPCConfig.UseInsecureGRPCConn,
	)
	if err != nil {
		return nil, fmt.Errorf("NewFullNode: error creating new GRPC connection at url %s: %w",
			fullNodeConfig.GRPCConfig.HostPort, err)
	}

	blockClient, err := newBlockClient(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("NewFullNode: error creating new Shannon block client at URL %s: %w", rpcURL, err)
	}

	fullNode := &fullNode{
		logger:        logger,
		sessionClient: newSessionClient(grpcConn),
		appClient:     newAppClient(grpcConn),
		accountClient: newAccClient(grpcConn),
		blockClient:   blockClient,
	}

	return fullNode, nil
}

// GetApp:
// - Returns the onchain application matching the supplied application address.
// - Required to fulfill the FullNode interface.
func (fn *fullNode) GetApp(ctx context.Context, appAddr string) (*apptypes.Application, error) {
	app, err := fn.appClient.GetApplication(ctx, appAddr)
	if err != nil {
		return nil, fmt.Errorf("GetApp: error getting the application for address %s: %w", appAddr, err)
	}

	fn.logger.Debug().Msgf("GetApp: fetched application %s", app)

	return &app, err
}

// GetSession:
// - Uses the session client to fetch a session for the (serviceID, appAddr) combination.
// - Required to fulfill the FullNode interface.
func (fn *fullNode) GetSession(
	ctx context.Context,
	serviceID sdk.ServiceID,
	appAddr string,
) (sessiontypes.Session, error) {
	session, err := fn.sessionClient.GetSession(
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

	fn.logger.Debug().Msgf("GetSession: fetched session %s", session)

	return *session, nil
}

// IsHealthy:
// - Always returns true for a fullNode.
// - Required to fulfill the FullNode interface.
func (fn *fullNode) IsHealthy() bool {
	return true
}

// GetAccountPubKey returns the public key of the account with the given address.
//
// - Queries the account module using the gRPC query client.
func (fn *fullNode) GetAccountPubKey(
	ctx context.Context,
	address string,
) (pubKey cryptotypes.PubKey, err error) {
	req := &accounttypes.QueryAccountRequest{Address: address}

	res, err := fn.accountClient.Account(ctx, req)
	if err != nil {
		return nil, err
	}

	var fetchedAccount types.AccountI
	if err = sdkTypes.QueryCodec.UnpackAny(res.Account, &fetchedAccount); err != nil {
		return nil, err
	}

	return fetchedAccount.GetPubKey(), nil
}
