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
    "github.com/pokt-network/ring-go"
    "github.com/stretchr/testify/require"
)

// mockPublicKeyFetcher is a test implementation of PublicKeyFetcher
type mockPublicKeyFetcher struct {
	publicKeys map[string]cryptotypes.PubKey
}

func (m *mockPublicKeyFetcher) GetPubKeyFromAddress(ctx context.Context, address string) (cryptotypes.PubKey, error) {
	if pubKey, exists := m.publicKeys[address]; exists {
		return pubKey, nil
	}
	return nil, nil
}

// setupBenchmarkData creates test data for benchmarks
func setupBenchmarkData(b *testing.B) (*Signer, *servicetypes.RelayRequest, *ApplicationRing) {
	// Generate test private keys
	appPrivKey := secp256k1.GenPrivKey()
	supplierPrivKey1 := secp256k1.GenPrivKey()
	supplierPrivKey2 := secp256k1.GenPrivKey()

	// Use the app private key for signing (convert to hex)
	privateKeyHex := hex.EncodeToString(appPrivKey.Bytes())

	signer, err := NewSignerFromHex(privateKeyHex)
	if err != nil {
		b.Fatalf("Failed to create signer: %v", err)
	}

	// Create a mock public key fetcher with corresponding public keys
	pubKeyFetcher := &mockPublicKeyFetcher{
		publicKeys: map[string]cryptotypes.PubKey{
			"pokt1app1":      appPrivKey.PubKey(),
			"pokt1supplier1": supplierPrivKey1.PubKey(),
			"pokt1supplier2": supplierPrivKey2.PubKey(),
		},
	}

	// Create an application
	app := apptypes.Application{
		Address: "pokt1app1",
	}

    appRing := &ApplicationRing{
        Application:      app,
        PublicKeyFetcher: pubKeyFetcher,
    }

	// Create a relay request
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

// BenchmarkSign measures the performance of the Sign method
func BenchmarkSign(b *testing.B) {
	ctx := context.Background()
	signer, relayRequest, appRing := setupBenchmarkData(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
        _, err := signer.Sign(ctx, relayRequest, appRing)
		if err != nil {
			b.Fatalf("Sign failed: %v", err)
		}
	}
}

// BenchmarkSignParallel measures the performance of Sign with parallel execution
func BenchmarkSignParallel(b *testing.B) {
	ctx := context.Background()

	b.RunParallel(func(pb *testing.PB) {
		signer, relayRequest, appRing := setupBenchmarkData(b)
		for pb.Next() {
            _, err := signer.Sign(ctx, relayRequest, appRing)
			if err != nil {
				b.Fatalf("Sign failed: %v", err)
			}
		}
	})
}

// BenchmarkPrivateKeyDecoding measures the overhead of hex decoding
func BenchmarkPrivateKeyDecoding(b *testing.B) {
	// Generate a valid private key for benchmarking
	privKey := secp256k1.GenPrivKey()
	privateKeyHex := hex.EncodeToString(privKey.Bytes())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		signerPrivKeyBz, err := hex.DecodeString(privateKeyHex)
		if err != nil {
			b.Fatalf("hex decode failed: %v", err)
		}

		_, err = ring.Secp256k1().DecodeToScalar(signerPrivKeyBz)
		if err != nil {
			b.Fatalf("decode to scalar failed: %v", err)
		}
	}
}

// BenchmarkSignWithCachedPrivateKey simulates improved performance with cached private key
func BenchmarkSignWithCachedPrivateKey(b *testing.B) {
	ctx := context.Background()
	signer, relayRequest, appRing := setupBenchmarkData(b)

	// Pre-decode the private key (simulating the improvement suggested in TODO)
	signerPrivKeyBz, err := hex.DecodeString(signer.PrivateKeyHex)
	require.NoError(b, err)
	signerPrivKey, err := ring.Secp256k1().DecodeToScalar(signerPrivKeyBz)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
        // Get the session ring
        sessionRing, err := appRing.GetRing(ctx, uint64(relayRequest.Meta.SessionHeader.SessionEndBlockHeight))
        if err != nil {
            b.Fatalf("GetRing failed: %v", err)
        }

		// Get signable bytes
		signableBz, err := relayRequest.GetSignableBytesHash()
		if err != nil {
			b.Fatalf("GetSignableBytesHash failed: %v", err)
		}

		// Sign with pre-decoded key
		ringSig, err := sessionRing.Sign(signableBz, signerPrivKey)
		if err != nil {
			b.Fatalf("Sign failed: %v", err)
		}

		// Serialize
		signature, err := ringSig.Serialize()
		if err != nil {
			b.Fatalf("Serialize failed: %v", err)
		}

		relayRequest.Meta.Signature = signature
	}
}

// BenchmarkSignLargePayload measures performance with larger relay payloads
func BenchmarkSignLargePayload(b *testing.B) {
	ctx := context.Background()
	signer, relayRequest, appRing := setupBenchmarkData(b)

	// Create a large payload (10KB)
	largePayload := make([]byte, 10240)
	for i := range largePayload {
		largePayload[i] = byte(i % 256)
	}
	relayRequest.Payload = largePayload

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
        _, err := signer.Sign(ctx, relayRequest, appRing)
		if err != nil {
			b.Fatalf("Sign failed: %v", err)
		}
	}
}

// BenchmarkGetSignableBytesHash measures the performance of getting signable bytes
func BenchmarkGetSignableBytesHash(b *testing.B) {
	_, relayRequest, _ := setupBenchmarkData(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := relayRequest.GetSignableBytesHash()
		if err != nil {
			b.Fatalf("GetSignableBytesHash failed: %v", err)
		}
	}
}

// BenchmarkSerializeSignature measures signature serialization performance
func BenchmarkSerializeSignature(b *testing.B) {
	ctx := context.Background()
	signer, relayRequest, appRing := setupBenchmarkData(b)

	// Prepare everything needed for signing
    sessionRing, _ := appRing.GetRing(ctx, uint64(relayRequest.Meta.SessionHeader.SessionEndBlockHeight))
	signableBz, _ := relayRequest.GetSignableBytesHash()
	signerPrivKeyBz, _ := hex.DecodeString(signer.PrivateKeyHex)
	signerPrivKey, _ := ring.Secp256k1().DecodeToScalar(signerPrivKeyBz)
	ringSig, _ := sessionRing.Sign(signableBz, signerPrivKey)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ringSig.Serialize()
		if err != nil {
			b.Fatalf("Serialize failed: %v", err)
		}
	}
}

// BenchmarkSignMemoryAllocation measures memory allocations during signing
func BenchmarkSignMemoryAllocation(b *testing.B) {
	ctx := context.Background()
	signer, relayRequest, appRing := setupBenchmarkData(b)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := signer.Sign(ctx, relayRequest, appRing)
		if err != nil {
			b.Fatalf("Sign failed: %v", err)
		}
	}
}

// sink variables to prevent compiler from optimizing away results
var (
    sinkSigBz []byte
)

// BenchmarkSignCore isolates the core ring signature path to better reflect
// crypto backend differences (portable vs ethereum).
func BenchmarkSignCore(b *testing.B) {
    ctx := context.Background()
    signer, relayRequest, appRing := setupBenchmarkData(b)

    // Precompute session ring, signable hash, and decode scalar once
    sessionRing, err := appRing.GetRing(ctx, uint64(relayRequest.Meta.SessionHeader.SessionEndBlockHeight))
    require.NoError(b, err)

    signableBz, err := relayRequest.GetSignableBytesHash()
    require.NoError(b, err)

    keyBz, err := hex.DecodeString(signer.PrivateKeyHex)
    require.NoError(b, err)

    scalar, err := ring.Secp256k1().DecodeToScalar(keyBz)
    require.NoError(b, err)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        sig, err := sessionRing.Sign(signableBz, scalar)
        if err != nil {
            b.Fatalf("Sign failed: %v", err)
        }
        bz, err := sig.Serialize()
        if err != nil {
            b.Fatalf("Serialize failed: %v", err)
        }
        sinkSigBz = bz
    }
}

// BenchmarkSignReuseRing reuses the computed ring across iterations to reduce
// benchmark overhead unrelated to crypto and better expose backend differences.
func BenchmarkSignReuseRing(b *testing.B) {
    ctx := context.Background()
    signer, relayRequest, appRing := setupBenchmarkData(b)

    sessionRing, err := appRing.GetRing(ctx, uint64(relayRequest.Meta.SessionHeader.SessionEndBlockHeight))
    require.NoError(b, err)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, err := signer.Sign(ctx, relayRequest, appRing)
        if err != nil {
            b.Fatalf("Sign failed: %v", err)
        }
        _ = sessionRing // ensure not optimized out
    }
}
