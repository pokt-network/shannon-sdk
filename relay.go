package sdk

import (
	"context"
	"errors"

	servicetypes "github.com/pokt-network/poktroll/x/service/types"
)

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
			SessionHeader:   &header,
			SupplierAddress: string(endpoint.Supplier()),
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

	if signatureErr := relayResponse.VerifySupplierSignature(supplierPubKey); signatureErr != nil {
		return nil, signatureErr
	}

	return relayResponse, nil
}
