package sdk

import (
	"context"
	"fmt"

	servicetypes "github.com/pokt-network/poktroll/x/service/types"
)

// Signer holds the application or gateway's private key used to sign Relay Requests.
// 
// This version has been updated to use pluggable crypto backends for optimal performance
// vs portability trade-offs. The backend is selected at build time:
// - With "ethereum_secp256k1" tag: Uses Ethereum's libsecp256k1 (fastest, requires CGO)
// - Without tag: Uses Decred's implementation (excellent performance, pure Go)
type Signer struct {
	// PrivateKeyHex is the hex-encoded private key string (maintained for compatibility)
	PrivateKeyHex string
	
	// cryptoSigner is the pluggable crypto backend
	cryptoSigner CryptoSigner
}

// NewSignerFromHex creates a new Signer instance from a hex-encoded private key.
// The crypto backend is automatically selected based on build tags.
//
// Example usage:
//   signer, err := sdk.NewSignerFromHex("1234567890abcdef...")
//   if err != nil {
//       log.Fatal(err)
//   }
//   
//   // Log which backend is being used
//   sdk.LogBackendInfo(signer.cryptoSigner)
func NewSignerFromHex(privateKeyHex string) (*Signer, error) {
	cryptoSigner, err := NewSigner(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("failed to create crypto signer: %w", err)
	}
	
	return &Signer{
		PrivateKeyHex: privateKeyHex,
		cryptoSigner:  cryptoSigner,
	}, nil
}

// GetBackendInfo returns information about the crypto backend being used.
func (s *Signer) GetBackendInfo() BackendInfo {
	if s.cryptoSigner == nil {
		// Fallback info for uninitialized signers
		return BackendInfo{
			Name:                "unknown",
			CGORequired:         false,
			SigningSpeedUs:      0,
			VerificationSpeedUs: 0,
			PerformanceLevel:    "uninitialized",
			Notes:               "Signer not properly initialized",
		}
	}
	return s.cryptoSigner.GetBackendInfo()
}

// Sign signs the given relay request using the signer's private key and the application's ring.
//
// This method now delegates to the pluggable crypto backend for optimal performance.
// The backend choice provides different performance characteristics:
//
// - Ethereum backend: ~20.5μs signing, ~23.8μs verification (requires CGO)
// - Decred backend:   ~37.6μs signing, ~129.8μs verification (pure Go)
//
// Returns a pointer instead of directly setting the signature on the input relay request to avoid implicit output.
func (s *Signer) Sign(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	appRing ApplicationRing,
) (*servicetypes.RelayRequest, error) {
	// Initialize crypto signer if not already done (lazy initialization)
	if s.cryptoSigner == nil {
		cryptoSigner, err := NewSigner(s.PrivateKeyHex)
		if err != nil {
			return nil, fmt.Errorf("failed to initialize crypto signer: %w", err)
		}
		s.cryptoSigner = cryptoSigner
	}
	
	// Delegate to the pluggable crypto backend
	return s.cryptoSigner.Sign(ctx, relayRequest, appRing)
}

// GetCryptoSigner returns the underlying crypto signer for advanced use cases.
// This allows access to backend-specific functionality if needed.
func (s *Signer) GetCryptoSigner() CryptoSigner {
	return s.cryptoSigner
}

// SetCryptoSigner allows setting a custom crypto signer implementation.
// This is primarily useful for testing or advanced customization scenarios.
func (s *Signer) SetCryptoSigner(cryptoSigner CryptoSigner) {
	s.cryptoSigner = cryptoSigner
}