package sdk

import (
	"context"
	"errors"

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

// TODO_DISCUSS: ServiceEndpoints can be replaced by a method on the Session struct.
// TODO_DISCUSS: Consider using a custom type, defined as a string, as supplier address.
// This can help enforce type safety by requiring explict type casting of a string before it can be used as a SupplierAddress.
//
// ServiceEndpoints returns the supplier endpoints assigned to a session for the given service id.
//
// The returned value is a map of supplierId to the list of endpoints.
func ServiceEndpoints(
	session *sessiontypes.Session,
	serviceId string,
) map[string][]*sharedtypes.SupplierEndpoint {
	supplierEndpoints := make(map[string][]*sharedtypes.SupplierEndpoint)
	for _, supplier := range session.Suppliers {
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

	return supplierEndpoints
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
