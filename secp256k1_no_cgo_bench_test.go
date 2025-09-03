//go:build !cgo
// +build !cgo

// Package sdk provides CGO-free secp256k1 benchmarks for comparison.
//
// This file contains benchmarks that work without CGO enabled,
// comparing CosmosSDK, BTCSuite, and Decred implementations.
package sdk

import (
	"crypto/rand"
	"crypto/sha256"
	"testing"

	// Current Cosmos SDK implementation
	cosmossdk "github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	
	// Alternative CGO-free implementations
	btcsuite "github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	decred "github.com/decred/dcrd/dcrec/secp256k1/v4"
	decred_ecdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	
	"github.com/stretchr/testify/require"
)

const (
	testMessageNoCgo = "test message for CGO-free secp256k1 benchmarking"
)

var (
	testHashNoCgo [32]byte
)

func init() {
	testHashNoCgo = sha256.Sum256([]byte(testMessageNoCgo))
}

// generateRandomBytesNoCgo creates random bytes for key generation
func generateRandomBytesNoCgo(size int) []byte {
	bytes := make([]byte, size)
	_, err := rand.Read(bytes)
	if err != nil {
		panic(err)
	}
	return bytes
}

// CGO-free Key Generation Benchmarks
func BenchmarkKeyGenerationNoCgo_CosmosSDK(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cosmossdk.GenPrivKey()
	}
}

func BenchmarkKeyGenerationNoCgo_BTCSuite(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		privKeyBytes := generateRandomBytesNoCgo(32)
		_, _ = btcsuite.PrivKeyFromBytes(privKeyBytes)
	}
}

func BenchmarkKeyGenerationNoCgo_Decred(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		privKeyBytes := generateRandomBytesNoCgo(32)
		_ = decred.PrivKeyFromBytes(privKeyBytes)
	}
}

// CGO-free Signing Benchmarks
func BenchmarkSigningNoCgo_CosmosSDK(b *testing.B) {
	privKey := cosmossdk.GenPrivKey()
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := privKey.Sign(testHashNoCgo[:])
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSigningNoCgo_BTCSuite(b *testing.B) {
	privKeyBytes := generateRandomBytesNoCgo(32)
	privKey, _ := btcsuite.PrivKeyFromBytes(privKeyBytes)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ecdsa.Sign(privKey, testHashNoCgo[:])
	}
}

func BenchmarkSigningNoCgo_Decred(b *testing.B) {
	privKeyBytes := generateRandomBytesNoCgo(32)
	privKey := decred.PrivKeyFromBytes(privKeyBytes)
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = decred_ecdsa.Sign(privKey, testHashNoCgo[:])
	}
}

// CGO-free Verification Benchmarks
func BenchmarkVerificationNoCgo_CosmosSDK(b *testing.B) {
	privKey := cosmossdk.GenPrivKey()
	pubKey := privKey.PubKey()
	signature, _ := privKey.Sign(testHashNoCgo[:])
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid := pubKey.VerifySignature(testHashNoCgo[:], signature)
		if !valid {
			b.Fatal("signature verification failed")
		}
	}
}

func BenchmarkVerificationNoCgo_BTCSuite(b *testing.B) {
	privKeyBytes := generateRandomBytesNoCgo(32)
	privKey, pubKey := btcsuite.PrivKeyFromBytes(privKeyBytes)
	signature := ecdsa.Sign(privKey, testHashNoCgo[:])
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid := signature.Verify(testHashNoCgo[:], pubKey)
		if !valid {
			b.Fatal("signature verification failed")
		}
	}
}

func BenchmarkVerificationNoCgo_Decred(b *testing.B) {
	privKeyBytes := generateRandomBytesNoCgo(32)
	privKey := decred.PrivKeyFromBytes(privKeyBytes)
	pubKey := privKey.PubKey()
	signature := decred_ecdsa.Sign(privKey, testHashNoCgo[:])
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid := signature.Verify(testHashNoCgo[:], pubKey)
		if !valid {
			b.Fatal("signature verification failed")
		}
	}
}

// TestCompatibilityNoCgo ensures all CGO-free libraries produce valid results
func TestCompatibilityNoCgo(t *testing.T) {
	// Generate a test private key
	privKeyBytes := generateRandomBytesNoCgo(32)
	
	// Test Cosmos SDK
	cosmosPrivKey := &cosmossdk.PrivKey{Key: privKeyBytes}
	cosmosPubKey := cosmosPrivKey.PubKey()
	cosmosSignature, err := cosmosPrivKey.Sign(testHashNoCgo[:])
	require.NoError(t, err)
	require.True(t, cosmosPubKey.VerifySignature(testHashNoCgo[:], cosmosSignature))
	
	// Test btcsuite
	btcPrivKey, btcPubKey := btcsuite.PrivKeyFromBytes(privKeyBytes)
	btcSignature := ecdsa.Sign(btcPrivKey, testHashNoCgo[:])
	require.True(t, btcSignature.Verify(testHashNoCgo[:], btcPubKey))
	
	// Test Decred
	decredPrivKey := decred.PrivKeyFromBytes(privKeyBytes)
	decredPubKey := decredPrivKey.PubKey()
	decredSignature := decred_ecdsa.Sign(decredPrivKey, testHashNoCgo[:])
	require.True(t, decredSignature.Verify(testHashNoCgo[:], decredPubKey))
	
	t.Log("All CGO-free secp256k1 implementations produce valid signatures")
}