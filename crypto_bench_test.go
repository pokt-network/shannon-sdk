package sdk

import (
	"context"
	"crypto/sha256"
	"testing"

	"github.com/stretchr/testify/require"
)

// setupCryptoBenchmarkData creates test data for crypto backend benchmarks
func setupCryptoBenchmarkData(t testing.TB) (*Signer, []byte, context.Context) {
	// Use a test private key
	testPrivateKey := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	signer, err := NewSignerFromHex(testPrivateKey)
	require.NoError(t, err)

	// Create a test hash
	testMessage := []byte("Shannon SDK crypto backend benchmark test message")
	testHash := sha256.Sum256(testMessage)

	return signer, testHash[:], context.Background()
}

// BenchmarkCryptoBackend_PrivateKeyDecoding measures private key decoding performance
func BenchmarkCryptoBackend_PrivateKeyDecoding(b *testing.B) {
	testPrivateKey := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"
	signer, _, _ := setupCryptoBenchmarkData(b)
	cryptoSigner := signer.GetCryptoSigner()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := cryptoSigner.DecodePrivateKey(testPrivateKey)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCryptoBackend_BasicSigning measures basic signature creation performance
func BenchmarkCryptoBackend_BasicSigning(b *testing.B) {
	signer, testHash, _ := setupCryptoBenchmarkData(b)
	cryptoSigner := signer.GetCryptoSigner()
	privateKey, err := cryptoSigner.DecodePrivateKey(signer.PrivateKeyHex)
	require.NoError(b, err)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := privateKey.Sign(testHash)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkCryptoBackend_BasicVerification measures basic signature verification performance
func BenchmarkCryptoBackend_BasicVerification(b *testing.B) {
	signer, testHash, _ := setupCryptoBenchmarkData(b)
	cryptoSigner := signer.GetCryptoSigner()
	privateKey, err := cryptoSigner.DecodePrivateKey(signer.PrivateKeyHex)
	require.NoError(b, err)

	// Pre-generate signature and public key
	signature, err := privateKey.Sign(testHash)
	require.NoError(b, err)
	publicKey := privateKey.PubKey()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		valid := publicKey.Verify(testHash, signature)
		if !valid {
			b.Fatal("signature verification failed")
		}
	}
}

// BenchmarkCryptoBackend_SignAndVerify measures complete sign+verify cycle
func BenchmarkCryptoBackend_SignAndVerify(b *testing.B) {
	signer, testHash, _ := setupCryptoBenchmarkData(b)
	cryptoSigner := signer.GetCryptoSigner()
	privateKey, err := cryptoSigner.DecodePrivateKey(signer.PrivateKeyHex)
	require.NoError(b, err)
	publicKey := privateKey.PubKey()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Sign
		signature, err := privateKey.Sign(testHash)
		if err != nil {
			b.Fatal(err)
		}

		// Verify
		valid := publicKey.Verify(testHash, signature)
		if !valid {
			b.Fatal("signature verification failed")
		}
	}
}

// BenchmarkCryptoBackend_Memory measures memory allocations
func BenchmarkCryptoBackend_Memory(b *testing.B) {
	signer, testHash, _ := setupCryptoBenchmarkData(b)
	cryptoSigner := signer.GetCryptoSigner()
	privateKey, err := cryptoSigner.DecodePrivateKey(signer.PrivateKeyHex)
	require.NoError(b, err)
	publicKey := privateKey.PubKey()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		signature, err := privateKey.Sign(testHash)
		if err != nil {
			b.Fatal(err)
		}

		valid := publicKey.Verify(testHash, signature)
		if !valid {
			b.Fatal("signature verification failed")
		}
	}
}

// TestCryptoBackend_Compatibility ensures the selected backend produces valid results
func TestCryptoBackend_Compatibility(t *testing.T) {
	signer, testHash, _ := setupCryptoBenchmarkData(t)
	info := signer.GetBackendInfo()

	t.Logf("Testing %s backend", info.Name)
	t.Logf("CGO Required: %v", info.CGORequired)
	t.Logf("Performance Level: %s", info.PerformanceLevel)

	cryptoSigner := signer.GetCryptoSigner()

	// Test private key decoding
	privateKey, err := cryptoSigner.DecodePrivateKey(signer.PrivateKeyHex)
	require.NoError(t, err)
	require.NotNil(t, privateKey)

	// Test signing
	signature, err := privateKey.Sign(testHash)
	require.NoError(t, err)
	require.NotEmpty(t, signature)
	t.Logf("Signature length: %d bytes", len(signature))

	// Test public key generation
	publicKey := privateKey.PubKey()
	require.NotNil(t, publicKey)
	t.Logf("Public key length: %d bytes", len(publicKey.Bytes()))

	// Test verification
	valid := publicKey.Verify(testHash, signature)
	require.True(t, valid, "Signature should be valid")

	// Test invalid signature
	invalidSignature := make([]byte, len(signature))
	copy(invalidSignature, signature)
	invalidSignature[0] ^= 0xFF // Flip bits

	valid = publicKey.Verify(testHash, invalidSignature)
	require.False(t, valid, "Invalid signature should not verify")

	t.Logf("✅ %s backend compatibility test passed", info.Name)
}

// TestCryptoBackend_ErrorHandling tests error conditions
func TestCryptoBackend_ErrorHandling(t *testing.T) {
	signer, _, _ := setupCryptoBenchmarkData(t)
	cryptoSigner := signer.GetCryptoSigner()

	// Test invalid hex key
	_, err := cryptoSigner.DecodePrivateKey("invalid-hex")
	require.Error(t, err, "Should reject invalid hex")

	// Test wrong length key
	_, err = cryptoSigner.DecodePrivateKey("1234") // Too short
	require.Error(t, err, "Should reject wrong length key")

	// Test zero key
	zeroKey := "0000000000000000000000000000000000000000000000000000000000000000"
	_, zeroErr := cryptoSigner.DecodePrivateKey(zeroKey)
	// Some backends may accept zero key, others may reject it - both are valid
	_ = zeroErr // Explicitly ignore the error as both accept/reject are valid

	t.Logf("✅ Error handling test passed")
}

// TestCryptoBackend_KeyFormats tests different key formats
func TestCryptoBackend_KeyFormats(t *testing.T) {
	testKey := "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef"

	signer, err := NewSignerFromHex(testKey)
	require.NoError(t, err)

	cryptoSigner := signer.GetCryptoSigner()
	privateKey, err := cryptoSigner.DecodePrivateKey(testKey)
	require.NoError(t, err)

	// Test hex roundtrip
	keyHex := privateKey.Hex()
	require.Equal(t, testKey, keyHex, "Hex encoding should roundtrip")

	// Test bytes roundtrip
	keyBytes := privateKey.Bytes()
	require.Equal(t, 32, len(keyBytes), "Private key should be 32 bytes")

	decodedKey, err := cryptoSigner.DecodePrivateKey(keyHex)
	require.NoError(t, err)
	require.Equal(t, keyBytes, decodedKey.Bytes(), "Bytes should roundtrip")

	t.Logf("✅ Key format test passed")
}
