package sdk

import (
	"context"
	"fmt"

	"github.com/cosmos/gogoproto/grpc"
	"github.com/pokt-network/poktroll/x/shared/types"
)

func ExampleSessionClient() {
	// Initialize the SessionClient.

	var grpcConn grpc.ClientConn
	// setup the grpc connection
	// ...

	sc := SessionClient{
		// Use the default implementation of the PoktNodeSessionFetcher interface.
		PoktNodeSessionFetcher: NewPoktNodeSessionFetcher(grpcConn),
	}

	// Get the session.
	session, err := sc.GetSession(context.Background(), "appId", "serviceId", 1)
	if err != nil {
		fmt.Printf("Erorr fetching session: %v\n", err)
		return
	}

	// Get all the endpoints for the session.
	sessionFilter := SessionFilter{
		Session:         session,
		EndpointFilters: []EndpointFilter{},
	}
	serviceEndpoints, err := sessionFilter.AllEndpoints()
	if err != nil {
		fmt.Printf("Error getting service endpoints: %v\n", err)
		return
	}

	for supplierId, endpoints := range serviceEndpoints {
		for _, e := range endpoints {
			fmt.Printf("Supplier: %s, Endpoint URL: %s\n", supplierId, e.Endpoint().Url)
		}
	}

	// Use a filter to get only the endpoints that satisfy the filter.
	filterEndpoints := func(e Endpoint) bool {
		return e.Endpoint().RpcType == types.RPCType_JSON_RPC
	}

	sessionFilter.EndpointFilters = append(sessionFilter.EndpointFilters, filterEndpoints)

	filteredEndpoints, err := sessionFilter.FilteredEndpoints()
	if err != nil {
		fmt.Printf("Error filtering service endpoints: %v\n", err)
		return
	}

	for _, endpoint := range filteredEndpoints {
		fmt.Printf(
			"Supplier: %s, Endpoint URL: %s\n",
			endpoint.Supplier(),
			endpoint.Endpoint().Url,
		)
	}
}
