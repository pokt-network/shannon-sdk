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
	req, err = signer.Sign(ctx, req, ring)
	if err != nil {
		fmt.Printf("error signing relay: %v", err)
		return
	}

	// 4.f. Send the Signed Relay Request to the selected endpoint
	responseBz, err := SendHttpRelay(ctx, endpoints[0].Endpoint().Url, *req)
	if err != nil {
		fmt.Printf("error sending relay: %v", err)
		return
	}

	// 4.g. Verify the returned response against supplier's public key
	validatedResponse, err := ValidateRelayResponse(ctx, SupplierAddress(req.Meta.SupplierAddress), responseBz, &accountClient)
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
