package client

import (
	"context"
	"fmt"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/pokt-network/poktroll/pkg/polylog"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	"golang.org/x/exp/slices"

	sdk "github.com/pokt-network/shannon-sdk"
)

// GatewayClient contains functionality needed to sign
// and validate relays on the Shannon protocol.
//
// It is used to:
//   - Fetch onchain data for the Shannon protocol integration
//   - Sign relays using ring signatures (with the gateway's private key)
//   - Validate relay responses
//
// It contains:
//   - A full node for fetching onchain data (caching or just-in-time)
//   - A relay signer for signing relays using ring signatures
//   - The gateway address
type GatewayClient struct {
	shannonFullNode
	*relaySigner
	gatewayAddress string
}

func NewGatewayClient(
	logger polylog.Logger,
	fullNodeConfig FullNodeConfig,
	gatewayAddress string,
	gatewayPrivateKeyHex string,
) (*GatewayClient, error) {
	fullNode, err := getFullNode(logger, fullNodeConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Shannon full node: %v", err)
	}

	relaySigner := &relaySigner{
		PrivateKeyHex:    gatewayPrivateKeyHex,
		PublicKeyFetcher: fullNode,
	}

	return &GatewayClient{
		shannonFullNode: fullNode,
		relaySigner:     relaySigner,
		gatewayAddress:  gatewayAddress,
	}, nil
}

// FullNode is the interface that wraps the basic methods used to interface with the Shannon full node.
// It is used to fetch onchain data for the Shannon protocol integration.
//
// It is implemented by the structs:
//   - fullNode: default implementation of a full node for the Shannon.
//   - fullNodeWithCache: the default full node with a SturdyC-based cache.
type shannonFullNode interface {
	// GetApp returns the onchain application matching the application address
	GetApp(ctx context.Context, appAddr string) (*apptypes.Application, error)

	// GetSession returns the latest session matching the supplied service+app combination.
	// Sessions are solely used for sending relays, and therefore only the latest session for any service+app combination is needed.
	// Note: Shannon returns the latest session for a service+app combination if no blockHeight is provided.
	GetSession(ctx context.Context, serviceID sdk.ServiceID, appAddr string) (sessiontypes.Session, error)

	// getAccountPubKey returns the account public key for the given address.
	// The cache has no TTL, so the public key is cached indefinitely.
	getAccountPubKey(ctx context.Context, address string) (cryptotypes.PubKey, error)

	// IsHealthy returns true if the full node is healthy.
	IsHealthy() bool
}

// getFullNode builds and returns a FullNode implementation for Shannon protocol integration.
//
// It may return a `fullNode` or a `fullNodeWithCache` depending on the caching configuration.
func getFullNode(logger polylog.Logger, config FullNodeConfig) (shannonFullNode, error) {

	// In both lazy and caching modes, we use the full node to fetch the onchain data.
	fullNode, err := newFullNode(logger, config.RpcURL, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create Shannon full node: %v", err)
	}

	// If caching is disabled, return the full node directly.
	if !config.CacheConfig.CachingEnabled {
		return fullNode, nil
	}

	// Hydrate the cache configuration with defaults if cache is enabled.
	config.CacheConfig.hydrateDefaults()

	// If caching is enabled, return the full node with cache.
	return newFullNodeWithCache(logger, fullNode, config.CacheConfig.SessionTTL)
}

// GetActiveSessions retrieves active sessions for a list of app addresses and a service ID.
// It verifies that each app delegates to the gateway and is staked for the requested service.
// This method encapsulates the shared logic between centralized and delegated gateway modes.
func (c *GatewayClient) GetActiveSessions(
	ctx context.Context,
	serviceID sdk.ServiceID,
	appAddresses []string,
) ([]sessiontypes.Session, error) {
	if len(appAddresses) == 0 {
		return nil, fmt.Errorf("no app addresses provided for service %s", serviceID)
	}

	var activeSessions []sessiontypes.Session

	for _, appAddr := range appAddresses {
		// Retrieve the session for the app
		session, err := c.GetSession(ctx, serviceID, appAddr)
		if err != nil {
			return nil, fmt.Errorf("%w: app: %s, error: %w",
				ErrProtocolContextSetupCentralizedAppFetchErr,
				appAddr,
				err,
			)
		}

		app := session.Application

		// Verify the app is staked for the requested service
		if !appIsStakedForService(serviceID, app) {
			return nil, fmt.Errorf("%w: app: %s",
				ErrProtocolContextSetupAppNotStaked,
				app.Address,
			)
		}

		// Verify the app delegates to the gateway
		if !c.gatewayHasDelegationForApp(app) {
			return nil, fmt.Errorf("%w: gateway: %s, app: %s",
				ErrProtocolContextSetupAppDoesNotDelegate,
				c.gatewayAddress,
				app.Address,
			)
		}

		activeSessions = append(activeSessions, session)
	}

	if len(activeSessions) == 0 {
		return nil, fmt.Errorf("%w: service %s",
			ErrProtocolContextSetupCentralizedNoSessions,
			serviceID,
		)
	}

	return activeSessions, nil
}

// gatewayHasDelegationForApp checks if the gateway has delegation for the application.
func (c *GatewayClient) gatewayHasDelegationForApp(app *apptypes.Application) bool {
	return slices.Contains(app.DelegateeGatewayAddresses, c.gatewayAddress)
}

// appIsStakedForService checks if the application is staked for the specified service.
func appIsStakedForService(serviceID sdk.ServiceID, app *apptypes.Application) bool {
	for _, serviceConfig := range app.GetServiceConfigs() {
		if serviceConfig.GetServiceId() == string(serviceID) {
			return true
		}
	}
	return false
}

// ValidateRelayResponse validates the RelayResponse and verifies the supplier's signature.
//
// - Returns the RelayResponse, even if basic validation fails (may contain error reason).
// - Verifies supplier's signature with the provided publicKeyFetcher.
func (c *GatewayClient) ValidateRelayResponse(
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

	supplierPubKey, err := c.getAccountPubKey(ctx, string(supplierAddress))
	if err != nil {
		return nil, err
	}
	if supplierPubKey == nil {
		return nil, fmt.Errorf("ValidateRelayResponse: supplier public key is nil for address %s", string(supplierAddress))
	}

	if signatureErr := relayResponse.VerifySupplierOperatorSignature(supplierPubKey); signatureErr != nil {
		return nil, signatureErr
	}

	return relayResponse, nil
}
