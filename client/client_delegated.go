package client

import (
	"context"
	"fmt"
	"net/http"

	"github.com/pokt-network/poktroll/pkg/polylog"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sdk "github.com/pokt-network/shannon-sdk"
)

// DelegatedGatewayClient implements the GatewayClient interface for delegated gateway mode.
// In delegated mode:
//   - Each relay request is signed by the gateway key and sent on behalf of an app selected by the user.
//   - Users must select a specific app for each relay request (currently via HTTP request headers).
type DelegatedGatewayClient struct {
	logger polylog.Logger

	sdk.FullNode

	gatewayAddr          string
	gatewayPrivateKeyHex string
}

// httpHeaderAppAddress is the HTTP header name for specifying the target application address.
const httpHeaderAppAddress = "X-App-Address"

// NewDelegatedGatewayClient creates a new DelegatedGatewayClient instance.
func NewDelegatedGatewayClient(
	fullNode sdk.FullNode,
	logger polylog.Logger,
	config GatewayConfig,
) (*DelegatedGatewayClient, error) {
	logger = logger.With("client_type", "delegated")

	return &DelegatedGatewayClient{
		FullNode:             fullNode,
		logger:               logger,
		gatewayAddr:          config.GatewayAddress,
		gatewayPrivateKeyHex: config.GatewayPrivateKeyHex,
	}, nil
}

// GetSessions implements GatewayClient interface.
// Returns the permitted session under Delegated gateway mode, for the supplied HTTP request.
func (d *DelegatedGatewayClient) GetSessions(
	ctx context.Context,
	serviceID sdk.ServiceID,
	httpReq *http.Request,
) ([]sessiontypes.Session, error) {
	logger := d.logger.With("method", "GetSessions")

	selectedAppAddr, err := getAppAddrFromHTTPReq(httpReq)
	if err != nil {
		err = fmt.Errorf("failed to get app address from HTTP request: %w", err)
		logger.Error().Err(err).Msg("error getting the app address from the HTTP request. Relay request will fail.")
		return nil, err
	}

	logger.Debug().Msgf("fetching the app with the selected address %s.", selectedAppAddr)

	selectedSession, err := d.FullNode.GetSession(ctx, serviceID, selectedAppAddr)
	if err != nil {
		err = fmt.Errorf("failed to fetch app %s: %w. Relay request will fail.", selectedAppAddr, err)
		logger.Error().Err(err).Msg("error fetching the app. Relay request will fail.")
		return nil, err
	}

	selectedApp := selectedSession.Application

	logger.Debug().Msgf("fetched the app with the selected address %s.", selectedApp.Address)

	// Skip the session's app if it is not staked for the requested service.
	if !appIsStakedForService(serviceID, selectedApp) {
		err = fmt.Errorf("app %s is not staked for the service", selectedApp.Address)
		logger.Error().Err(err).Msg("app is not staked for the service. Relay request will fail.")
		return nil, err
	}

	if !gatewayHasDelegationForApp(d.gatewayAddr, selectedApp) {
		err = fmt.Errorf("gateway %s does not have delegation for app %s. Relay request will fail.", d.gatewayAddr, selectedApp.Address)
		logger.Error().Err(err).Msg("Gateway does not have delegation for the app. Relay request will fail.")
		return nil, err
	}

	logger.Debug().Msgf("successfully verified the gateway has delegation for the selected app with address %s.", selectedApp.Address)

	return []sessiontypes.Session{selectedSession}, nil
}

// GetRelaySigner implements GatewayClient interface.
// Returns the relay request signer for delegated mode.
func (d *DelegatedGatewayClient) GetRelaySigner(ctx context.Context, serviceID sdk.ServiceID, httpReq *http.Request) (*sdk.Signer, error) {
	return &sdk.Signer{
		PrivateKeyHex:    d.gatewayPrivateKeyHex,
		PublicKeyFetcher: d.FullNode,
	}, nil
}

// GetConfiguredServiceIDs is a no-op for delegated mode because
// the service an app is staked for is known only at request time.
func (d *DelegatedGatewayClient) GetConfiguredServiceIDs() map[sdk.ServiceID]struct{} {
	return nil
}

// appIsStakedForService returns true if the supplied application is staked for the supplied service ID.
func appIsStakedForService(serviceID sdk.ServiceID, app *apptypes.Application) bool {
	for _, svcCfg := range app.ServiceConfigs {
		if sdk.ServiceID(svcCfg.ServiceId) == serviceID {
			return true
		}
	}
	return false
}

// getAppAddrFromHTTPReq extracts the application address specified by the supplied HTTP request's headers.
func getAppAddrFromHTTPReq(httpReq *http.Request) (string, error) {
	if httpReq == nil || len(httpReq.Header) == 0 {
		return "", fmt.Errorf("getAppAddrFromHTTPReq: no HTTP headers supplied")
	}

	selectedAppAddr := httpReq.Header.Get(httpHeaderAppAddress)
	if selectedAppAddr == "" {
		return "", fmt.Errorf("getAppAddrFromHTTPReq: a target app must be supplied as HTTP header %s", httpHeaderAppAddress)
	}

	return selectedAppAddr, nil
}
