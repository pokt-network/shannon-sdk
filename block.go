package sdk

import (
	"context"

	ctypes "github.com/cometbft/cometbft/rpc/core/types"
	cosmos "github.com/cosmos/cosmos-sdk/client"
)

// NodeStatusFetcher returns the status of the node to which it is connected.
//
// This status is used here in the BlockClient in order to get the latest block height.
type NodeStatusFetcher interface {
	Status(ctx context.Context) (*ctypes.ResultStatus, error)
}

// BlockClient is the interface to interact with the on-chain block module.
//
// For example, it can be used to get the latest block height.
//
// It uses the StatusClient interface of the cometbft to pull the latest block height.
// This is done to avoid fetching the entire latest block just to extract the block height.
//
// The implementations of this interface could leverage websockets to get notified
// about new blocks and cache the latest block height to avoid querying the blockchain
// for it every time.
type BlockClient struct {
	NodeStatusFetcher
}

// NewBlockClient creates a new block client with the provided RPC URL.
func NewBlockClient(queryNodeRPCUrl string) (*BlockClient, error) {
	// TODO: drop the cosmos dependency and directly use cometbft rpchttp.New, once the latter publishes a release that includes this functionality
	statusFetcher, err := cosmos.NewClientFromNode(queryNodeRPCUrl)
	if err != nil {
		return nil, err
	}

	return &BlockClient{
		NodeStatusFetcher: statusFetcher,
	}, nil
}

// GetLatestBlockHeight returns the height of the latest committed block in the blockchain.
func (bc *BlockClient) GetLatestBlockHeight(ctx context.Context) (height int64, err error) {
	nodeStatus, err := bc.NodeStatusFetcher.Status(ctx)
	if err != nil {
		return 0, err
	}

	return nodeStatus.SyncInfo.LatestBlockHeight, nil
}
