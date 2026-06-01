package sdk

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"

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

	// signerContextCache caches SignerContext per ring to avoid redundant
	// cryptographic operations (public key computation, hash-to-curve, key image).
	// Key is the ring's memory address as a unique identifier.
	signerContextCache sync.Map // map[*ring.Ring]*ring.SignerContext
}

// NewSignerFromHex creates a new Signer instance from a hex-encoded private key.
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

// getOrCreateSignerContext returns a cached SignerContext for the given ring,
// or creates and caches a new one if not present.
func (s *Signer) getOrCreateSignerContext(sessionRing *ring.Ring) (*ring.SignerContext, error) {
	// Check cache first
	if cached, ok := s.signerContextCache.Load(sessionRing); ok {
		return cached.(*ring.SignerContext), nil
	}

	// Decode private key to scalar
	privKey, err := ring.Secp256k1().DecodeToScalar(s.privateKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("getOrCreateSignerContext: error decoding private key: %w", err)
	}

	// Create new SignerContext with pre-computed values
	ctx, err := sessionRing.NewSignerContext(privKey)
	if err != nil {
		return nil, fmt.Errorf("getOrCreateSignerContext: error creating signer context: %w", err)
	}

	// Cache it (use LoadOrStore to handle concurrent creation)
	actual, _ := s.signerContextCache.LoadOrStore(sessionRing, ctx)
	return actual.(*ring.SignerContext), nil
}

// Sign signs the given relay request using the signer's private key and the application's ring.
//
// This method uses SignerContext caching for optimal performance when signing
// multiple requests with the same ring (which is common within a session).
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

	// Get or create cached SignerContext for this ring
	signerCtx, err := s.getOrCreateSignerContext(sessionRing)
	if err != nil {
		return nil, fmt.Errorf("Sign: error getting signer context: %w", err)
	}

	// Sign using the cached context (avoids redundant ScalarBaseMul, hashToCurve, etc.)
	ringSig, err := sessionRing.SignWithContext(signableBz, signerCtx)
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

// SignWithRing signs the given relay request using the signer's private key and a pre-built ring.
//
// This method is useful when the caller caches the ring externally (e.g., by session)
// to ensure the same ring pointer is reused, enabling SignerContext cache hits.
//
// Returns a pointer instead of directly setting the signature on the input relay request to avoid implicit output.
func (s *Signer) SignWithRing(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	sessionRing *ring.Ring,
) (*servicetypes.RelayRequest, error) {
	// Get the signable bytes hash from the relay request
	signableBz, err := relayRequest.GetSignableBytesHash()
	if err != nil {
		return nil, fmt.Errorf("SignWithRing: error getting signable bytes hash from the relay request: %w", err)
	}

	// Get or create cached SignerContext for this ring
	signerCtx, err := s.getOrCreateSignerContext(sessionRing)
	if err != nil {
		return nil, fmt.Errorf("SignWithRing: error getting signer context: %w", err)
	}

	// Sign using the cached context (avoids redundant ScalarBaseMul, hashToCurve, etc.)
	ringSig, err := sessionRing.SignWithContext(signableBz, signerCtx)
	if err != nil {
		return nil, fmt.Errorf("SignWithRing: error signing relay request: %w", err)
	}

	// Serialize the signature
	signature, err := ringSig.Serialize()
	if err != nil {
		return nil, fmt.Errorf("SignWithRing: error serializing the signature: %w", err)
	}

	// Set the signature on the relay request
	relayRequest.Meta.Signature = signature
	return relayRequest, nil
}

// ClearSignerContextCache clears the cached SignerContexts.
// This should be called when sessions roll over and old rings are no longer needed.
func (s *Signer) ClearSignerContextCache() {
	s.signerContextCache = sync.Map{}
}