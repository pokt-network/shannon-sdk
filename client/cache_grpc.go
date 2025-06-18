package client

import (
	"crypto/tls"
	"fmt"
	"net/url"

	apptypes "github.com/pokt-network/poktroll/x/application/types"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	sdk "github.com/pokt-network/shannon-sdk"
)

// connectGRPC creates a new gRPC connection.
// Backoff configuration may be customized using the config YAML fields
// under `grpc_config`. TLS is enabled by default, unless overridden by
// the `grpc_config.insecure` field.
// TODO_TECHDEBT: use an enhanced grpc connection with reconnect logic.
// All GRPC settings have been disabled to focus the E2E tests on the
// gateway functionality rather than GRPC settings.
func connectGRPC(hostPort string, useInsecure bool) (*grpc.ClientConn, error) {
	if useInsecure {
		transport := grpc.WithTransportCredentials(insecure.NewCredentials())
		dialOptions := []grpc.DialOption{transport}
		return grpc.NewClient(
			hostPort,
			dialOptions...,
		)
	}

	// TODO_TECHDEBT: make the necessary changes to allow using grpc.NewClient here.
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
