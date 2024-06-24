package sdk

// The returned RelayRequest struct can be marshalled and delivered to a service endpoint through an HTTP POST request.
func BuildRelayRequest(
	endpointProvider EndpointProvider,
	requestBz []byte,
) (*servicetypes.RelayRequest, error) {
	if endpointProvider == nil {
		return nil, errors.New("BuildRelayRequest: endpointProvider not specified")
	}

	sessionHeader, err := endpointProvider.SessionHeader()
	if err != nil {
		return nil, fmt.Errorf("BuildRelayRequest: could not get session header: %w", err)
	}

	if err := sessionHeader.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("BuildRelayRequest: error validating session header: %w", err)
	}

	supplierAddress, err := endpointProvider.SupplierAddress()
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
		supplierAddress,
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
// EndpointProvider is used by Relay utility functions to provide details on the target endpoint for a relay.
// A basic implementation of this interface is fulfilled by the `FilteredSession` struct.
type EndpointProvider interface {
	SessionHeader() (*sessiontypes.SessionHeader, error)
	Endpoint() (*sharedtypes.SupplierEndpoint, error)
	SupplierAddress() (string, error)
}
