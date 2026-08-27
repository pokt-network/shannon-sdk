package sdk

import (
	"testing"

	"github.com/pokt-network/poktroll/app"
	"github.com/stretchr/testify/require"
)

// TestAccountAddressPrefixMatchesPoktroll guards against drift between the SDK's
// local accountAddressPrefix constant and poktroll's canonical app.AccountAddressPrefix.
//
// The constant is duplicated (instead of imported) to keep the heavy poktroll/app
// package out of the non-test dependency graph of SDK consumers. Test-only imports
// do not propagate to downstream binaries, so it is safe to import it here.
func TestAccountAddressPrefixMatchesPoktroll(t *testing.T) {
	require.Equal(t, app.AccountAddressPrefix, accountAddressPrefix)
}
