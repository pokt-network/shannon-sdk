package sdk

import (
	"context"
	"errors"

	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
)

// The returned RelayRequest struct can be marshalled and delivered to a service endpoint through an HTTP POST request.
func BuildRelayRequest(
	endpoint Endpoint,
	requestBz []byte,
) (*servicetypes.RelayRequest, error) {
	if endpoint == nil {
		return nil, errors.New("BuildRelayRequest: endpointSelector not specified")
	}

	header := endpoint.Header()
	// TODO_DISCUSS: the Header provided by the Endpoint is assumed to be valid, is this a reasonable assumption?
	return &servicetypes.RelayRequest{
		Meta: servicetypes.RelayRequestMetadata{
			SessionHeader:   &header,
			SupplierAddress: string(endpoint.Supplier()),
		},
		Payload: requestBz,
	}, nil
}

func ValidateRelayResponse(
	ctx context.Context,
	supplierAddress SupplierAddress,
	relayResponseBz []byte,
	publicKeyFetcher PublicKeyFetcher,
) (*servicetypes.RelayResponse, error) {
	// ---> Verify Response
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

	if signatureErr := relayResponse.VerifySupplierSignature(supplierPubKey); signatureErr != nil {
		return nil, signatureErr
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
