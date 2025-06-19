package client

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/url"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	accounttypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/pokt-network/poktroll/pkg/polylog"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sdkTypes "github.com/pokt-network/shannon-sdk/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	sdk "github.com/pokt-network/shannon-sdk"
)

// GRPCClient implements OnchainDataFetcher interface.
var _ OnchainDataFetcher = &GRPCClient{}

// GRPCClient is a client used to fetch onchain data
// from the Shannon protocol over a gRPC connection.
type GRPCClient struct {
	logger polylog.Logger

	applicationClient *sdk.ApplicationClient
	sessionClient     *sdk.SessionClient
	accountClient     *sdk.AccountClient
	blockClient       *sdk.BlockClient
}

func NewGRPCClient(logger polylog.Logger, grpcConfig GRPCConfig) (*GRPCClient, error) {
	logger = logger.With("client", "grpc_client")

	// Connect to the full node
	grpcConn, err := connectGRPC(
		grpcConfig.HostPort,
		grpcConfig.UseInsecureGRPCConn,
	)
	if err != nil {
		return nil, fmt.Errorf("NewGatewayClientCache: error creating new GRPC connection at url %s: %w",
			grpcConfig.HostPort, err)
	}

	// Create the block client
	blockClient, err := newBlockClient(grpcConfig.RpcURL)
	if err != nil {
		return nil, fmt.Errorf("NewGatewayClientCache: error creating new Shannon block client at URL %s: %w", grpcConfig.RpcURL, err)
	}

	return &GRPCClient{
		applicationClient: newAppClient(grpcConn),
		sessionClient:     newSessionClient(grpcConn),
		accountClient:     newAccClient(grpcConn),
		blockClient:       blockClient,
	}, nil
}

// GetApp fetches an application from the full node.
//
// - Uses the GRPCClient's applicationClient to fetch an application from the full node.
func (g *GRPCClient) GetApp(ctx context.Context, appAddr string) (apptypes.Application, error) {
	app, err := g.applicationClient.GetApplication(ctx, appAddr)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrGRPCClientGetAppError, err)
		g.logger.Error().Msg(err.Error())
		return apptypes.Application{}, err
	}

	return app, nil
}

// GetSession fetches a session for the (serviceID, appAddr) combination.
//
// - Uses the GRPCClient's sessionClient to fetch a session for the (serviceID, appAddr) combination.
func (g *GRPCClient) GetSession(
	ctx context.Context,
	serviceID sdk.ServiceID,
	appAddr string,
) (sessiontypes.Session, error) {
	logger := g.logger.With(
		"method", "GetSession",
		"service_id", serviceID,
		"app_addr", appAddr,
	)

	session, err := g.sessionClient.GetSession(
		ctx,
		appAddr,
		string(serviceID),
		0,
	)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrGRPCClientGetSessionError, err)
		logger.Error().Msg(err.Error())
		return sessiontypes.Session{}, err
	}
	if session == nil {
		err = fmt.Errorf("%w: %w", ErrGRPCClientGetSessionNilSession, err)
		logger.Error().Msg(err.Error())
		return sessiontypes.Session{}, err
	}

	return *session, nil
}

// GetAccountPubKey returns the public key of the account with the given address.
//
// - Uses the GRPCClient's accountClient to query the account module using the gRPC query client.
func (g *GRPCClient) GetAccountPubKey(
	ctx context.Context,
	address string,
) (pubKey cryptotypes.PubKey, err error) {
	logger := g.logger.With(
		"method", "GetAccountPubKey",
		"address", address,
	)

	req := &accounttypes.QueryAccountRequest{Address: address}

	res, err := g.accountClient.Account(ctx, req)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrGRPCClientGetAccountPubKeyError, err)
		logger.Error().Msg(err.Error())
		return nil, err
	}

	var fetchedAccount types.AccountI
	if err = sdkTypes.QueryCodec.UnpackAny(res.Account, &fetchedAccount); err != nil {
		err = fmt.Errorf("%w: %w", ErrGRPCClientGetAccountPubKeyError, err)
		logger.Error().Msg(err.Error())
		return nil, err
	}

	return fetchedAccount.GetPubKey(), nil
}

// connectGRPC creates a new gRPC connection.
//
// TLS is enabled by default, unless overridden by the `grpc_config.insecure` field.
//
// TODO_TECHDEBT: use an enhanced grpc connection with reconnect logic.
func connectGRPC(hostPort string, useInsecure bool) (*grpc.ClientConn, error) {
	if useInsecure {
		transport := grpc.WithTransportCredentials(insecure.NewCredentials())
		dialOptions := []grpc.DialOption{transport}
		return grpc.NewClient(
			hostPort,
			dialOptions...,
		)
	}

	// TODO_TECHDEBT(@commoddity): make the necessary changes to allow using grpc.NewClient here.
	// Currently using the grpc.NewClient method fails the E2E tests.
	return grpc.Dial( //nolint:all
		hostPort,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	)
}

// newSessionClient creates a new session client used by GatewayClientCache to fetch sessions from the full node
// Uses a gRPC connection to the full node.
func newSessionClient(conn *grpc.ClientConn) *sdk.SessionClient {
	return &sdk.SessionClient{PoktNodeSessionFetcher: sdk.NewPoktNodeSessionFetcher(conn)}
}

// newAppClient creates a new application client used by GatewayClientCache to fetch applications from the full node
// Uses a gRPC connection to the full node.
func newAppClient(conn *grpc.ClientConn) *sdk.ApplicationClient {
	return &sdk.ApplicationClient{QueryClient: apptypes.NewQueryClient(conn)}
}

// newAccClient creates a new account client used by GatewayClientCache to fetch accounts from the full node
// Uses a gRPC connection to the full node.
func newAccClient(conn *grpc.ClientConn) *sdk.AccountClient {
	return &sdk.AccountClient{PoktNodeAccountFetcher: sdk.NewPoktNodeAccountFetcher(conn)}
}

// newBlockClient creates a new block client used by GatewayClientCache to fetch block information from the full node
// Uses an RPC request to the full node.
func newBlockClient(rpcURL string) (*sdk.BlockClient, error) {
	_, err := url.Parse(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("newBlockClient: error parsing url %s: %w", rpcURL, err)
	}

	nodeStatusFetcher, err := sdk.NewPoktNodeStatusFetcher(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("newBlockClient: error connecting to a full node %s: %w", rpcURL, err)
	}

	return &sdk.BlockClient{PoktNodeStatusFetcher: nodeStatusFetcher}, nil
}

// IsHealthy satisfies the interface required by the ShannonFullNode interface.
// TODO_IMPROVE(@commoddity):
//   - Add smarter health checks (e.g. verify cached apps/sessions)
//   - Currently always true (cache fills as needed)
func (g *GRPCClient) IsHealthy() bool {
	return true
}
