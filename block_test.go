package sdk

import (
	"context"
	"fmt"
)

func ExampleBlockClient_LatestBlockHeight() {
	poktFullNode, err := NewPoktNodeStatusFetcher("pokt-full-node-URL")
	if err != nil {
		fmt.Printf("Erorr creating a connection to POKT full node: %v\n", err)
		return
	}

	bc := BlockClient{
		PoktNodeStatusFetcher: poktFullNode,
	}

	queryHeight, err := bc.LatestBlockHeight(context.Background())
	if err != nil {
		fmt.Printf("Erorr fetching latest block height: %v\n", err)
		return
	}

	fmt.Printf("Latest block height: %d\n", queryHeight)
}
