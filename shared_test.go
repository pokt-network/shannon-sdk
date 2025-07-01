package sdk

import (
	"context"
	"fmt"
	"testing"

	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
	"github.com/stretchr/testify/require"
)

func TestSharedClient_GetParams_Success(t *testing.T) {
	expectedParams := sharedtypes.Params{
		NumBlocksPerSession:               10,
		GracePeriodEndOffsetBlocks:        1,
		ClaimWindowOpenOffsetBlocks:       1,
		ClaimWindowCloseOffsetBlocks:      4,
		ProofWindowOpenOffsetBlocks:       0,
		ProofWindowCloseOffsetBlocks:      4,
		SupplierUnbondingPeriodSessions:   1,
		ApplicationUnbondingPeriodSessions: 1,
		GatewayUnbondingPeriodSessions:    1,
		ComputeUnitsToTokensMultiplier:    42000000,
	}

	mockQueryClient := &testSharedQueryClient{
		paramsResponse: &sharedtypes.QueryParamsResponse{
			Params: expectedParams,
		},
	}

	sharedClient := &SharedClient{QueryClient: mockQueryClient}
	ctx := context.Background()

	actualParams, err := sharedClient.GetParams(ctx)

	require.NoError(t, err)
	require.Equal(t, expectedParams.NumBlocksPerSession, actualParams.NumBlocksPerSession)
	require.Equal(t, expectedParams.GracePeriodEndOffsetBlocks, actualParams.GracePeriodEndOffsetBlocks)
	require.Equal(t, expectedParams.ClaimWindowOpenOffsetBlocks, actualParams.ClaimWindowOpenOffsetBlocks)
	require.Equal(t, expectedParams.ComputeUnitsToTokensMultiplier, actualParams.ComputeUnitsToTokensMultiplier)
}

func TestSharedClient_GetParams_Error(t *testing.T) {
	expectedErr := fmt.Errorf("test error")
	mockQueryClient := &testSharedQueryClient{
		err: expectedErr,
	}

	sharedClient := &SharedClient{QueryClient: mockQueryClient}
	ctx := context.Background()

	_, err := sharedClient.GetParams(ctx)

	require.Error(t, err)
	require.Equal(t, expectedErr, err)
}

// testSharedQueryClient is a mock implementation of sharedtypes.QueryClient for testing
type testSharedQueryClient struct {
	paramsResponse *sharedtypes.QueryParamsResponse
	err            error
}

func (m *testSharedQueryClient) Params(
	ctx context.Context,
	req *sharedtypes.QueryParamsRequest,
	opts ...interface{},
) (*sharedtypes.QueryParamsResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.paramsResponse, nil
}