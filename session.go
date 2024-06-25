package sdk

import (
	"context"
	"errors"
	"fmt"

	"github.com/cosmos/gogoproto/grpc"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
	grpcoptions "google.golang.org/grpc"
)

// SessionClient is the interface to interact with the on-chain session module.
//
// For example, it can be used to get the current session for a given application
// and service id at a given height.
type SessionClient struct {
	PoktNodeSessionFetcher
}

// GetSession returns the session with the given application address, service id and height.
func (s *SessionClient) GetSession(
	ctx context.Context,
	appAddress string,
	serviceId string,
	height int64,
) (session *sessiontypes.Session, err error) {
	if s.PoktNodeSessionFetcher == nil {
		return nil, errors.New("PoktNodeSessionFetcher not set")
	}

	req := &sessiontypes.QueryGetSessionRequest{
		ApplicationAddress: appAddress,
		Service:            &sharedtypes.Service{Id: serviceId},
		BlockHeight:        height,
	}

	// TODO_DISCUSS: Would it be feasible to add a GetCurrentSession, supported by the underlying protocol?
	// It seems likely that GetSession will almost always be used to get the session matching the latest height.
	// If this is a correct assumption, the aforementioned support from the protocol could simplify fetching the current session.
	// In addition, the current session that is being returned could include the latest block height, reducing the number of
	// SDK calls needed for sending relays.
	res, err := s.PoktNodeSessionFetcher.GetSession(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.Session, nil
}

// NewPoktNodeSessionFetcher returns the default implementation of the PoktNodeSessionFetcher interface.
// It connects to a POKT full node through the session module's query client to get session data.
func NewPoktNodeSessionFetcher(grpcConn grpc.ClientConn) PoktNodeSessionFetcher {
	return sessiontypes.NewQueryClient(grpcConn)
}

// PoktNodeSessionFetcher is used by the SessionClient to fetch sessions using poktroll request/response types.
//
// Most users can rely on the default implementation provided by NewPoktNodeSessionFetcher function.
// A custom implementation of this interface can be used to gain more granular control over the interactions
// of the SessionClient with the POKT full node.
type PoktNodeSessionFetcher interface {
	GetSession(
		context.Context,
		*sessiontypes.QueryGetSessionRequest,
		...grpcoptions.CallOption,
	) (*sessiontypes.QueryGetSessionResponse, error)
}

// SupplierAddress captures the address for a supplier.
// This is defined to help enforce type safety by requiring explict type casting of a string before it can be used as a Supplier's address.
type SupplierAddress string

// An EndpointFilter returns a boolean indicating whether the input endpoint should be filtered out.
type EndpointFilter func(Endpoint) bool

// TODO_IMPROVE: add a `FilterSupplier` method to drop a supplier from consideration when sending a relay.
// TODO_IMPROVE: add a `FilterEndpoint` method to drop an endpoint from consideration when sending a relay.
// TODO_IMPROVE: add an OrderEndpoints to allow ordering the list of available, i.e. not filtered out, endpoints.
// The `SelectedEndpoint` method (and potentially a `SelectedEndpoints` method) should then return the ordered list of endpoints for the target service.
//
// FilteredSession wraps a Session, allowing node selection, currently through directly adding endpoints, and potentially through filtering/ordering in future versions.
// This is needed so functions that enable sending relays can be provided with a struct that contains both session data and the endpoint(s) selected for receiving relays.
type FilteredSession struct {
	*sessiontypes.Session

	EndpointFilters []EndpointFilter
	// TODO_IMPROVE: add a slice of endpoint ordering functions
}

// AllEndpoints returns all the endpoints corresponding to a session for the service id specified by the session header.
// The endpoints are not filtered.
func (f *FilteredSession) AllEndpoints() (map[SupplierAddress][]Endpoint, error) {
	if f.Session == nil {
		return nil, fmt.Errorf("AllEndpoints: Session not set on FilteredSession struct")
	}

	header := f.Session.Header
	supplierEndpoints := make(map[SupplierAddress][]Endpoint)
	for _, supplier := range f.Session.Suppliers {
		// TODO_DISCUSS: It may be a good idea to use a map[serviceID][]Endpoint under each supplier,
		//   especially if service IDs under each supplier should be unique.
		//   This would enable easy lookups based on service ID, without impacting any use cases that
		//   need to iterate through all the services.
		//
		// The endpoins slice is intentionally defined here to prevent any overwrites in the unlikely case
		// that there are duplicate service IDs under a supplier.
		var endpoints []Endpoint
		for _, service := range supplier.Services {
			// TODO_DISCUSS: considering a service Id is required to get a session, does it make sense for the suppliers under the session
			// to only contain endpoints for the (single) service covered by the session?
			if service.Service.Id != f.Session.Header.Service.Id {
				continue
			}

			var newEndpoints []Endpoint
			for _, e := range service.Endpoints {
				newEndpoints = append(newEndpoints, endpoint{
					// TODO_TECHDEBT: need deep copying here.
					header:           *header,
					supplierEndpoint: *e,
					supplier:         SupplierAddress(supplier.Address),
				})
			}
			endpoints = append(endpoints, newEndpoints...)
		}
		supplierEndpoints[SupplierAddress(supplier.Address)] = endpoints
	}

	return supplierEndpoints, nil
}

// TODO_DISCUSS: do we need a supplier-level filter to filter out all endpoints corresponding to a specific supplier?
// TODO_TECHDEBT: add a unit test to cover this method.
func (f *FilteredSession) FilteredEndpoints() ([]Endpoint, error) {
	allEndpoints, err := f.AllEndpoints()
	if err != nil {
		return nil, fmt.Errorf("FilteredEndpoints: error getting all endpoints: %w", err)
	}

	var filteredEndpoints []Endpoint
	for _, endpoints := range allEndpoints {
		for _, endpoint := range endpoints {
			includePoint := true
			for _, filter := range f.EndpointFilters {
				if filter(endpoint) {
					includePoint = false
					break
				}
			}
			if includePoint {
				filteredEndpoints = append(filteredEndpoints, endpoint)
			}
		}
	}

	return filteredEndpoints, nil
}

type endpoint struct {
	header           sessiontypes.SessionHeader
	supplierEndpoint sharedtypes.SupplierEndpoint
	supplier         SupplierAddress
}

func (e endpoint) Endpoint() sharedtypes.SupplierEndpoint {
	return e.supplierEndpoint
}

func (e endpoint) Supplier() SupplierAddress {
	return e.supplier
}

func (e endpoint) Header() sessiontypes.SessionHeader {
	return e.header
}

type Endpoint interface {
	Header() sessiontypes.SessionHeader
	Supplier() SupplierAddress
	Endpoint() sharedtypes.SupplierEndpoint
}
