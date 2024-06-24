package sdk

import (
	"context"
	"errors"
	"fmt"

	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
)

// The returned RelayRequest struct can be marshalled and delivered to a service endpoint through an HTTP POST request.
func BuildRelayRequest(
	endpointSelector EndpointSelector,
	requestBz []byte,
) (*servicetypes.RelayRequest, error) {
	if endpointSelector == nil {
		return nil, errors.New("BuildRelayRequest: endpointSelector not specified")
	}

	sessionHeader, err := endpointSelector.SessionHeader()
	if err != nil {
		return nil, fmt.Errorf("BuildRelayRequest: could not get session header: %w", err)
	}

	if err := sessionHeader.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("BuildRelayRequest: error validating session header: %w", err)
	}

	supplierAddress, err := endpointSelector.SelectedSupplierAddress()
	if err != nil {
		return nil, fmt.Errorf("BuildRelayRequest: error getting a supplier: %w", err)
	}

	return &servicetypes.RelayRequest{
		Meta: servicetypes.RelayRequestMetadata{
			SessionHeader:   sessionHeader,
			SupplierAddress: supplierAddress,
		},
		Payload: requestBz,
	}, nil
}

func ValidateRelayResponse(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	relayResponseBz []byte,
	publicKeyFetcher PublicKeyFetcher,
) (relayResponse *servicetypes.RelayResponse, err error) {
	// ---> Verify Response
	relayResponse = &servicetypes.RelayResponse{}
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
		relayRequest.Meta.SupplierAddress,
	)
	if err != nil {
		return nil, err
	}

	if err := relayResponse.VerifySupplierSignature(supplierPubKey); err != nil {
		return nil, err
	}

	return relayResponse, nil
}

// TODO_IMPROVE: add a detailed example on how to use the FilteredSession struct to provide endpoints to relay builder.
//
// EndpointSelector is used by Relay utility functions to provide details on the target endpoint for a relay.
// A basic implementation of this interface is fulfilled by the `FilteredSession` struct.
type EndpointSelector interface {
	SessionHeader() (*sessiontypes.SessionHeader, error)
	SelectedEndpoint() (*sharedtypes.SupplierEndpoint, error)
	SelectedSupplierAddress() (string, error)
}
