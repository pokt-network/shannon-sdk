package sdk

import (
	"context"

	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
)

// SharedClient interacts with the on-chain shared module.
//
// - Used to get shared module parameters such as grace period, session length, etc.
// - Simple client that directly embeds the QueryClient for straightforward usage
type SharedClient struct {
	sharedtypes.QueryClient
}

// GetParams returns the shared module parameters from the blockchain.
//
// - Returns the current shared module parameters including grace period, session configuration, etc.
// - Returns error if context deadline is exceeded or query fails
func (sc *SharedClient) GetParams(ctx context.Context) (sharedtypes.Params, error) {
	req := &sharedtypes.QueryParamsRequest{}
	res, err := sc.QueryClient.Params(ctx, req)
	if err != nil {
		return sharedtypes.Params{}, err
	}
	return res.Params, nil
}