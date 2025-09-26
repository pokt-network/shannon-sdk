package sdk

import (
	"context"
	"encoding/hex"
	"fmt"

	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	"github.com/pokt-network/ring-go"
)

// Signer holds the application or gateway's private key used to sign Relay Requests.
//
// Pluggable crypto backends for optimal performance vs portability trade-offs.
// The backend is selected at build time:
// - With "ethereum_secp256k1" tag: Uses Ethereum's libsecp256k1 (fastest, requires CGO)
// - Without tag: Uses Decred's implementation (excellent performance, pure Go)
type Signer struct {
	// PrivateKeyHex is retained for compatibility and debugging.
	PrivateKeyHex string
	// privateKeyBytes caches the 32-byte private key to avoid repeated hex decoding.
	privateKeyBytes []byte
}

// NewSignerFromHex creates a new Signer instance from a hex-encoded private key.
//
// Example usage:
//
//	signer, err := sdk.NewSignerFromHex("1234567890abcdef...")
//	if err != nil {
//	    log.Fatal(err)
//	}
func NewSignerFromHex(privateKeyHex string) (*Signer, error) {
	keyBytes, err := hex.DecodeString(privateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid hex private key: %w", err)
	}
	if len(keyBytes) != 32 {
		return nil, fmt.Errorf("invalid private key length: expected 32 bytes, got %d", len(keyBytes))
	}
	return &Signer{
		PrivateKeyHex:   privateKeyHex,
		privateKeyBytes: keyBytes,
	}, nil
}

// Sign signs the given relay request using the signer's private key and the application's ring.
//
// This method delegates to the pluggable crypto backend for optimal performance.
// The backend choice provides different performance characteristics:
//
// Returns a pointer instead of directly setting the signature on the input relay request to avoid implicit output.
func (s *Signer) Sign(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	appRing *ApplicationRing,
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

	// Sign the request using the session ring and signer's private key
	// TODO_OPTIMIZE: Pre-cache the decoded scalar to avoid per-call DecodeToScalar
	// once ring-go exposes a stable public scalar type for long-lived reuse.
	// Decode private key bytes to scalar (fast, but still cheaper than hex decoding each call)
	signerPrivKey, err := ring.Secp256k1().DecodeToScalar(s.privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("Sign: error decoding private key to scalar: %w", err)
	}

	ringSig, err := sessionRing.Sign(signableBz, signerPrivKey)
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error signing using the ring of application with address %s: %w",
			appRing.Address,
			err,
		)
	}

	// Serialize the signature
	signature, err := ringSig.Serialize()
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error serializing the signature of application with address %s: %w",
			appRing.Address,
			err,
		)
	}

	// Set the signature on the relay request
	relayRequest.Meta.Signature = signature
	return relayRequest, nil
}
