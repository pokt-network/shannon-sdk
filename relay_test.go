package sdk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
)

func ExampleRelay() {
	var filteredSession *SessionFilter
	// 1. Get the currnet session and set it on the filteredSession
	// 2. Select Endpoint using FilteredSession
	endpoints, err := filteredSession.FilteredEndpoints()
	if err != nil {
		fmt.Printf("error getting filtered endpoints: %v\n", err)
		return
	}

	if len(endpoints) == 0 {
		fmt.Println("no endpoints returned by the filter.")
		return
	}

	// 3. Build a Relay Request
	req, err := BuildRelayRequest(endpoints[0], []byte("relay request payload"))
	if err != nil {
		fmt.Printf("error building relay request: %v", err)
		return
	}

	// 4. Sign the Relay Request
	// 4.a. Create a mock FullNode
	mockFullNode := &mockFullNode{}

	// 4.b. Create a signer with the mock FullNode
	signer := Signer{
		PrivateKeyHex:    "private key hex",
		PublicKeyFetcher: mockFullNode,
	}

	// 4.c. Create an application
	var app apptypes.Application
	// Load/Set app to the target application

	ctx := context.Background()
	// 4.d. Sign the Relay Request
	req, err = signer.SignRelayRequest(ctx, req, app)
	if err != nil {
		fmt.Printf("error signing relay: %v", err)
		return
	}

	// 4.e. Send the Signed Relay Request to the selected endpoint
	responseBz, err := SendHttpRelay(ctx, endpoints[0].Endpoint().Url, *req)
	if err != nil {
		fmt.Printf("error sending relay: %v", err)
		return
	}

	// 4.f. Verify the returned response against supplier's public key
	validatedResponse, err := mockFullNode.ValidateRelayResponse(ctx, SupplierAddress(req.Meta.SupplierOperatorAddress), responseBz)
	if err != nil {
		fmt.Printf("response failed validation: %v", err)
		return
	}

	fmt.Printf("Validated response: %v\n", validatedResponse)
}

// SendHttpRelay sends the relay request to the supplier at the given URL using an HTTP Post request.
func SendHttpRelay(
	ctx context.Context,
	supplierUrlStr string,
	relayRequest servicetypes.RelayRequest,
) (relayResponseBz []byte, err error) {
	supplierUrl, err := url.Parse(supplierUrlStr)
	if err != nil {
		return nil, err
	}

	relayRequestBz, err := relayRequest.Marshal()
	if err != nil {
		return nil, err
	}

	relayRequestReadCloser := io.NopCloser(bytes.NewReader(relayRequestBz))
	defer relayRequestReadCloser.Close()

	relayHTTPRequest := &http.Request{
		Method: http.MethodPost,
		URL:    supplierUrl,
		Body:   relayRequestReadCloser,
	}

	relayHTTPResponse, err := http.DefaultClient.Do(relayHTTPRequest)
	if err != nil {
		return nil, err
	}
	defer relayHTTPResponse.Body.Close()

	return io.ReadAll(relayHTTPResponse.Body)
}

// mockFullNode implements the FullNode interface for testing purposes
type mockFullNode struct{}

func (m *mockFullNode) GetApp(ctx context.Context, appAddr string) (*apptypes.Application, error) {
	// Return a mock application for testing
	return &apptypes.Application{
		Address: appAddr,
	}, nil
}

func (m *mockFullNode) GetSession(ctx context.Context, serviceID ServiceID, appAddr string) (sessiontypes.Session, error) {
	// Return a mock session for testing
	return sessiontypes.Session{}, nil
}

func (m *mockFullNode) GetAccountPubKey(ctx context.Context, address string) (cryptotypes.PubKey, error) {
	// Return a mock public key for testing
	return nil, fmt.Errorf("mock implementation - not implemented")
}

func (m *mockFullNode) ValidateRelayResponse(ctx context.Context, supplierAddr SupplierAddress, responseBz []byte) (*servicetypes.RelayResponse, error) {
	// Return a mock validated response for testing
	return &servicetypes.RelayResponse{}, nil
}

func (m *mockFullNode) IsHealthy() bool {
	return true
}
