package client

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"

	"github.com/pokt-network/shannon-sdk/sdk"
)

var _ sdk.RelayClient = (*relayClient)(nil)

// relayClient is a RelayClient implementation that uses the HTTP client
// of the Go standard library.
type relayClient struct {
	httpClient *http.Client
}

// NewRelayClient creates a new relay client with the default HTTP client, which
// is used to send the relay requests.
func NewRelayClient() (sdk.RelayClient, error) {
	return &relayClient{
		httpClient: http.DefaultClient,
	}, nil
}

// SendRequest sends the relay request to the supplier at the given URL.
// It accepts the relay request bytes to avoid relying on protocol specific
// request objects such as http.Request.
func (r *relayClient) SendRequest(
	ctx context.Context,
	supplierUrlStr string,
	requestBz []byte,
) (relayResponseBz []byte, err error) {
	supplierUrl, err := url.Parse(supplierUrlStr)
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

	relayHTTPResponse, err := r.httpClient.Do(relayHTTPRequest)
	if err != nil {
		return nil, err
	}
	defer relayHTTPResponse.Body.Close()

	return io.ReadAll(relayHTTPResponse.Body)
}
