package sdk

import (
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
)

type SessionSuppliers struct {
	// Session is the fully hydrated session object returned by the query.
	Session *sessiontypes.Session

	// SuppliersEndpoints is a slice of the session's suppliers endpoints each
	// item representing a single supplier endpoint augmented with the session
	// header and the supplier's address.
	// An item from this slice is what needs to be passed to the `SendRelay`
	// function so it has all the information needed to send the relay request.
	SuppliersEndpoints []*SingleSupplierEndpoint
}

// SingleSupplierEndpoint is the structure that represents a supplier's endpoint
// augmented with the session's header and the supplier's address for easy
// access to the needed information when sending a relay request.
type SingleSupplierEndpoint struct {
	Url             string
	RpcType         sharedtypes.RPCType
	SupplierAddress string
	Header          *sessiontypes.SessionHeader
}
