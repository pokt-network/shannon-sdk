package client

import (
	"context"
	"encoding/hex"
	"fmt"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/pokt-network/poktroll/pkg/crypto/rings"
	"github.com/pokt-network/poktroll/pkg/polylog"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	"github.com/pokt-network/ring-go"
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
//   - The gateway address
type GatewayClient struct {
	logger polylog.Logger

	// Embeds the ShannonFullNode interface to fetch onchain data.
	// May be either:
	//   - fullNode: default implementation of a full node for the Shannon protocol.
	//   - fullNodeWithCache: the default full node with a SturdyC-based cache.
	ShannonFullNode

	gatewayAddress       string
	gatewayPrivateKeyHex string
}

// NewGatewayClient builds and returns a GatewayClient using the supplied configuration.
func NewGatewayClient(
	logger polylog.Logger,
	fullNode ShannonFullNode,
	gatewayAddress string,
	gatewayPrivateKeyHex string,
) (*GatewayClient, error) {
	return &GatewayClient{
		logger:               logger,
		ShannonFullNode:      fullNode,
		gatewayAddress:       gatewayAddress,
		gatewayPrivateKeyHex: gatewayPrivateKeyHex,
	}, nil
}

// FullNode is the interface that wraps the basic methods used to interface with the Shannon full node.
// It is used to fetch onchain data for the Shannon protocol integration.
//
// It is implemented by the structs:
//   - fullNode: default implementation of a full node for the Shannon protocol.
//   - fullNodeWithCache: the default full node with a SturdyC-based cache.
type ShannonFullNode interface {
	// GetApp returns the onchain application matching the application address
	GetApp(ctx context.Context, appAddr string) (*apptypes.Application, error)

	// GetSession returns the latest session matching the supplied service+app combination.
	// Sessions are solely used for sending relays, and therefore only the latest session for any service+app combination is needed.
	// Note: Shannon returns the latest session for a service+app combination if no blockHeight is provided.
	GetSession(ctx context.Context, serviceID sdk.ServiceID, appAddr string) (sessiontypes.Session, error)

	// GetAccountPubKey returns the account public key for the given address.
	// The cache has no TTL, so the public key is cached indefinitely.
	GetAccountPubKey(ctx context.Context, address string) (cryptotypes.PubKey, error)

	// IsHealthy returns true if the full node is healthy.
	IsHealthy() bool
}

// GetActiveSessions retrieves active sessions for a list of app addresses and a service ID.
// It verifies that each app delegates to the gateway and is staked for the requested service.
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
		// Retrieve the session for the app from the gateway client's full node.
		session, err := c.GetSession(ctx, serviceID, appAddr)
		if err != nil {
			return nil, fmt.Errorf("%w: app: %s, error: %w",
				ErrProtocolContextSetupAppFetchErr,
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
				ErrProtocolContextSetupAppDelegation,
				c.gatewayAddress,
				app.Address,
			)
		}

		activeSessions = append(activeSessions, session)
	}

	if len(activeSessions) == 0 {
		return nil, fmt.Errorf("%w: service %s",
			ErrProtocolContextSetupNoSessions,
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

// SignRelayRequest signs the given relay request using the signer's private key and the application's ring.
//
//   - Returns a pointer instead of directly setting the signature on the input relay request to avoid implicit output.
//   - Ideally, the function should accept a struct rather than a pointer, and also return an updated struct instead of a pointer.
func (c *GatewayClient) SignRelayRequest(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	app apptypes.Application,
) (*servicetypes.RelayRequest, error) {
	sessionEndBlockHeight := uint64(relayRequest.Meta.SessionHeader.SessionEndBlockHeight)

	logger := c.logger.With(
		"method", "SignRelayRequest",
		"app_address", app.Address,
		"session_end_block_height", sessionEndBlockHeight,
	)

	// Get the session ring for the application's session end block height
	sessionRing, err := c.getRing(ctx, app, sessionEndBlockHeight)
	if err != nil {
		err := fmt.Errorf("%w: %w: app: %s", ErrSignRelayRequestAppFetchErr, err, app.Address)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	// Get the signable bytes hash from the relay request
	signableBz, err := relayRequest.GetSignableBytesHash()
	if err != nil {
		logger.Error().Err(err).Msgf("error getting signable bytes hash from the relay request")
		return nil, fmt.Errorf("Sign: error getting signable bytes hash from the relay request: %w", err)
	}

	// TODO_IMPROVE:
	// - Store the private key as a scalar in Signer to reduce processing steps per Relay Request.
	signerPrivKeyBz, err := hex.DecodeString(c.gatewayPrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("Sign: error decoding private key to a string: %w", err)
	}

	signerPrivKey, err := ring.Secp256k1().DecodeToScalar(signerPrivKeyBz)
	if err != nil {
		return nil, fmt.Errorf("Sign: error decoding private key to a scalar: %w", err)
	}

	// Sign the request using the session ring and signer's private key
	ringSig, err := sessionRing.Sign(signableBz, signerPrivKey)
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error signing using the ring of application with address %s: %w",
			app.Address,
			err,
		)
	}

	// Serialize the signature
	signature, err := ringSig.Serialize()
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error serializing the signature of application with address %s: %w",
			app.Address,
			err,
		)
	}

	// Set the signature on the relay request
	relayRequest.Meta.Signature = signature
	return relayRequest, nil
}

// GetRing returns the ring for the application until the current session end height.
//
//   - Ring is created using the application's public key and the public keys of gateways currently delegated from the application
//   - Returns error if PublicKeyFetcher is not set or any pubkey fetch fails
func (c *GatewayClient) getRing(
	ctx context.Context,
	app apptypes.Application,
	sessionEndHeight uint64,
) (addressRing *ring.Ring, err error) {
	currentGatewayAddresses := rings.GetRingAddressesAtSessionEndHeight(&app, sessionEndHeight)

	ringAddresses := make([]string, 0)
	ringAddresses = append(ringAddresses, app.Address)

	if len(currentGatewayAddresses) == 0 {
		ringAddresses = append(ringAddresses, app.Address)
	} else {
		ringAddresses = append(ringAddresses, currentGatewayAddresses...)
	}

	ringPubKeys := make([]cryptotypes.PubKey, 0, len(ringAddresses))
	for _, address := range ringAddresses {
		pubKey, err := c.GetAccountPubKey(ctx, address)
		if err != nil {
			return nil, err
		}
		ringPubKeys = append(ringPubKeys, pubKey)
	}

	return rings.GetRingFromPubKeys(ringPubKeys)
}

// ValidateRelayResponse validates the RelayResponse and verifies the supplier's signature.
//
//   - Returns the RelayResponse, even if basic validation fails (may contain error reason).
//   - Verifies supplier's signature with the provided publicKeyFetcher.
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

	supplierPubKey, err := c.GetAccountPubKey(ctx, string(supplierAddress))
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
