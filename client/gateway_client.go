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
//   - Fetch onchain data for the Shannon protocol integration,
//     such as: sessions, applications, and account public keys.
//   - Sign relays using ring signatures (with the gateway's private key)
//   - Validate relay responses
//
// It contains:
//   - A GatewayClientCache for fetching and caching onchain data
//   - The gateway address & private key
type GatewayClient struct {
	logger polylog.Logger

	// Embeds the GatewayClientCache to fetch and cache onchain data.
	*GatewayClientCache

	gatewayAddress       string
	gatewayPrivateKeyHex string
}

// NewGatewayClient builds and returns a GatewayClient using the supplied configuration.
func NewGatewayClient(
	logger polylog.Logger,
	cache *GatewayClientCache,
	gatewayAddress string,
	gatewayPrivateKeyHex string,
) (*GatewayClient, error) {
	return &GatewayClient{
		logger:               logger,
		GatewayClientCache:   cache,
		gatewayAddress:       gatewayAddress,
		gatewayPrivateKeyHex: gatewayPrivateKeyHex,
	}, nil
}

// -- Onchain Session Data Fetching --

// GetActiveSessions retrieves active sessions for a list of app addresses and a service ID.
// It verifies that each app delegates to the gateway and is staked for the requested service.
func (c *GatewayClient) GetActiveSessions(
	ctx context.Context,
	serviceID sdk.ServiceID,
	appAddresses []string,
) ([]sessiontypes.Session, error) {
	logger := c.logger.With(
		"method", "GetActiveSessions",
		"service_id", serviceID,
		"app_addresses_len", len(appAddresses),
	)

	if len(appAddresses) == 0 {
		err := fmt.Errorf("%w: service: %s", ErrProtocolContextSetupNoAppAddresses, serviceID)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	var activeSessions []sessiontypes.Session

	for _, appAddr := range appAddresses {
		// Retrieve the session for the app from the gateway client cache.
		session, err := c.GetSession(ctx, serviceID, appAddr)
		if err != nil {
			err := fmt.Errorf("%w: app: %s, error: %w", ErrProtocolContextSetupAppFetchErr, appAddr, err)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}

		app := session.Application

		// Verify the app is staked for the requested service
		if !appIsStakedForService(serviceID, app) {
			err := fmt.Errorf("%w: app: %s", ErrProtocolContextSetupAppNotStaked, app.Address)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}

		// Verify the app delegates to the gateway
		if !c.gatewayHasDelegationForApp(app) {
			err := fmt.Errorf("%w: gateway: %s, app: %s", ErrProtocolContextSetupAppDelegation, c.gatewayAddress, app.Address)
			logger.Error().Err(err).Msg(err.Error())
			return nil, err
		}

		activeSessions = append(activeSessions, session)
	}

	if len(activeSessions) == 0 {
		err := fmt.Errorf("%w: service: %s", ErrProtocolContextSetupNoSessions, serviceID)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
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

// -- Relay Request Signing --

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
		err := fmt.Errorf("%w: %w", ErrSignRelayRequestSignableBytesHash, err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	// TODO_IMPROVE:
	// - Store the private key as a scalar in Signer to reduce processing steps per Relay Request.
	signerPrivKeyBz, err := hex.DecodeString(c.gatewayPrivateKeyHex)
	if err != nil {
		err := fmt.Errorf("%w: %w", ErrSignRelayRequestSignerPrivKey, err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	signerPrivKey, err := ring.Secp256k1().DecodeToScalar(signerPrivKeyBz)
	if err != nil {
		err := fmt.Errorf("%w: %w", ErrSignRelayRequestSignerPrivKeyDecode, err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	// Sign the request using the session ring and signer's private key
	ringSig, err := sessionRing.Sign(signableBz, signerPrivKey)
	if err != nil {
		err := fmt.Errorf("%w: %w: app: %s", ErrSignRelayRequestSignerPrivKeySign, err, app.Address)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	// Serialize the signature
	signature, err := ringSig.Serialize()
	if err != nil {
		err := fmt.Errorf("%w: %w: app: %s", ErrSignRelayRequestSignerPrivKeySerialize, err, app.Address)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	// Set the signature on the relay request
	relayRequest.Meta.Signature = signature
	return relayRequest, nil
}

// getRing returns the ring for the application until the current session end height.
//
//   - Ring is created using the application's public key and the public keys of gateways currently delegated from the application
//   - Returns error if PublicKeyFetcher is not set or any pubkey fetch fails
func (c *GatewayClient) getRing(
	ctx context.Context,
	app apptypes.Application,
	sessionEndHeight uint64,
) (addressRing *ring.Ring, err error) {
	currentGatewayAddresses := rings.GetRingAddressesAtSessionEndHeight(&app, sessionEndHeight)

	// Add the application address to the ring addresses
	ringAddresses := make([]string, 0)
	ringAddresses = append(ringAddresses, app.Address)

	// If there are no current gateway addresses, use the application address
	if len(currentGatewayAddresses) == 0 {
		ringAddresses = append(ringAddresses, app.Address)
	} else {
		ringAddresses = append(ringAddresses, currentGatewayAddresses...)
	}

	// Get the public keys for the ring addresses
	ringPubKeys := make([]cryptotypes.PubKey, 0, len(ringAddresses))
	for _, address := range ringAddresses {
		// TODO_TECHDEBT(@commoddity): investigate if we can avoid needing
		// to fetch the public key for the application address for every relay request.
		pubKey, err := c.getAccountPubKey(ctx, address)
		if err != nil {
			return nil, err
		}
		ringPubKeys = append(ringPubKeys, pubKey)
	}

	return rings.GetRingFromPubKeys(ringPubKeys)
}

// -- Relay Response Validation --

// ValidateRelayResponse validates the RelayResponse and verifies the supplier's signature.
//
//   - Returns the RelayResponse, even if basic validation fails (may contain error reason).
//   - Verifies supplier's signature with the provided publicKeyFetcher.
func (c *GatewayClient) ValidateRelayResponse(
	ctx context.Context,
	supplierAddress sdk.SupplierAddress,
	relayResponseBz []byte,
) (*servicetypes.RelayResponse, error) {
	logger := c.logger.With(
		"method", "ValidateRelayResponse",
		"supplier_address", supplierAddress,
		"relay_response_bz_len", len(relayResponseBz),
	)

	// Unmarshal the relay response
	relayResponse := &servicetypes.RelayResponse{}
	if err := relayResponse.Unmarshal(relayResponseBz); err != nil {
		err := fmt.Errorf("%w: %w", ErrValidateRelayResponseUnmarshal, err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	// Perform basic validation of the relay response
	if err := relayResponse.ValidateBasic(); err != nil {
		err := fmt.Errorf("%w: %w", ErrValidateRelayResponseValidateBasic, err)
		logger.Warn().Err(err).Msg(err.Error())

		// Even if the relay response is invalid, return it (may contain failure reason)
		return relayResponse, err
	}

	// Get the supplier's public key
	supplierPubKey, err := c.getAccountPubKey(ctx, string(supplierAddress))
	if err != nil {
		err := fmt.Errorf("%w: %w", ErrValidateRelayResponseAccountPubKey, err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}
	if supplierPubKey == nil {
		err := fmt.Errorf("%w: %s", ErrValidateRelayResponsePubKeyNil, supplierAddress)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	// Verify the supplier's signature
	if signatureErr := relayResponse.VerifySupplierOperatorSignature(supplierPubKey); signatureErr != nil {
		err := fmt.Errorf("%w: %w", ErrValidateRelayResponseSignature, signatureErr)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	return relayResponse, nil
}
