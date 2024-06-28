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
		PoktNodeSessionFetcher: NewPoktNodeSessionFetcher(grpcConn),
	}

	// Get the session.
	session, err := sc.GetSession(context.Background(), "appId", "serviceId", 1)
	if err != nil {
		fmt.Printf("Erorr fetching session: %v\n", err)
		return
	}

	// Get all the endpoints for the session.
	fs := SessionFilter{
		Session:         session,
		EndpointFilters: []EndpointFilter{},
	}
	serviceEndpoints, err := fs.AllEndpoints()
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

	fs.EndpointFilters = append(fs.EndpointFilters, filterEndpoints)

	filteredEndpoints, err := fs.FilteredEndpoints()
	for _, endpoint := range filteredEndpoints {
		fmt.Printf(
			"Supplier: %s, Endpoint URL: %s\n",
			endpoint.Supplier(),
			endpoint.Endpoint().Url,
		)
	}
}
