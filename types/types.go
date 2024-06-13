package types

import (
	"net/http"

	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
)

// SessionSuppliers capture a single session and the list of supplier endpoints
// that can be used to send relay requests to the suppliers.
type SessionSuppliers struct {
	// Session is the fully hydrated session object returned by the query.
	Session *sessiontypes.Session

	// SuppliersEndpoints is a slice of the session's suppliers endpoints.
	// Any of these endpoints can be used to send a relay while the session
	// is active.
	SuppliersEndpoints []*SingleSupplierEndpoint
}

// SingleSupplierEndpoint represents a supplier's endpoint augmented with the
// session's header and the supplier's address for easy access to the needed
// information when sending a relay request.
type SingleSupplierEndpoint struct {
	Url             string
	RpcType         sharedtypes.RPCType
	SupplierAddress string
	SessionHeader   *sessiontypes.SessionHeader
}

// CopyToHTTPHeader copies the poktHeader map to the httpHeader map.
func CopyToHTTPHeader(poktHeader map[string]*Header, httpHeader http.Header) {
	for key, header := range poktHeader {
		for _, value := range header.Values {
			httpHeader.Add(key, value)
		}
	}
}
