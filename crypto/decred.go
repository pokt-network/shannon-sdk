//go:build !ethereum_secp256k1
// +build !ethereum_secp256k1

package crypto

import (
	"context"
	"encoding/hex"
	"fmt"

	decred "github.com/decred/dcrd/dcrec/secp256k1/v4"
	decred_ecdsa "github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	"github.com/pokt-network/ring-go"
)

// TODO_OPTIMIZE: Consider caching computed public keys to avoid regenerating them on each call
// decredSigner implements CryptoSigner using Decred's pure Go secp256k1 implementation.
// This provides excellent performance without requiring CGO, making it highly portable.
type decredSigner struct {
	privateKey *DecredPrivateKey
}

// DecredPrivateKey implements PrivateKey using Decred's secp256k1 library.
type DecredPrivateKey struct {
	key *decred.PrivateKey
}

// DecredPublicKey implements PublicKey using Decred's secp256k1 library.
type DecredPublicKey struct {
	key *decred.PublicKey
}

// newCryptoSigner creates a new Decred-based crypto signer.
// This function is called by NewSigner when the ethereum_secp256k1 build tag is NOT active.
func newCryptoSigner(privateKeyHex string) (CryptoSigner, error) {
	signer := &decredSigner{}

	privKey, err := signer.DecodePrivateKey(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	signer.privateKey = privKey.(*DecredPrivateKey)
	return signer, nil
}

// Sign implements CryptoSigner.Sign using Decred's secp256k1 implementation.
// This follows the same logic as the original signer but uses the Decred backend.
func (s *decredSigner) Sign(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	appRing ApplicationRing,
) (*servicetypes.RelayRequest, error) {
	// Get the session ring for the application's session end block height
	sessionRingInterface, err := appRing.GetRing(ctx, uint64(relayRequest.Meta.SessionHeader.SessionEndBlockHeight))
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error getting a ring for application address %s: %w",
			appRing.GetAddress(),
			err,
		)
	}

	// Type assert to *ring.Ring
	sessionRing, ok := sessionRingInterface.(*ring.Ring)
	if !ok {
		return nil, fmt.Errorf("Sign: unexpected ring type: %T", sessionRingInterface)
	}

	// Get the signable bytes hash from the relay request
	signableBz, err := relayRequest.GetSignableBytesHash()
	if err != nil {
		return nil, fmt.Errorf("Sign: error getting signable bytes hash from the relay request: %w", err)
	}

	// Convert hex private key to ring-go scalar format
	signerPrivKeyBz, err := hex.DecodeString(s.privateKey.Hex())
	if err != nil {
		return nil, fmt.Errorf("Sign: error decoding private key to bytes: %w", err)
	}

	signerPrivKey, err := ring.Secp256k1().DecodeToScalar(signerPrivKeyBz)
	if err != nil {
		return nil, fmt.Errorf("Sign: error decoding private key to scalar: %w", err)
	}

	// TODO_INVESTIGATE: Profile memory allocations here - Decred shows 32 allocs vs Ethereum's 3
	// Sign the request using the session ring and signer's private key
	ringSig, err := sessionRing.Sign(signableBz, signerPrivKey)
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error signing using the ring of application with address %s: %w",
			appRing.GetAddress(),
			err,
		)
	}

	// Serialize the signature
	signature, err := ringSig.Serialize()
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error serializing the signature of application with address %s: %w",
			appRing.GetAddress(),
			err,
		)
	}

	// Set the signature on the relay request
	relayRequest.Meta.Signature = signature
	return relayRequest, nil
}

// DecodePrivateKey implements CryptoSigner.DecodePrivateKey.
func (s *decredSigner) DecodePrivateKey(hexKey string) (PrivateKey, error) {
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid hex key: %w", err)
	}

	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(keyBytes))
	}

	privKey := decred.PrivKeyFromBytes(keyBytes)
	return &DecredPrivateKey{key: privKey}, nil
}

// GetBackendInfo implements CryptoSigner.GetBackendInfo.
func (s *decredSigner) GetBackendInfo() BackendInfo {
	return BackendInfo{
		Name:                "decred",
		CGORequired:         false,
		SigningSpeedUs:      37.6,  // From benchmark results
		VerificationSpeedUs: 129.8, // From benchmark results
		PerformanceLevel:    "excellent (CGO-free)",
		Notes:               "Pure Go implementation with optimal CGO-free performance",
	}
}

// getAvailableBackends returns info about available backends when Decred is the default.
func getAvailableBackends() []BackendInfo {
	backends := []BackendInfo{
		{
			Name:                "decred",
			CGORequired:         false,
			SigningSpeedUs:      37.6,
			VerificationSpeedUs: 129.8,
			PerformanceLevel:    "excellent (CGO-free)",
			Notes:               "Pure Go implementation with optimal CGO-free performance",
		},
	}

	// Note about Ethereum backend being available with different build tags
	backends = append(backends, BackendInfo{
		Name:                "ethereum",
		CGORequired:         true,
		SigningSpeedUs:      20.5,
		VerificationSpeedUs: 23.8,
		PerformanceLevel:    "fastest (not compiled)",
		Notes:               "Available with 'ethereum_secp256k1' build tag - requires CGO",
	})

	return backends
}

// Sign implements PrivateKey.Sign using Decred's secp256k1.
func (k *DecredPrivateKey) Sign(hash []byte) ([]byte, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash must be exactly 32 bytes, got %d", len(hash))
	}

	signature := decred_ecdsa.Sign(k.key, hash)
	return signature.Serialize(), nil
}

// Bytes implements PrivateKey.Bytes.
func (k *DecredPrivateKey) Bytes() []byte {
	return k.key.Serialize()
}

// PubKey implements PrivateKey.PubKey.
func (k *DecredPrivateKey) PubKey() PublicKey {
	return &DecredPublicKey{key: k.key.PubKey()}
}

// Hex implements PrivateKey.Hex.
func (k *DecredPrivateKey) Hex() string {
	return hex.EncodeToString(k.Bytes())
}

// Verify implements PublicKey.Verify using Decred's secp256k1.
func (k *DecredPublicKey) Verify(hash []byte, signatureBytes []byte) bool {
	if len(hash) != 32 {
		return false
	}

	// Parse DER signature format - this is what Decred's Serialize() returns
	signature, err := decred_ecdsa.ParseDERSignature(signatureBytes)
	if err != nil {
		return false
	}

	return signature.Verify(hash, k.key)
}

// Bytes implements PublicKey.Bytes.
func (k *DecredPublicKey) Bytes() []byte {
	return k.key.SerializeCompressed()
}

// Hex implements PublicKey.Hex.
func (k *DecredPublicKey) Hex() string {
	return hex.EncodeToString(k.Bytes())
}
