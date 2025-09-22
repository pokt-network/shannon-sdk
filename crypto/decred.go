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
)

// TODO_OPTIMIZE: Consider caching computed public keys to avoid regenerating them on each call
// decredSigner implements CryptoSigner using Decred's pure Go secp256k1 implementation.
// This provides excellent performance without requiring CGO, making it highly portable.

var _ CryptoSigner = (*decredSigner)(nil)

// decredSigner implements CryptoSigner using Decred's pure Go secp256k1 implementation.
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

// NewCryptoSigner creates a new Decred-based crypto signer.
// This function is called by NewSigner when the ethereum_secp256k1 build tag is NOT active.
func NewCryptoSigner(privateKeyHex string) (CryptoSigner, error) {
	signer := &decredSigner{}

	privKey, err := signer.DecodePrivateKey(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}

	signer.privateKey = privKey.(*DecredPrivateKey)
	return signer, nil
}

// Sign implements CryptoSigner.Sign using Decred's secp256k1 implementation.
// TODO_INVESTIGATE: Profile memory allocations - Decred shows 32 allocs vs Ethereum's 3
func (s *decredSigner) Sign(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	appRing ApplicationRing,
) (*servicetypes.RelayRequest, error) {
	return commonSign(ctx, relayRequest, appRing, s.privateKey)
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
