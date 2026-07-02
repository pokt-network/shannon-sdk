package sdk

import (
	"context"
	"encoding/hex"
	"testing"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	ring "github.com/pokt-network/ring-go"
	"github.com/stretchr/testify/require"
)

// setupSignerTestData mirrors setupBenchmarkData but for *testing.T.
func setupSignerTestData(t *testing.T) (*Signer, *servicetypes.RelayRequest, *ApplicationRing) {
	t.Helper()

	appPrivKey := secp256k1.GenPrivKey()
	supplierPrivKey1 := secp256k1.GenPrivKey()
	supplierPrivKey2 := secp256k1.GenPrivKey()

	signer, err := NewSignerFromHex(hex.EncodeToString(appPrivKey.Bytes()))
	require.NoError(t, err)

	pubKeyFetcher := &mockPublicKeyFetcher{
		publicKeys: map[string]cryptotypes.PubKey{
			"pokt1app1":      appPrivKey.PubKey(),
			"pokt1supplier1": supplierPrivKey1.PubKey(),
			"pokt1supplier2": supplierPrivKey2.PubKey(),
		},
	}

	appRing := &ApplicationRing{
		Application:      apptypes.Application{Address: "pokt1app1"},
		PublicKeyFetcher: pubKeyFetcher,
	}

	relayRequest := &servicetypes.RelayRequest{
		Meta: servicetypes.RelayRequestMetadata{
			SessionHeader: &sessiontypes.SessionHeader{
				ApplicationAddress:      "pokt1app1",
				ServiceId:               "test-service",
				SessionStartBlockHeight: 1,
				SessionEndBlockHeight:   10,
			},
			SupplierOperatorAddress: "pokt1supplier1",
		},
		Payload: []byte(`{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}`),
	}

	return signer, relayRequest, appRing
}

func cachedContextCount(s *Signer) int {
	count := 0
	s.signerContextCache.Range(func(_, _ any) bool { count++; return true })
	return count
}

// TestSignerSignRoundTrip proves the deterministic (consensus-safe) default path
// produces verifiable signatures, rejects a tampered message, and never touches
// the off-chain SignerContext cache.
func TestSignerSignRoundTrip(t *testing.T) {
	ctx := context.Background()
	signer, relayRequest, appRing := setupSignerTestData(t)

	signableBz, err := relayRequest.GetSignableBytesHash()
	require.NoError(t, err)

	signed, err := signer.Sign(ctx, relayRequest, appRing)
	require.NoError(t, err)
	require.NotEmpty(t, signed.Meta.Signature)

	var sig ring.RingSig
	require.NoError(t, sig.Deserialize(ring.Secp256k1(), signed.Meta.Signature))
	require.True(t, sig.Verify(signableBz), "deterministic signature must verify")

	// Negative case: must not verify against a tampered message hash.
	wrong := signableBz
	wrong[0] ^= 0xFF
	require.False(t, sig.Verify(wrong), "signature must not verify against a different message")

	// The deterministic path must not populate the off-chain context cache.
	require.Equal(t, 0, cachedContextCount(signer), "Sign must not use the context cache")
}

// TestSignerSignOffChainRoundTrip proves the off-chain path (per-call key +
// cached SignerContext + SkipSelfCheck) produces verifiable signatures and
// populates the cache.
func TestSignerSignOffChainRoundTrip(t *testing.T) {
	ctx := context.Background()
	signer, relayRequest, appRing := setupSignerTestData(t)

	signableBz, err := relayRequest.GetSignableBytesHash()
	require.NoError(t, err)

	signed, err := signer.SignOffChain(ctx, relayRequest, appRing)
	require.NoError(t, err)
	require.NotEmpty(t, signed.Meta.Signature)

	var sig ring.RingSig
	require.NoError(t, sig.Deserialize(ring.Secp256k1(), signed.Meta.Signature))
	require.True(t, sig.Verify(signableBz), "off-chain signature must verify")

	require.Equal(t, 1, cachedContextCount(signer), "SignOffChain must cache one context for the ring")
}

// TestSignerOffChainCacheReuse proves repeated off-chain signs reuse a single
// cached SignerContext for one ring, every signature verifies, and clearing
// empties the cache.
func TestSignerOffChainCacheReuse(t *testing.T) {
	ctx := context.Background()
	signer, relayRequest, appRing := setupSignerTestData(t)

	sessionRing, err := appRing.GetRing(ctx, uint64(relayRequest.Meta.SessionHeader.SessionEndBlockHeight))
	require.NoError(t, err)

	for i := 0; i < 3; i++ {
		signableBz, err := relayRequest.GetSignableBytesHash()
		require.NoError(t, err)

		signed, err := signer.SignOffChainWithRing(ctx, relayRequest, sessionRing)
		require.NoError(t, err)

		var sig ring.RingSig
		require.NoError(t, sig.Deserialize(ring.Secp256k1(), signed.Meta.Signature))
		require.True(t, sig.Verify(signableBz), "off-chain sign #%d must verify", i)
	}

	require.Equal(t, 1, cachedContextCount(signer), "expected a single cached SignerContext for one ring")

	// Clearing the cache empties it.
	signer.ClearSignerContextCache()
	require.Equal(t, 0, cachedContextCount(signer), "cache must be empty after ClearSignerContextCache")
}
