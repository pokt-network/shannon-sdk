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

// --- Interfaces ---

// PoktNodeSessionFetcher is used by SessionClient to fetch sessions using poktroll session type.
//
// - Most users can rely on the default implementation (see NewPoktNodeSessionFetcher).
// - Custom implementations allow granular control over SessionClient interactions with the POKT full node.
type PoktNodeSessionFetcher interface {
	GetSession(
		context.Context,
		*sessiontypes.QueryGetSessionRequest,
		...grpcoptions.CallOption,
	) (*sessiontypes.QueryGetSessionResponse, error)
}

// Endpoint is an interface representing an endpoint with its supplier and session.
//
// TODO_REFACTOR: Prefix Endpoint methods with `Get` to clarify they are getters.
type Endpoint interface {
	Header() sessiontypes.SessionHeader
	Supplier() SupplierAddress
	Endpoint() sharedtypes.SupplierEndpoint
}

// --- Structs ---

// SessionClient interacts with the on-chain session module.
//
// - Used to get the current session for a given application and service id at a given height.
type SessionClient struct {
	PoktNodeSessionFetcher
}

// SupplierAddress captures the address for a supplier.
//
// - Enforces type safety by requiring explicit type casting of a string before use as a Supplier's address.
type SupplierAddress string

// EndpointFilter is a function type used by SessionFilter to return a boolean indicating whether the input endpoint should be filtered out.
type EndpointFilter func(Endpoint) bool

// SessionFilter wraps a Session, allowing node selection by filtering out endpoints based on the filters set on the struct.
//
// - Needed so relay-sending functions can be provided with a struct containing both session data and the selected endpoint(s).
type SessionFilter struct {
	*sessiontypes.Session
	EndpointFilters []EndpointFilter
}

// endpoint represents an endpoint with its corresponding supplier and session.
// Implements the Endpoint interface.
type endpoint struct {
	header           sessiontypes.SessionHeader
	supplierEndpoint sharedtypes.SupplierEndpoint
	supplier         SupplierAddress
}

// --- Functions ---

// GetSession returns the session with the given application address, service id, and height.
//
// - Returns an error if the context deadline is exceeded while fetching the session.
func (s *SessionClient) GetSession(
	ctx context.Context,
	appAddress string,
	serviceId string,
	height int64,
) (session *sessiontypes.Session, err error) {
	if s.PoktNodeSessionFetcher == nil {
		return nil, errors.New("PoktNodeSessionFetcher not set")
	}

	var (
		fetchedSession *sessiontypes.Session
		fetchErr       error
		// Will be closed to signal that fetch is completed.
		doneCh = make(chan struct{})
	)

	// Launch QueryGetSessionRequest in goroutine
	go func() {
		// Close the channel to signal completion of fetch.
		defer close(doneCh)

		req := &sessiontypes.QueryGetSessionRequest{
			ApplicationAddress: appAddress,
			ServiceId:          serviceId,
			BlockHeight:        height,
		}

		// TODO_TECHDEBT(@adshmh): consider increasing the default response size:
		// e.g. using google.golang.org/grpc's MaxCallRecvMsgSize CallOption.
		//
		res, err := s.PoktNodeSessionFetcher.GetSession(ctx, req)
		if err != nil {
			fetchErr = err
			return
		}
		fetchedSession = res.Session
	}()

	// Wait for either result or context deadline
	select {
	case <-doneCh:
		return fetchedSession, fetchErr
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// NewPoktNodeSessionFetcher returns the default implementation of the
// PoktNodeSessionFetcher interface.
// It connects to a POKT full node through the session module's query client
// to get session data.
func NewPoktNodeSessionFetcher(grpcConn grpc.ClientConn) PoktNodeSessionFetcher {
	return sessiontypes.NewQueryClient(grpcConn)
}

// AllEndpoints returns all endpoints corresponding to a session for the service id specified by the session header.
//
// - Endpoints are not filtered.
func (f *SessionFilter) AllEndpoints() (map[SupplierAddress][]Endpoint, error) {
	if f.Session == nil {
		return nil, fmt.Errorf("AllEndpoints: Session not set on FilteredSession struct")
	}

	header := f.Header
	supplierEndpoints := make(map[SupplierAddress][]Endpoint)
	for _, supplier := range f.Suppliers {
		// The endpoints slice is intentionally defined here to prevent any overwrites
		// in the unlikely case that there are duplicate service IDs under a supplier.
		var endpoints []Endpoint
		for _, service := range supplier.Services {
			// TODO_TECHDEBT: Remove this check once the session module ensures that
			// only the services corresponding to the session header are returned.
			if service.ServiceId != f.Header.ServiceId {
				continue
			}

			var newEndpoints []Endpoint
			for _, e := range service.Endpoints {
				newEndpoints = append(newEndpoints, endpoint{
					// TODO_TECHDEBT: Investigate whether we need to do deep copying here and why.
					header:           *header,
					supplierEndpoint: *e,
					supplier:         SupplierAddress(supplier.OperatorAddress),
				})
			}
			endpoints = append(endpoints, newEndpoints...)
		}
		supplierEndpoints[SupplierAddress(supplier.OperatorAddress)] = endpoints
	}

	return supplierEndpoints, nil
}

// FilteredEndpoints returns the endpoints that pass all the filters set of
// the FilteredSession.
func (f *SessionFilter) FilteredEndpoints() ([]Endpoint, error) {
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

// Endpoint is a struct that represents an endpoint with its corresponding
// supplier and session that contains the endpoint.
// It implements the Endpoint interface.
// Endpoint returns the supplier endpoint for the endpoint.
func (e endpoint) Endpoint() sharedtypes.SupplierEndpoint {
	return e.supplierEndpoint
}

// Supplier returns the supplier address for the endpoint.
func (e endpoint) Supplier() SupplierAddress {
	return e.supplier
}

// Header returns the session header on which the supplier's endpoint was retrieved.
func (e endpoint) Header() sessiontypes.SessionHeader {
	return e.header
}
