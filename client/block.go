package client

import (
	"context"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	sdkclient "github.com/cosmos/cosmos-sdk/client"

	"github.com/pokt-network/shannon-sdk/sdk"
)

var _ sdk.BlockClient = (*blockClient)(nil)

type blockClient struct {
	blockQueryClient *rpchttp.HTTP
}

func NewBlockClient(queryNodeRPCUrl string) (sdk.BlockClient, error) {
	blockQueryClient, err := sdkclient.NewClientFromNode(queryNodeRPCUrl)
	if err != nil {
		return nil, err
	}

	return &blockClient{
		blockQueryClient: blockQueryClient,
	}, nil
}

func (bc *blockClient) GetLatestBlockHeight(ctx context.Context) (height int64, err error) {
	block, err := bc.blockQueryClient.Block(ctx, nil)
	if err != nil {
		return 0, err
	}

	return block.Block.Height, nil
}