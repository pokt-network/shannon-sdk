// Package sdk implements utility functions for interacting with POKT full nodes.
package sdk

import (
	"context"
	"errors"
	"fmt"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	cosmos "github.com/cosmos/cosmos-sdk/client"
)

// TODO_IDEA: The BlockClient could leverage websockets to get notified about new blocks
//   and cache the latest block height to avoid querying the blockchain for it every time.

// A BlockClient is used to interact with the on-chain block module.
// For example, it can be used to get the latest block height.
//
// PoktNodeStatusFetcher specifies the functionality required by the
// BlockClient to interact with a POKT full node.
//
// For obtaining the latest height, BlockClient uses a POKT full
// node's status which contains the latest block height.
// This is done to avoid fetching the entire latest block just to extract the block height.
type BlockClient struct {
	PoktNodeStatusFetcher
}

// LatestBlockHeight returns the height of the latest committed block in the blockchain.
func (bc *BlockClient) LatestBlockHeight(ctx context.Context) (height int64, err error) {
	if bc.PoktNodeStatusFetcher == nil {
		return 0, errors.New("LatestBlockHeight: nil PoktNodeStatusFetcher")
	}

	nodeStatus, err := bc.PoktNodeStatusFetcher.Status(ctx)
	if err != nil {
		return 0, err
	}

	return nodeStatus.SyncInfo.LatestBlockHeight, nil
}

// NewPoktNodeStatusFetcher returns the default implementation of the PoktNodeStatusFetcher interface.
// It connects, through a cometbft RPC HTTP client, to a POKT full node to get its status.
func NewPoktNodeStatusFetcher(queryNodeRpcUrl string) (PoktNodeStatusFetcher, error) {
	// TODO_IMPROVE: drop the cosmos dependency and directly use cometbft rpchttp.New, once the latter publishes a release that includes this functionality.
	// Directly using the cometbft will simplify the code by both reducing imported repos and removing the cosmos wrapper which we don't use.
	// This can be done once there is a cometbft release that includes the following version: github.com/cometbft/cometbft v1.0.0-alpha.2.0.20240530055211-ae27f7eb3c08
	statusFetcher, err := cosmos.NewClientFromNode(queryNodeRpcUrl)
	if err != nil {
		return nil, fmt.Errorf("error constructing a default POKT full node status fetcher: %w", err)
	}

	return statusFetcher, nil
}

// PoktNodeStatusFetcher interface is used by the BlockClient to get the status of a POKT full node.
// The BlokClient extracts the latest height from this status struct.
//
// Most users can rely on the default implementation provided by NewPoktNodeStatusFetcher function.
// A custom implementation of this interface be used to customize the interactions of the BlockClient with the POKT full node.
type PoktNodeStatusFetcher interface {
	Status(ctx context.Context) (*ctypes.ResultStatus, error)
}
