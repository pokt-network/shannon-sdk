package sdk

import (
	"context"
	"errors"
	"sync"

	cosmossdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pokt-network/poktroll/app"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
)

var once sync.Once

func init() {
	once.Do(func() {
		initCosmosSDKConfig()
	})
}

// initCosmosSDKConfig sets the prefix for application address to "pokt"
// This is necessary as otherwise the relay response validation would fail
// while validating the session header which should contain an application
// address in the expected format, i.e. Bech32 format with a "pokt" prefix.
func initCosmosSDKConfig() {
	// Set prefixes
	accountPubKeyPrefix := app.AccountAddressPrefix + "pub"
	validatorAddressPrefix := app.AccountAddressPrefix + "valoper"
	validatorPubKeyPrefix := app.AccountAddressPrefix + "valoperpub"
	consNodeAddressPrefix := app.AccountAddressPrefix + "valcons"
	consNodePubKeyPrefix := app.AccountAddressPrefix + "valconspub"

	// Set and seal config
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount(app.AccountAddressPrefix, accountPubKeyPrefix)
	config.SetBech32PrefixForValidator(validatorAddressPrefix, validatorPubKeyPrefix)
	config.SetBech32PrefixForConsensusNode(consNodeAddressPrefix, consNodePubKeyPrefix)
	config.Seal()
}

// The returned RelayRequest struct can be marshalled and delivered to a service
// endpoint through an HTTP POST request.
// BuildRelayRequest creates a RelayRequest struct from the given endpoint and request bytes,
// where requestBz is the serialized request (body and header) to be relayed.
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
		// Even if the relay response is invalid, we still return it to the caller
		// as it might contain the reason why it's failing basic validation.
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
