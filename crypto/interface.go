package crypto

import (
	"context"
	"fmt"

	servicetypes "github.com/pokt-network/poktroll/x/service/types"
)

// ApplicationRing is an interface that crypto package needs from the main SDK.
// This avoids circular dependencies.
type ApplicationRing interface {
	GetRing(ctx context.Context, sessionEndHeight uint64) (interface{}, error)
}

// CryptoSigner defines the interface for signing operations in the Shannon SDK.
// BUILD-TIME CONFIGURATION: Different implementations are selected at compile time
// based on build tags for optimal performance vs portability trade-offs.
//
// Available backends:
// - Ethereum (build tag: ethereum_secp256k1): Uses libsecp256k1 C library, fastest performance, requires CGO
// - Decred (default, no build tag): Pure Go implementation, excellent performance, maximum portability
type CryptoSigner interface {
	// Sign signs the given relay request using the signer's private key and the application's ring.
	// Returns a pointer to avoid implicit output modification.
	Sign(ctx context.Context, relayRequest *servicetypes.RelayRequest, appRing ApplicationRing) (*servicetypes.RelayRequest, error)

	// DecodePrivateKey converts a hex-encoded private key string to a PrivateKey instance.
	// This method handles the backend-specific private key format and validation.
	DecodePrivateKey(hexKey string) (PrivateKey, error)

	// GetBackendInfo returns information about the crypto backend being used.
	GetBackendInfo() BackendInfo
}

// PrivateKey represents a secp256k1 private key with signing capabilities.
// The underlying implementation varies based on the selected crypto backend.
type PrivateKey interface {
	// Sign creates a signature over the given hash using this private key.
	Sign(hash []byte) ([]byte, error)

	// Bytes returns the raw bytes of the private key.
	Bytes() []byte

	// PubKey returns the corresponding public key.
	PubKey() PublicKey

	// Hex returns the hex-encoded string representation of the private key.
	Hex() string
}

// PublicKey represents a secp256k1 public key with verification capabilities.
type PublicKey interface {
	// Verify checks if the given signature is valid for the hash using this public key.
	Verify(hash []byte, signature []byte) bool

	// Bytes returns the raw bytes of the public key.
	Bytes() []byte

	// Hex returns the hex-encoded string representation of the public key.
	Hex() string
}

// BackendInfo provides information about the crypto backend implementation.
type BackendInfo struct {
	// Name of the backend (e.g., "ethereum", "decred")
	Name string

	// CGORequired indicates if this backend requires CGO to be enabled
	CGORequired bool

	// Performance metrics from benchmarks (in microseconds)
	SigningSpeedUs      float64
	VerificationSpeedUs float64

	// Human-readable performance description
	PerformanceLevel string

	// Additional notes about the backend
	Notes string
}

// String returns a formatted string representation of the backend info.
func (bi BackendInfo) String() string {
	cgoStatus := "pure Go"
	if bi.CGORequired {
		cgoStatus = "requires CGO"
	}

	return fmt.Sprintf("%s backend (%s) - %s - Signing: %.1fμs, Verification: %.1fμs",
		bi.Name, cgoStatus, bi.PerformanceLevel, bi.SigningSpeedUs, bi.VerificationSpeedUs)
}

// NewSigner creates a new CryptoSigner instance using the specified private key.
// BUILD-TIME CONFIGURATION: The actual implementation is determined at compile time
// based on build tags:
//
// - With "ethereum_secp256k1" tag: Uses Ethereum's libsecp256k1 (fastest, requires CGO)
// - Without tag: Uses Decred's implementation (portable, pure Go)
//
// Example usage:
//
//	signer := crypto.NewSigner("1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
//	info := signer.GetBackendInfo()
//	fmt.Printf("Using: %s\n", info)
func NewSigner(privateKeyHex string) (CryptoSigner, error) {
	return newCryptoSigner(privateKeyHex)
}

// GetAvailableBackends returns information about all backends that could be compiled
// based on the current environment and build tags.
// BUILD-TIME CONFIGURATION: This reflects what was available at compile time.
func GetAvailableBackends() []BackendInfo {
	return getAvailableBackends()
}

// LogBackendInfo logs information about the currently active crypto backend.
// This is useful for debugging and verifying which backend was selected at build time.
func LogBackendInfo(signer CryptoSigner) {
	info := signer.GetBackendInfo()
	fmt.Printf("🔐 Shannon SDK Crypto Backend: %s\n", info)
}