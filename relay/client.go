package relay

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
)

var _ RelayClient = (*relayClient)(nil)

type relayClient struct {
	httpClient *http.Client
}

func NewRelayClient() (RelayClient, error) {
	return &relayClient{
		httpClient: http.DefaultClient,
	}, nil
}

func (r *relayClient) Do(
	ctx context.Context,
	supplierUrlStr string,
	relayRequestBz []byte,
	method string,
	requestHeaders map[string][]string,
) (relayResponseBz []byte, err error) {
	supplierUrl, err := url.Parse(supplierUrlStr)
	if err != nil {
		return nil, err
	}

	relayRequestReadCloser := io.NopCloser(bytes.NewReader(relayRequestBz))
	defer relayRequestReadCloser.Close()

	relayHTTPRequest := &http.Request{
		Method: method,
		Header: requestHeaders,
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
