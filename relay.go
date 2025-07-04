package sdk

import (
	"context"
	"errors"
	"fmt"
	"sync"

	cosmossdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pokt-network/poktroll/app"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
)

// =======================
// Interfaces & Structs
// =======================
// (No interfaces or structs defined in this file. If you add any, group them here.)

var (
	// Ensures Cosmos SDK init is run only once
	once sync.Once

	// Endpoint payload failed to unmarshal as RelayResponse
	ErrRelayResponseValidationUnmarshal = errors.New("Endpoint payload failed to unmarshal as RelayResponse")
	// RelayResponse failed basic validation: e.g. empty session header in the RelayResponse struct.
	ErrRelayResponseValidationBasicValidation = errors.New("RelayResponse failed basic validation")
	// Could not fetch the public key for supplier address used for the relay.
	ErrRelayResponseValidationGetPubKey = errors.New("error getting public key for supplier address")
	// Received nil public key on supplier lookup using its address
	ErrRelayResponseValidationNilSupplierPubKey = errors.New("received nil public key for supplier address")
	// RelayResponse's signature failed validation.
	ErrRelayResponseValidationSignatureError = errors.New("RelayResponse signature failed validation")
)

func init() {
	once.Do(func() {
		initCosmosSDKConfig()
	})
}

// initCosmosSDKConfig sets the prefix for application address to "pokt"
//
// - Required for relay response validation, specifically for validating the session header.
// - Ensures application addresses are in the expected Bech32 format with a "pokt" prefix.
func initCosmosSDKConfig() {
	// Set Bech32 prefixes
	accountPubKeyPrefix := app.AccountAddressPrefix + "pub"
	validatorAddressPrefix := app.AccountAddressPrefix + "valoper"
	validatorPubKeyPrefix := app.AccountAddressPrefix + "valoperpub"
	consNodeAddressPrefix := app.AccountAddressPrefix + "valcons"
	consNodePubKeyPrefix := app.AccountAddressPrefix + "valconspub"

	config := cosmossdk.GetConfig()
	config.SetBech32PrefixForAccount(app.AccountAddressPrefix, accountPubKeyPrefix)
	config.SetBech32PrefixForValidator(validatorAddressPrefix, validatorPubKeyPrefix)
	config.SetBech32PrefixForConsensusNode(consNodeAddressPrefix, consNodePubKeyPrefix)
	config.Seal()
}

// BuildRelayRequest creates a RelayRequest struct from the given endpoint and request bytes.
//
// - The returned RelayRequest can be marshalled and delivered to a service endpoint via HTTP POST.
// - requestBz: serialized request (body and header) to be relayed.
func BuildRelayRequest(
	endpoint Endpoint,
	requestBz []byte,
) (*servicetypes.RelayRequest, error) {
	if endpoint == nil {
		return nil, errors.New("BuildRelayRequest: endpointSelector not specified")
	}

	header := endpoint.Header()
	return &servicetypes.RelayRequest{
		Meta: servicetypes.RelayRequestMetadata{
			SessionHeader:           &header,
			SupplierOperatorAddress: string(endpoint.Supplier()),
		},
		Payload: requestBz,
	}, nil
}

// ValidateRelayResponse validates the RelayResponse and verifies the supplier's signature.
//
// - Returns the RelayResponse, even if basic validation fails (may contain error reason).
// - Verifies supplier's signature with the provided publicKeyFetcher.
func ValidateRelayResponse(
	ctx context.Context,
	supplierAddress SupplierAddress,
	relayResponseBz []byte,
	publicKeyFetcher PublicKeyFetcher,
) (*servicetypes.RelayResponse, error) {
	relayResponse := &servicetypes.RelayResponse{}
	if err := relayResponse.Unmarshal(relayResponseBz); err != nil {
		return nil, fmt.Errorf("%w: error unmarshaling the raw payload into a RelayResponse struct: %w", ErrRelayResponseValidationUnmarshal, err)
	}

	if err := relayResponse.ValidateBasic(); err != nil {
		// Even if the relay response is invalid, return it (may contain failure reason)
		return relayResponse, fmt.Errorf("%w: Relay response failed basic validation: %w", ErrRelayResponseValidationBasicValidation, err)
	}

	supplierPubKey, err := publicKeyFetcher.GetPubKeyFromAddress(
		ctx,
		string(supplierAddress),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to get public key for supplier address %s: %w", ErrRelayResponseValidationGetPubKey, string(supplierAddress), err)
	}

	// This can happen if a supplier has never been used (e.g. funded) onchain
	if supplierPubKey == nil {
		return nil, fmt.Errorf("%w: received nil supplier public key for address %s", ErrRelayResponseValidationNilSupplierPubKey, string(supplierAddress))
	}

	if signatureErr := relayResponse.VerifySupplierOperatorSignature(supplierPubKey); signatureErr != nil {
		return nil, fmt.Errorf("%s: relay response failed signature verification: %w", ErrRelayResponseValidationSignatureError, signatureErr)
	}

	return relayResponse, nil
}
