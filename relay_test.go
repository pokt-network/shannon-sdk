package sdk

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
)

func ExampleSigner() {
	signedRelay, err := r.Signer.Sign(ctx, relayRequest, app, queryHeight)
	if err != nil {
		return nil, err
	}
}

func ExampleSendRelay() {
	var filteredSession FilteredSession
	// 1. Get the currnet session and set it on the filteredSession
	// 2. Select Endpoint using FilteredSession
	// ...

	// 3. Build a Relay Request
	req, err := BuildRelayRequest(filteredSession, []byte("relay request payload"))
	if err != nil {
		fmt.Printf("error building relay request: %v", err)
		return
	}

	// 4. Sign the Relya Request
	// 4.a. Create a signer
	signer := Signer{PrivateKeyHex: []byte("private key hex")}

	// 4.b. setup the grpc connection
	var grpcConn grpc.ClientConn
	// ...

	// 4.c. Create an AccountClient
	ac := AccountClinet{
		PoktNodeAccountFetcher: NewPoktNodeAccountFetcher(grpcConn),
	}

	// 4.d. Create an application ring
	ring := ApplicationRing{
		Application:      app,
		PublicKeyFetcher: &accountClinet,
	}

	// 4.e. Sign the Relay Request
	req, err = signer.Sign(ctx, ring, queryHeight)
	if err != nil {
		fmt.Printf("error signing relay: %v", err)
		return
	}

	// 4.f. Send the Signed Relay Request to the selected endpoint
	responseBz, err := SendHttpRelay(ctx, req, filteredSession)
	if err != nil {
		fmt.Printf("error sending relay: %v", err)
		return
	}

	// 4.g. Verify the returned response against supplier's public key

}

// SendRequest sends the relay request to the supplier at the given URL using an HTTP Post request.
func SendHttpRequest(
	ctx context.Context,
	supplierUrlStr string,
	relayRequest RelayRequest,
) (relayResponseBz []byte, err error) {
	supplierUrl, err := url.Parse(supplierUrlStr)
	if err != nil {
		return nil, err
	}

	relayRequestBz, err := relayRequest.Marshal()
	if err != nil {
		return nil, err
	}

	relayRequestReadCloser := io.NopCloser(bytes.NewReader(requestBz))
	defer relayRequestReadCloser.Close()

	relayHTTPRequest := &http.Request{
		Method: http.MethodPost,
		URL:    supplierUrl,
		Body:   relayRequestReadCloser,
	}

	relayHTTPResponse, err := httpClient.Do(relayHTTPRequest)
	if err != nil {
		return nil, err
	}
	defer relayHTTPResponse.Body.Close()

	return io.ReadAll(relayHTTPResponse.Body)
}
