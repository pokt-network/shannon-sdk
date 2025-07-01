package sdk

import (
	"context"
	"testing"

	sharedtypes "github.com/pokt-network/poktroll/x/shared/types"
	"github.com/stretchr/testify/require"
)

// mockSharedFetcher implements PoktNodeSharedFetcher for testing
type mockSharedFetcher struct {
	params sharedtypes.Params
	err    error
}

func (m *mockSharedFetcher) Params(
	ctx context.Context,
	req *sharedtypes.QueryParamsRequest,
	opts ...interface{},
) (*sharedtypes.QueryParamsResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &sharedtypes.QueryParamsResponse{
		Params: m.params,
	}, nil
}

func TestSharedClient_GetParams(t *testing.T) {
	tests := []struct {
		name           string
		mockParams     sharedtypes.Params
		mockErr        error
		expectedParams sharedtypes.Params
		expectError    bool
	}{
		{
			name: "successful params fetch",
			mockParams: sharedtypes.Params{
				NumBlocksPerSession:            10,
				GracePeriodEndOffsetBlocks:     1,
				ClaimWindowOpenOffsetBlocks:    1,
				ClaimWindowCloseOffsetBlocks:   4,
				ProofWindowOpenOffsetBlocks:    0,
				ProofWindowCloseOffsetBlocks:   4,
				SupplierUnbondingPeriodSessions:   1,
				ApplicationUnbondingPeriodSessions: 1,
				GatewayUnbondingPeriodSessions:    1,
				ComputeUnitsToTokensMultiplier:    42000000,
			},
			expectedParams: sharedtypes.Params{
				NumBlocksPerSession:            10,
				GracePeriodEndOffsetBlocks:     1,
				ClaimWindowOpenOffsetBlocks:    1,
				ClaimWindowCloseOffsetBlocks:   4,
				ProofWindowOpenOffsetBlocks:    0,
				ProofWindowCloseOffsetBlocks:   4,
				SupplierUnbondingPeriodSessions:   1,
				ApplicationUnbondingPeriodSessions: 1,
				GatewayUnbondingPeriodSessions:    1,
				ComputeUnitsToTokensMultiplier:    42000000,
			},
			expectError: false,
		},
		{
			name:        "fetch error",
			mockErr:     require.Error(t, nil, "fetch failed"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockFetcher := &mockSharedFetcher{
				params: tt.mockParams,
				err:    tt.mockErr,
			}

			client := &SharedClient{QueryClient: mockFetcher}
			ctx := context.Background()

			params, err := client.GetParams(ctx)

			if tt.expectError {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.expectedParams.NumBlocksPerSession, params.NumBlocksPerSession)
			require.Equal(t, tt.expectedParams.GracePeriodEndOffsetBlocks, params.GracePeriodEndOffsetBlocks)
			require.Equal(t, tt.expectedParams.ClaimWindowOpenOffsetBlocks, params.ClaimWindowOpenOffsetBlocks)
			require.Equal(t, tt.expectedParams.ComputeUnitsToTokensMultiplier, params.ComputeUnitsToTokensMultiplier)
		})
	}
}

func TestNewPoktNodeSharedFetcher(t *testing.T) {
	// This test verifies that NewPoktNodeSharedFetcher returns a non-nil fetcher
	// We can't test the actual functionality without a real gRPC connection
	fetcher := NewPoktNodeSharedFetcher(nil) // Pass nil for this test
	require.NotNil(t, fetcher)
}