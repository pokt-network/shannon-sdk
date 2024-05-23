package relay

import "context"

type RelayClient interface {
	Do(
		ctx context.Context,
		supplierUrl string,
		relayRequestBz []byte,
		method string,
		requestHeaders map[string][]string,
	) (relayResponseBz []byte, err error)
}
