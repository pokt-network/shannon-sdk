package sdk

import (
	"context"
	"errors"
	"sync"

	cosmossdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pokt-network/poktroll/app"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
)

// =======================
// Interfaces & Structs
// =======================
// (No interfaces or structs defined in this file. If you add any, group them here.)

var once sync.Once

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
		return nil, err
	}

	if err := relayResponse.ValidateBasic(); err != nil {
		// Even if the relay response is invalid, return it (may contain failure reason)
		return relayResponse, err
	}

	supplierPubKey, err := publicKeyFetcher.GetPubKeyFromAddress(
		ctx,
		string(supplierAddress),
	)
	if err != nil {
		return nil, err
	}

	if signatureErr := relayResponse.VerifySupplierOperatorSignature(supplierPubKey); signatureErr != nil {
		return nil, signatureErr
	}

	return relayResponse, nil
}
