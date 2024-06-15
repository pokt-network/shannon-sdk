package sdk

import (
	"context"

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
	PoktrollSessionFetcher
}

// PoktrollSessionFetcher is used by the SessionClient to fetch sessions using poktroll request/response types.
//
// It is defined to allow customizing the interactions of SessionClient with poktroll, without affecting
// the functionality/interface of SessionClient.
type PoktrollSessionFetcher interface {
	GetSession(
		context.Context,
		*sessiontypes.QueryGetSessionRequest,
		...grpcoptions.CallOption,
	) (*sessiontypes.QueryGetSessionResponse, error)
}

// NewSessionClient creates a new session client with the provided gRPC connection.
//
// It builds an instance of SessionClient that utilizes the gRPC query client of the session module.
func NewSessionClient(grpcConn grpc.ClientConn) *SessionClient {
	return &SessionClient{
		PoktrollSessionFetcher: sessiontypes.NewQueryClient(grpcConn),
	}
}

// GetSession returns the session with the given application address, service id and height.
func (s *SessionClient) GetSession(
	ctx context.Context,
	appAddress string,
	serviceId string,
	height int64,
) (session *sessiontypes.Session, err error) {
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
	res, err := s.PoktrollSessionFetcher.GetSession(ctx, req)
	if err != nil {
		return nil, err
	}

	return res.Session, nil
}

// TODO_DISCUSS: ServiceEndpoints can be replaced by a method on the Session struct.
// TODO_DISCUSS: Consider using a custom type, defined as a string, as supplier address.
//
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
			if service.Service.Id != serviceId {
				continue
			}

			endpoints = append(endpoints, service.Endpoints...)
		}
		supplierEndpoints[supplier.Address] = endpoints
	}

	return supplierEndpoints
}
