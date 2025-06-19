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
	ring "github.com/pokt-network/ring-go"
	"golang.org/x/exp/slices"

	sdk "github.com/pokt-network/shannon-sdk"
)

// OnchainDataFetcher defines the interface for fetching blockchain data.
// Implemented by both GRPCClient (direct access) and GatewayClientCache (cached access).
type OnchainDataFetcher interface {
	GetApp(ctx context.Context, appAddr string) (apptypes.Application, error)
	GetSession(ctx context.Context, serviceID sdk.ServiceID, appAddr string) (sessiontypes.Session, error)
	GetAccountPubKey(ctx context.Context, address string) (cryptotypes.PubKey, error)
	LatestBlockHeight(ctx context.Context) (height int64, err error)
	IsHealthy() bool
}

// GatewayClient provides high-level functionality for Shannon protocol relay operations.
//
// Core capabilities:
//   - Fetch onchain data (sessions, applications, account public keys)
//   - Sign relay requests using ring signatures
//   - Validate relay responses from suppliers
//
// The client embeds an OnchainDataFetcher (typically GatewayClientCache) for efficient
// data access and includes the gateway's credentials for signing operations.
type GatewayClient struct {
	logger polylog.Logger

	// Embedded data fetcher (typically GatewayClientCache for caching)
	OnchainDataFetcher

	gatewayAddress       string
	gatewayPrivateKeyHex string
}

// NewGatewayClient creates a new gateway client with the provided credentials and data fetcher.
func NewGatewayClient(
	logger polylog.Logger,
	dataFetcher OnchainDataFetcher,
	gatewayAddress string,
	gatewayPrivateKeyHex string,
) (*GatewayClient, error) {
	return &GatewayClient{
		logger:               logger,
		OnchainDataFetcher:   dataFetcher,
		gatewayAddress:       gatewayAddress,
		gatewayPrivateKeyHex: gatewayPrivateKeyHex,
	}, nil
}

// GetActiveSessions retrieves and validates sessions for the given service and applications.
//
// For each application, this method:
//   - Fetches the session from the blockchain
//   - Verifies the app is staked for the requested service
//   - Confirms the app delegates to this gateway
//
// Returns an error if any application fails validation or if no valid sessions are found.
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
		// Fetch session from cache or blockchain
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

		// Verify the app delegates to this gateway
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

// gatewayHasDelegationForApp checks if this gateway is delegated by the application
func (c *GatewayClient) gatewayHasDelegationForApp(app *apptypes.Application) bool {
	return slices.Contains(app.DelegateeGatewayAddresses, c.gatewayAddress)
}

// appIsStakedForService checks if the application is staked for the specified service
func appIsStakedForService(serviceID sdk.ServiceID, app *apptypes.Application) bool {
	for _, serviceConfig := range app.GetServiceConfigs() {
		if serviceConfig.GetServiceId() == string(serviceID) {
			return true
		}
	}
	return false
}

// SignRelayRequest signs a relay request using the gateway's private key and application's ring.
//
// The signing process:
//  1. Creates a ring from the application and its delegated gateways
//  2. Generates signable bytes from the relay request
//  3. Signs using the gateway's private key with ring signature
//  4. Attaches the signature to the relay request
//
// Returns a new relay request with the signature attached.
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

	// Create the session ring for signing
	sessionRing, err := c.getRing(ctx, app, sessionEndBlockHeight)
	if err != nil {
		err := fmt.Errorf("%w: %w: app: %s", ErrSignRelayRequestAppFetchErr, err, app.Address)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	// Get signable bytes hash from the relay request
	signableBz, err := relayRequest.GetSignableBytesHash()
	if err != nil {
		err := fmt.Errorf("%w: %w", ErrSignRelayRequestSignableBytesHash, err)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	// Decode the gateway's private key
	// TODO_IMPROVE: Store as scalar to avoid repeated decoding
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

	// Create ring signature
	ringSig, err := sessionRing.Sign(signableBz, signerPrivKey)
	if err != nil {
		err := fmt.Errorf("%w: %w: app: %s", ErrSignRelayRequestSignerPrivKeySign, err, app.Address)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	// Serialize and attach signature
	signature, err := ringSig.Serialize()
	if err != nil {
		err := fmt.Errorf("%w: %w: app: %s", ErrSignRelayRequestSignerPrivKeySerialize, err, app.Address)
		logger.Error().Err(err).Msg(err.Error())
		return nil, err
	}

	relayRequest.Meta.Signature = signature
	return relayRequest, nil
}

// getRing creates a ring signature ring for the application at the given session end height.
//
// The ring includes:
//   - The application's public key
//   - Public keys of all gateways delegated by the app at the session end height
//   - Falls back to just the application if no gateways are delegated
func (c *GatewayClient) getRing(
	ctx context.Context,
	app apptypes.Application,
	sessionEndHeight uint64,
) (*ring.Ring, error) {
	currentGatewayAddresses := rings.GetRingAddressesAtSessionEndHeight(&app, sessionEndHeight)

	// Build ring addresses starting with the application
	ringAddresses := make([]string, 0)
	ringAddresses = append(ringAddresses, app.Address)

	// Add delegated gateways or duplicate app address if none
	if len(currentGatewayAddresses) == 0 {
		ringAddresses = append(ringAddresses, app.Address)
	} else {
		ringAddresses = append(ringAddresses, currentGatewayAddresses...)
	}

	// Fetch public keys for all ring members
	ringPubKeys := make([]cryptotypes.PubKey, 0, len(ringAddresses))
	for _, address := range ringAddresses {
		// TODO_TECHDEBT: Cache app public key to avoid repeated fetches
		pubKey, err := c.GetAccountPubKey(ctx, address)
		if err != nil {
			return nil, err
		}
		ringPubKeys = append(ringPubKeys, pubKey)
	}

	return rings.GetRingFromPubKeys(ringPubKeys)
}

// ValidateRelayResponse validates and verifies a relay response from a supplier.
//
// Validation steps:
//  1. Unmarshal the response bytes
//  2. Perform basic validation (structure, required fields)
//  3. Fetch the supplier's public key
//  4. Verify the supplier's signature
//
// Returns the relay response even if basic validation fails (may contain error details).
// Returns nil only if unmarshaling fails or signature verification fails.
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

	// Basic validation (return response even if invalid - may contain error reason)
	if err := relayResponse.ValidateBasic(); err != nil {
		err := fmt.Errorf("%w: %w", ErrValidateRelayResponseValidateBasic, err)
		logger.Warn().Err(err).Msg(err.Error())
		return relayResponse, err
	}

	// Fetch supplier's public key for signature verification
	supplierPubKey, err := c.GetAccountPubKey(ctx, string(supplierAddress))
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
