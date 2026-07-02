package sdk

import (
	"context"
	"encoding/hex"
	"fmt"
	"sync"

	dleqtypes "github.com/pokt-network/go-dleq/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	"github.com/pokt-network/ring-go"
)

// Signer holds the application or gateway's private key used to sign Relay Requests.
//
// Pluggable crypto backends for optimal performance vs portability trade-offs.
// The backend is selected at build time:
// - With "ethereum_secp256k1" tag: Uses Ethereum's libsecp256k1 (fastest, requires CGO)
// - Without tag: Uses Decred's implementation (excellent performance, pure Go)
//
// Two signing families are exposed, mirroring ring-go:
//   - Sign: the deterministic, consensus-safe default (ring-go's Ring.Sign).
//   - SignOffChain / SignOffChainWithRing: the faster off-chain path (ring-go's
//     SignWithContext) that caches per-ring precompute. OFF-CHAIN ONLY — its
//     per-signature work is not deterministic across nodes.
type Signer struct {
	// PrivateKeyHex is retained for compatibility and debugging.
	PrivateKeyHex string
	// privateKeyBytes caches the 32-byte private key to avoid repeated hex decoding.
	privateKeyBytes []byte
	// signerScalar is the decoded private-key scalar, cached once at construction
	// to avoid a per-call DecodeToScalar. Supplied per call to both Ring.Sign and
	// Ring.SignWithContext. Held for the signer's lifetime alongside
	// privateKeyBytes; both are secret material.
	signerScalar dleqtypes.Scalar

	// signerContextCache caches a secret-free SignerContext per ring for the
	// off-chain path only, avoiding redundant cryptographic setup (public key,
	// hash-to-curve, key image, and the hash-to-curve of every ring member).
	// Key is the ring pointer. The SignerContext holds no secret material, so it
	// is safe to cache. The deterministic Sign path never touches this cache.
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
	// Decode the private key to a scalar once; reused for every sign call.
	signerScalar, err := ring.Secp256k1().DecodeToScalar(keyBytes)
	if err != nil {
		return nil, fmt.Errorf("error decoding private key to scalar: %w", err)
	}
	return &Signer{
		PrivateKeyHex:   privateKeyHex,
		privateKeyBytes: keyBytes,
		signerScalar:    signerScalar,
	}, nil
}

// Sign signs the given relay request using the signer's private key and the application's ring.
//
// This is the deterministic, consensus-safe default (ring-go's Ring.Sign): its
// per-signature work is identical across nodes. Use it anywhere correctness or
// gas-metering depends on deterministic work. For high-throughput off-chain
// relay signing, prefer SignOffChain — both produce signatures that verify
// identically.
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

	// Deterministic path: plain ring.Sign, no SignerContext and no SkipSelfCheck.
	ringSig, err := sessionRing.Sign(signableBz, s.signerScalar)
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

// getOrCreateSignerContext returns a cached SignerContext for the given ring,
// or creates and caches a new one if not present. Used only by the off-chain
// signing path.
func (s *Signer) getOrCreateSignerContext(sessionRing *ring.Ring) (*ring.SignerContext, error) {
	// Check cache first
	if cached, ok := s.signerContextCache.Load(sessionRing); ok {
		return cached.(*ring.SignerContext), nil
	}

	// Create a new SignerContext with pre-computed values for this ring.
	signerCtx, err := sessionRing.NewSignerContext(s.signerScalar)
	if err != nil {
		return nil, fmt.Errorf("getOrCreateSignerContext: error creating signer context: %w", err)
	}

	// Off-chain path: skip the closing self-checks for a bit more speed
	// (~4 scalar-muls/sign). Safe here — the signature stays fully verifiable and
	// this path is never consensus-critical/gas-metered. The mismatched-key
	// caveat does not apply: the context is built from, and always signed with,
	// the same s.signerScalar.
	signerCtx.SkipSelfCheck = true

	// Cache it (use LoadOrStore to handle concurrent creation)
	actual, _ := s.signerContextCache.LoadOrStore(sessionRing, signerCtx)
	return actual.(*ring.SignerContext), nil
}

// SignOffChain signs the given relay request using the faster off-chain path
// (ring-go's SignWithContext) with a per-ring cached SignerContext.
//
// It is the throughput-optimized counterpart to Sign for the off-chain
// relay/session signing path. OFF-CHAIN ONLY: the precompute means its
// per-signature work differs from Sign, so it must NOT be used on
// consensus-critical / gas-metered paths — use Sign there. The produced
// signature verifies identically to one from Sign.
//
// Returns a pointer instead of directly setting the signature on the input relay request to avoid implicit output.
func (s *Signer) SignOffChain(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	appRing *ApplicationRing,
) (*servicetypes.RelayRequest, error) {
	// Get the session ring for the application's session end block height
	sessionRing, err := appRing.GetRing(ctx, uint64(relayRequest.Meta.SessionHeader.SessionEndBlockHeight))
	if err != nil {
		return nil, fmt.Errorf(
			"SignOffChain: error getting a ring for application address %s: %w",
			appRing.Address,
			err,
		)
	}

	return s.SignOffChainWithRing(ctx, relayRequest, sessionRing)
}

// SignOffChainWithRing signs the given relay request using the faster off-chain
// path and a pre-built ring.
//
// This is useful when the caller caches the ring externally (e.g., by session)
// to ensure the same ring pointer is reused, enabling SignerContext cache hits.
// Same OFF-CHAIN ONLY constraint as SignOffChain.
//
// Returns a pointer instead of directly setting the signature on the input relay request to avoid implicit output.
func (s *Signer) SignOffChainWithRing(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	sessionRing *ring.Ring,
) (*servicetypes.RelayRequest, error) {
	// Get the signable bytes hash from the relay request
	signableBz, err := relayRequest.GetSignableBytesHash()
	if err != nil {
		return nil, fmt.Errorf("SignOffChainWithRing: error getting signable bytes hash from the relay request: %w", err)
	}

	// Get or create cached SignerContext for this ring
	signerCtx, err := s.getOrCreateSignerContext(sessionRing)
	if err != nil {
		return nil, fmt.Errorf("SignOffChainWithRing: error getting signer context: %w", err)
	}

	// Sign using the cached context (avoids redundant ScalarBaseMul, hashToCurve, etc.).
	// The private key scalar is supplied per call in the hash-cache API.
	ringSig, err := sessionRing.SignWithContext(signableBz, s.signerScalar, signerCtx)
	if err != nil {
		return nil, fmt.Errorf("SignOffChainWithRing: error signing relay request: %w", err)
	}

	// Serialize the signature
	signature, err := ringSig.Serialize()
	if err != nil {
		return nil, fmt.Errorf("SignOffChainWithRing: error serializing the signature: %w", err)
	}

	// Set the signature on the relay request
	relayRequest.Meta.Signature = signature
	return relayRequest, nil
}

// ClearSignerContextCache clears the cached SignerContexts used by the off-chain
// signing path.
//
// The cache is keyed by ring pointer and is UNBOUNDED: it grows one entry per
// distinct ring (i.e. per session) and is never evicted automatically. Callers
// using SignOffChain/SignOffChainWithRing MUST call this on session rollover,
// once the old rings are no longer used, or memory leaks slowly over time.
func (s *Signer) ClearSignerContextCache() {
	s.signerContextCache = sync.Map{}
}
