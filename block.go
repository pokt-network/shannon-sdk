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
// and cache the latest block height to avoid querying the blockchain for it every time.

// PoktNodeStatusFetcher interface is used by the BlockClient to get the status of a POKT full node.
//
// - The BlockClient extracts the latest height from this status struct.
// - Most users can rely on the default implementation provided by NewPoktNodeStatusFetcher.
// - A custom implementation can be used for more granular control over the BlockClient's interactions with the POKT full node.
type PoktNodeStatusFetcher interface {
	Status(ctx context.Context) (*ctypes.ResultStatus, error)
}

// BlockClient is a concrete type used to interact with the on-chain block module.
//
// - Can be used to get the latest block height.
// - Uses a POKT full node's status to obtain the latest height, avoiding fetching the entire block just for the height.
type BlockClient struct {
	// PoktNodeStatusFetcher specifies the functionality required by the BlockClient to interact with a POKT full node.
	PoktNodeStatusFetcher
}

// LatestBlockHeight returns the height of the latest committed block in the blockchain.
func (bc *BlockClient) LatestBlockHeight(ctx context.Context) (height int64, err error) {
	if bc.PoktNodeStatusFetcher == nil {
		return 0, errors.New("LatestBlockHeight: nil PoktNodeStatusFetcher")
	}

	nodeStatus, err := bc.Status(ctx)
	if err != nil {
		return 0, err
	}

	return nodeStatus.SyncInfo.LatestBlockHeight, nil
}

// NewPoktNodeStatusFetcher returns the default implementation of the PoktNodeStatusFetcher interface.
//
// - Connects, through a cometbft RPC HTTP client, to a POKT full node to get its status.
// - TODO_IMPROVE: Drop the cosmos dependency and directly use cometbft rpchttp.New once a compatible release is available.
//   - This will simplify the code by reducing imported repos and removing the unused cosmos wrapper.
//   - Target cometbft version: github.com/cometbft/cometbft v1.0.0-alpha.2.0.20240530055211-ae27f7eb3c08
func NewPoktNodeStatusFetcher(queryNodeRpcUrl string) (PoktNodeStatusFetcher, error) {
	statusFetcher, err := cosmos.NewClientFromNode(queryNodeRpcUrl)
	if err != nil {
		return nil, fmt.Errorf("error constructing a default POKT full node status fetcher: %w", err)
	}

	return statusFetcher, nil
}
