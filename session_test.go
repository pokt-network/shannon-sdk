package sdk

import (
	"context"
	"fmt"
	"github.com/cosmos/gogoproto/grpc"
)

func ExampleLatestBlockHeight() {
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

	for supplierId, endpoints := range ServiceEndpoints(session, "serviceId") {
		for _, e := range endpoints {
			fmt.Printf("Supplier: %s, Endpoint URL: %s\n", supplierId, e.Url)
		}
	}
}
