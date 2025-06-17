package client

import (
	"context"
	"fmt"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/pokt-network/poktroll/pkg/polylog"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"

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
	FullNode
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
		FullNode:       fullNode,
		relaySigner:    relaySigner,
		gatewayAddress: gatewayAddress,
	}, nil
}

// FullNode is the interface that wraps the basic methods used to interface with the Shannon full node.
// It is used to fetch onchain data for the Shannon protocol integration.
//
// It is implemented by the structs:
//   - fullNode: default implementation of a full node for the Shannon.
//   - fullNodeWithCache: the default full node with a SturdyC-based cache.
type FullNode interface {
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
func getFullNode(logger polylog.Logger, config FullNodeConfig) (FullNode, error) {

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

func (c *GatewayClient) GetGatewayAddress() string {
	return c.gatewayAddress
}
