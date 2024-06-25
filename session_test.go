package sdk

import (
	"context"
	"fmt"

	"github.com/cosmos/gogoproto/grpc"
)

func ExampleGetSession() {
	var grpcConn grpc.ClientConn
	// setup the grpc connection
	// ...

	sc := SessionClient{
		PoktNodeSessionFetcher: NewPoktNodeSessionFetcher(grpcConn),
	}

	session, err := sc.GetSession(context.Background(), "appId", "serviceId", 1)
	if err != nil {
		fmt.Printf("Erorr fetching session: %v\n", err)
		return
	}

	fs := FilteredSession{Session: session}
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
}
