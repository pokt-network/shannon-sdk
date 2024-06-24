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

// TODO_IMPROVE: add a `FilterSupplier` method to drop a supplier from consideration when sending a relay.
// TODO_IMPROVE: add a `FilterEndpoint` method to drop an endpoint from consideration when sending a relay.
// TODO_IMPROVE: add an OrderEndpoints to allow ordering the list of available, i.e. not filtered out, endpoints.
// The `SelectedEndpoint` method (and potentially a `SelectedEndpoints` method) should then return the ordered list of endpoints for the target service.
//
// FilteredSession wraps a Session, allowing node selection, currently through directly adding endpoints, and potentially through filtering/ordering in future versions.
// This is needed so functions that enable sending relays can be provided with a struct that contains both session data and the endpoint(s) selected for receiving relays.
type FilteredSession struct {
	*sessiontypes.Session

	// selectedEndpoints is the set of a specific service's endpoints, from the underlying session, that are selected by the user of FilteredNodes.
	// This selection can be for any purpose, e.g. sending relays.
	// The set of selected endpoints is stored as a map of supplier addresses to a list of endpoints.
	selectedEndpoints map[string][]*sharedtypes.SupplierEndpoint

	// selectedServiceId stores the ID of the service selected by the user.
	selectedServiceId string
}

// TODO_DISCUSS: Consider using a custom type, defined as a string, as supplier address.
// This can help enforce type safety by requiring explict type casting of a string before it can be used as a SupplierAddress.
//
// ServiceEndpoints returns the supplier endpoints assigned to a session for the given service id.
//
// The returned value is a map of supplierId to the list of endpoints.
func (f *FilteredSession) ServiceEndpoints(
	serviceId string,
) (map[string][]*sharedtypes.SupplierEndpoint, error) {
	if f.Session == nil {
		return nil, fmt.Errorf("ServiceEndpoints: Session not set on FilteredSession struct")
	}

	supplierEndpoints := make(map[string][]*sharedtypes.SupplierEndpoint)
	for _, supplier := range f.Session.Suppliers {
		// TODO_DISCUSS: It may be a good idea to use a map[serviceID][]Endpoint under each supplier,
		//   especially if service IDs under each supplier should be unique.
		//   This would enable easy lookups based on service ID, without impacting any use cases that
		//   need to iterate through all the services.
		//
		// The endpoins slice is intentionally defined here to prevent any overwrites in the unlikely case
		//   that there are duplicate service IDs under a supplier.
		var endpoints []*sharedtypes.SupplierEndpoint
		for _, service := range supplier.Services {
			// TODO_DISCUSS: considering a service Id is required to get a session, does it make sense for the suppliers under the session
			// to only contain endpoints for the (single) service covered by the session?
			if service.Service.Id != serviceId {
				continue
			}

			endpoints = append(endpoints, service.Endpoints...)
		}
		supplierEndpoints[supplier.Address] = endpoints
	}

	return supplierEndpoints, nil
}

// SelectEndpoint adds the specifed supplier+endpoint combination to the list of selected endpoints of a service, e.g. for sending relays.
// This method is not safe to use concurrently by multiple goroutines.
func (f *FilteredSession) AddEndpointToSelection(serviceId string, supplierAddress string, endpoint *sharedtypes.SupplierEndpoint) error {
	if serviceId == "" {
		return errors.New("Cannot add endpoint to selection without a service ID.")
	}

	if f.selectedServiceId != "" && serviceId != f.selectedServiceId {
		return fmt.Errorf("Cannot add endpoint to selection: already selected service ID %s does not match supplied service ID %s",
			f.selectedServiceId,
			serviceId,
		)
	}

	if supplierAddress == "" {
		return errors.New("Cannot add endpoint to selection without a supplier ID.")
	}

	var supplier *sharedtypes.Supplier
	for _, s := range f.Session.GetSuppliers() {
		if s.Address == supplierAddress {
			supplier = s
			break
		}
	}
	if supplier == nil {
		return fmt.Errorf("Cannot add endpoint to selection: supplier %s not found.", supplierAddress)
	}

	var serviceIdMatched bool
	for _, serviceConfig := range supplier.Services {
		if serviceConfig.Service.GetId() == serviceId {
			serviceIdMatched = true
			break
		}
	}
	if !serviceIdMatched {
		return fmt.Errorf("Cannot add endpoint to selection: supplier %s does not provide service %s", supplier.Address, serviceId)
	}

	// TODO_TECHDEBT: verify the input endpoint is included in the session.

	// Input passed all validation, add the input endpoint to selection
	f.selectedServiceId = serviceId
	if f.selectedEndpoints == nil {
		f.selectedEndpoints = make(map[string][]*sharedtypes.SupplierEndpoint)
	}

	f.selectedEndpoints[supplier.Address] = append(f.selectedEndpoints[supplier.Address], endpoint)
	return nil
}

// SelectedEndpoint returns the supplier address and the selected supplier endpoint.
func (f *FilteredSession) SelectedEndpoint() (*sharedtypes.SupplierEndpoint, error) {
	if len(f.selectedEndpoints) == 0 {
		return nil, errors.New("no endpoint has been marked as selected")
	}

	for _, endpoints := range f.selectedEndpoints {
		if len(endpoints) > 0 {
			// TODO_IMPROVE: once FilteredSession supports ordering of the endpoints, this function needs to return the best endpoint.
			return endpoints[0], nil
		}
	}

	return nil, fmt.Errorf("could not find any selected endpoints for service ID: %s", f.selectedServiceId)
}

func (f *FilteredSession) SelectedSupplierAddress() (string, error) {
	if len(f.selectedEndpoints) == 0 {
		return "", errors.New("Error finding the selected supplier address: no endpoint has been marked as selected")
	}

	for supplierAddress, endpoints := range f.selectedEndpoints {
		if len(endpoints) > 0 {
			return supplierAddress, nil
		}
	}

	return "", fmt.Errorf("could not find any selected endpoints for service ID: %s", f.selectedServiceId)
}

func (f *FilteredSession) SessionHeader() (*sessiontypes.SessionHeader, error) {
	if f.Session == nil {
		return nil, errors.New("SessionHeader: Supporting session not set on FilteredSession struct")
	}

	return f.Session.GetHeader(), nil
}
