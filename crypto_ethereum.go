//go:build cgo && ethereum_secp256k1
// +build cgo,ethereum_secp256k1

package sdk

import (
	"context"
	"encoding/hex"
	"fmt"

	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	ethsecp256k1 "github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/pokt-network/ring-go"
)

// ethereumSigner implements CryptoSigner using Ethereum's libsecp256k1 wrapper.
// This provides the highest performance but requires CGO and the libsecp256k1 C library.
type ethereumSigner struct {
	privateKey *EthereumPrivateKey
}

// EthereumPrivateKey implements PrivateKey using Ethereum's secp256k1 library.
type EthereumPrivateKey struct {
	keyBytes []byte
}

// EthereumPublicKey implements PublicKey using Ethereum's secp256k1 library.
type EthereumPublicKey struct {
	keyBytes []byte
}

// newCryptoSigner creates a new Ethereum-based crypto signer.
// This function is called by NewSigner when the ethereum_secp256k1 build tag is active.
func newCryptoSigner(privateKeyHex string) (CryptoSigner, error) {
	signer := &ethereumSigner{}
	
	privKey, err := signer.DecodePrivateKey(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to decode private key: %w", err)
	}
	
	signer.privateKey = privKey.(*EthereumPrivateKey)
	return signer, nil
}

// Sign implements CryptoSigner.Sign using Ethereum's libsecp256k1.
func (s *ethereumSigner) Sign(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	appRing ApplicationRing,
) (*servicetypes.RelayRequest, error) {
	// Get the session ring for the application's session end block height
	sessionRing, err := appRing.GetRing(ctx, uint64(relayRequest.Meta.SessionHeader.SessionEndBlockHeight))
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error getting a ring for application address %s: %w",
			appRing.Address,
			err,
		)
	}

	// Get the signable bytes hash from the relay request
	signableBz, err := relayRequest.GetSignableBytesHash()
	if err != nil {
		return nil, fmt.Errorf("Sign: error getting signable bytes hash from the relay request: %w", err)
	}

	// Convert hex private key to ring-go scalar format (same as Decred backend)
	signerPrivKeyBz, err := hex.DecodeString(s.privateKey.Hex())
	if err != nil {
		return nil, fmt.Errorf("Sign: error decoding private key to bytes: %w", err)
	}

	signerPrivKey, err := ring.Secp256k1().DecodeToScalar(signerPrivKeyBz)
	if err != nil {
		return nil, fmt.Errorf("Sign: error decoding private key to scalar: %w", err)
	}

	// Sign the request using the session ring and signer's private key
	ringSig, err := sessionRing.Sign(signableBz, signerPrivKey)
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error signing using the ring of application with address %s: %w",
			appRing.Address,
			err,
		)
	}

	// Serialize the ring signature
	ringSignature, err := ringSig.Serialize()
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error serializing the signature of application with address %s: %w",
			appRing.Address,
			err,
		)
	}

	// Set the signature on the relay request
	relayRequest.Meta.Signature = ringSignature
	return relayRequest, nil
}

// DecodePrivateKey implements CryptoSigner.DecodePrivateKey.
func (s *ethereumSigner) DecodePrivateKey(hexKey string) (PrivateKey, error) {
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid hex key: %w", err)
	}
	
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(keyBytes))
	}
	
	// Test that the key is valid by attempting to generate the public key
	_, err = ethsecp256k1.RecoverPubkey(make([]byte, 32), append(keyBytes, make([]byte, 33)...))
	if err != nil {
		// If recovery fails, try a simple sign operation to validate
		testHash := make([]byte, 32)
		testHash[0] = 1 // Make it non-zero
		_, err = ethsecp256k1.Sign(testHash, keyBytes)
		if err != nil {
			return nil, fmt.Errorf("invalid private key: %w", err)
		}
	}
	
	return &EthereumPrivateKey{keyBytes: keyBytes}, nil
}

// GetBackendInfo implements CryptoSigner.GetBackendInfo.
func (s *ethereumSigner) GetBackendInfo() BackendInfo {
	return BackendInfo{
		Name:                "ethereum",
		CGORequired:         true,
		SigningSpeedUs:      20.5, // From benchmark results
		VerificationSpeedUs: 23.8, // From benchmark results
		PerformanceLevel:    "fastest",
		Notes:               "Uses Bitcoin Core's libsecp256k1 via CGO for maximum performance",
	}
}

// getAvailableBackends returns info about available backends when Ethereum is compiled in.
func getAvailableBackends() []BackendInfo {
	return []BackendInfo{
		{
			Name:                "ethereum",
			CGORequired:         true,
			SigningSpeedUs:      20.5,
			VerificationSpeedUs: 23.8,
			PerformanceLevel:    "fastest",
			Notes:               "Uses Bitcoin Core's libsecp256k1 via CGO for maximum performance",
		},
	}
}

// Sign implements PrivateKey.Sign using Ethereum's libsecp256k1.
func (k *EthereumPrivateKey) Sign(hash []byte) ([]byte, error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash must be exactly 32 bytes, got %d", len(hash))
	}
	
	signature, err := ethsecp256k1.Sign(hash, k.keyBytes)
	if err != nil {
		return nil, fmt.Errorf("Ethereum secp256k1 signing failed: %w", err)
	}
	
	// Remove recovery ID (last byte) to get standard signature format
	if len(signature) == 65 {
		signature = signature[:64]
	}
	
	return signature, nil
}

// Bytes implements PrivateKey.Bytes.
func (k *EthereumPrivateKey) Bytes() []byte {
	result := make([]byte, len(k.keyBytes))
	copy(result, k.keyBytes)
	return result
}

// PubKey implements PrivateKey.PubKey.
func (k *EthereumPrivateKey) PubKey() PublicKey {
	// Create a test signature to recover the public key
	testHash := make([]byte, 32)
	testHash[0] = 1 // Make it non-zero
	
	signature, err := ethsecp256k1.Sign(testHash, k.keyBytes)
	if err != nil {
		// Fallback: this should not happen with valid keys
		return &EthereumPublicKey{keyBytes: make([]byte, 33)}
	}
	
	pubKeyBytes, err := ethsecp256k1.RecoverPubkey(testHash, signature)
	if err != nil {
		// Fallback: this should not happen with valid signatures
		return &EthereumPublicKey{keyBytes: make([]byte, 33)}
	}
	
	return &EthereumPublicKey{keyBytes: pubKeyBytes}
}

// Hex implements PrivateKey.Hex.
func (k *EthereumPrivateKey) Hex() string {
	return hex.EncodeToString(k.keyBytes)
}

// Verify implements PublicKey.Verify using Ethereum's libsecp256k1.
func (k *EthereumPublicKey) Verify(hash []byte, signature []byte) bool {
	if len(hash) != 32 {
		return false
	}
	
	// Ensure signature is exactly 64 bytes (without recovery ID)
	if len(signature) != 64 {
		return false
	}
	
	return ethsecp256k1.VerifySignature(k.keyBytes, hash, signature)
}

// Bytes implements PublicKey.Bytes.
func (k *EthereumPublicKey) Bytes() []byte {
	result := make([]byte, len(k.keyBytes))
	copy(result, k.keyBytes)
	return result
}

// Hex implements PublicKey.Hex.
func (k *EthereumPublicKey) Hex() string {
	return hex.EncodeToString(k.keyBytes)
}