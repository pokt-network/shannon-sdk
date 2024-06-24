package sdk

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"

	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"

	grpc "github.com/cosmos/gogoproto/grpc"
)

func ExampleSendRelay() {
	var filteredSession *FilteredSession
	// 1. Get the currnet session and set it on the filteredSession
	// 2. Select Endpoint using FilteredSession
	// ...

	// 3. Build a Relay Request
	req, err := BuildRelayRequest(filteredSession, []byte("relay request payload"))
	if err != nil {
		fmt.Printf("error building relay request: %v", err)
		return
	}

	// 4. Sign the Relay Request
	// 4.a. Create a signer
	signer := Signer{PrivateKeyHex: "private key hex"}

	// 4.b. setup the grpc connection
	var grpcConn grpc.ClientConn
	// ...

	// 4.c. Create an AccountClient
	accountClient := AccountClient{
		PoktNodeAccountFetcher: NewPoktNodeAccountFetcher(grpcConn),
	}

	// 4.d. Create an application ring
	var app apptypes.Application
	// Load/Set app to the target application
	ring := ApplicationRing{
		Application:      app,
		PublicKeyFetcher: &accountClient,
	}

	ctx := context.Background()
	// 4.e. Sign the Relay Request
	var queryHeight uint64
	// Set queryHeight to the desired block height
	req, err = signer.Sign(ctx, req, ring, queryHeight)
	if err != nil {
		fmt.Printf("error signing relay: %v", err)
		return
	}

	// 4.f. Send the Signed Relay Request to the selected endpoint
	endpoint, err := filteredSession.SelectedEndpoint()
	if err != nil {
		fmt.Errorf("error getting the selected endpoint for sending a relay: %v", err)
		return
	}

	responseBz, err := SendHttpRelay(ctx, endpoint.Url, *req)
	if err != nil {
		fmt.Printf("error sending relay: %v", err)
		return
	}

	// 4.g. Verify the returned response against supplier's public key
	validatedResponse, err := ValidateRelayResponse(ctx, req, responseBz, &accountClient)
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
