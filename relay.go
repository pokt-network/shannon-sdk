package sdk

import (
	"errors"
	"sync"

	cosmossdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/pokt-network/poktroll/app"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
)

// TODO_IN_THIS_PR(@commoddity): create a new `relay_test.go` file the tests the process of signing and building a relay request.

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
	ErrRelayResponseValidationGetPubKey = errors.New("Error getting public key for supplier address")
	// Received nil public key on supplier lookup using its address
	ErrRelayResponseValidationNilSupplierPubKey = errors.New("Received nil public key for supplier address")
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
