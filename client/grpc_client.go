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

// GRPCClient provides direct access to Shannon blockchain data via gRPC.
// This is the underlying data fetcher used by GatewayClientCache for non-cached requests.
type GRPCClient struct {
	logger polylog.Logger

	applicationClient *sdk.ApplicationClient
	sessionClient     *sdk.SessionClient
	accountClient     *sdk.AccountClient
	blockClient       *sdk.BlockClient
}

// NewGRPCClient creates a new gRPC client connected to a Shannon full node.
func NewGRPCClient(logger polylog.Logger, grpcConfig GRPCConfig) (*GRPCClient, error) {
	logger = logger.With("client", "grpc_client")

	// Establish gRPC connection to the full node
	grpcConn, err := connectGRPC(
		grpcConfig.HostPort,
		grpcConfig.UseInsecureGRPCConn,
	)
	if err != nil {
		return nil, fmt.Errorf("NewGRPCClient: error creating gRPC connection to %s: %w",
			grpcConfig.HostPort, err)
	}

	// Create block client for RPC requests
	blockClient, err := newBlockClient(grpcConfig.RpcURL)
	if err != nil {
		return nil, fmt.Errorf("NewGRPCClient: error creating block client for %s: %w", grpcConfig.RpcURL, err)
	}

	return &GRPCClient{
		logger: logger,

		applicationClient: newAppClient(grpcConn),
		sessionClient:     newSessionClient(grpcConn),
		accountClient:     newAccClient(grpcConn),
		blockClient:       blockClient,
	}, nil
}

// GetApp fetches application data directly from the Shannon full node.
func (g *GRPCClient) GetApp(ctx context.Context, appAddr string) (apptypes.Application, error) {
	app, err := g.applicationClient.GetApplication(ctx, appAddr)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrGRPCClientGetAppError, err)
		g.logger.Error().Msg(err.Error())
		return apptypes.Application{}, err
	}

	return app, nil
}

// GetSession fetches a session for the given service and application from the full node.
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

// GetAccountPubKey fetches an account's public key from the full node.
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

// LatestBlockHeight returns the current blockchain height from the full node.
func (g *GRPCClient) LatestBlockHeight(ctx context.Context) (height int64, err error) {
	return g.blockClient.LatestBlockHeight(ctx)
}

// connectGRPC establishes a gRPC connection with optional TLS.
// TLS is enabled by default unless explicitly disabled for local development.
func connectGRPC(hostPort string, useInsecure bool) (*grpc.ClientConn, error) {
	if useInsecure {
		transport := grpc.WithTransportCredentials(insecure.NewCredentials())
		dialOptions := []grpc.DialOption{transport}
		return grpc.NewClient(
			hostPort,
			dialOptions...,
		)
	}

	// TODO_TECHDEBT: Migrate to grpc.NewClient once E2E tests support it
	return grpc.Dial( //nolint:all
		hostPort,
		grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{})),
	)
}

// newSessionClient creates a session client for fetching session data
func newSessionClient(conn *grpc.ClientConn) *sdk.SessionClient {
	return &sdk.SessionClient{PoktNodeSessionFetcher: sdk.NewPoktNodeSessionFetcher(conn)}
}

// newAppClient creates an application client for fetching application data
func newAppClient(conn *grpc.ClientConn) *sdk.ApplicationClient {
	return &sdk.ApplicationClient{QueryClient: apptypes.NewQueryClient(conn)}
}

// newAccClient creates an account client for fetching account data
func newAccClient(conn *grpc.ClientConn) *sdk.AccountClient {
	return &sdk.AccountClient{PoktNodeAccountFetcher: sdk.NewPoktNodeAccountFetcher(conn)}
}

// newBlockClient creates a block client for fetching blockchain height via RPC
func newBlockClient(rpcURL string) (*sdk.BlockClient, error) {
	_, err := url.Parse(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("newBlockClient: invalid RPC URL %s: %w", rpcURL, err)
	}

	nodeStatusFetcher, err := sdk.NewPoktNodeStatusFetcher(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("newBlockClient: error connecting to full node %s: %w", rpcURL, err)
	}

	return &sdk.BlockClient{PoktNodeStatusFetcher: nodeStatusFetcher}, nil
}

// IsHealthy reports the health status of the gRPC client.
// Currently always returns true as connections are established on-demand.
//
// TODO_IMPROVE: Add meaningful health checks:
//   - Test gRPC connection connectivity
//   - Verify recent successful requests
//   - Check full node sync status
func (g *GRPCClient) IsHealthy() bool {
	return true
}
