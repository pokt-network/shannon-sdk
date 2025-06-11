package sdk

import (
	"context"
	"encoding/hex"
	"fmt"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	"github.com/pokt-network/ring-go"
)

// TODO_IN_THIS_PR(@commoddity): move this to its own file.
type FullNode interface {
	// GetApp returns the onchain application matching the application address
	GetApp(ctx context.Context, appAddr string) (*apptypes.Application, error)

	// GetSession returns the latest session matching the supplied service+app combination.
	// Sessions are solely used for sending relays, and therefore only the latest session for any service+app combination is needed.
	// Note: Shannon returns the latest session for a service+app combination if no blockHeight is provided.
	GetSession(ctx context.Context, serviceID ServiceID, appAddr string) (sessiontypes.Session, error)

	// GetAccountPubKey returns the account public key for the given address.
	// The cache has no TTL, so the public key is cached indefinitely.
	GetAccountPubKey(ctx context.Context, address string) (cryptotypes.PubKey, error)

	// ValidateRelayResponse validates the raw bytes returned from an endpoint (in response to a relay request) and returns the parsed response.
	ValidateRelayResponse(supplierAddr SupplierAddress, responseBz []byte) (*servicetypes.RelayResponse, error)

	// IsHealthy returns true if the FullNode instance is healthy.
	// A LazyFullNode will always return true.
	// A CachingFullNode will return true if it has data in app and session caches.
	IsHealthy() bool
}

// Structs & Interfaces
// --------------------
// Signer holds the application or gateway's private key used to sign Relay Requests.
type Signer struct {
	PrivateKeyHex string
	FullNode      FullNode
}

// Methods
// -------
// Sign signs the given relay request using the signer's private key and the application's ring.
//
// - Returns a pointer instead of directly setting the signature on the input relay request to avoid implicit output.
// - Ideally, the function should accept a struct rather than a pointer, and also return an updated struct instead of a pointer.
func (s *Signer) SignRelayRequest(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	app apptypes.Application,
) (*servicetypes.RelayRequest, error) {
	appRing := ApplicationRing{
		Application:      app,
		PublicKeyFetcher: s.FullNode,
	}

	// Get the session ring for the application's session end block height
	sessionRing, err := appRing.GetRing(ctx, uint64(relayRequest.Meta.SessionHeader.SessionEndBlockHeight))
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error getting a ring for application address %s: %w",
			appRing.Application.Address,
			err,
		)
	}

	// Get the signable bytes hash from the relay request
	signableBz, err := relayRequest.GetSignableBytesHash()
	if err != nil {
		return nil, fmt.Errorf("Sign: error getting signable bytes hash from the relay request: %w", err)
	}

	// TODO_IMPROVE:
	// - Store the private key as a scalar in Signer to reduce processing steps per Relay Request.
	signerPrivKeyBz, err := hex.DecodeString(s.PrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("Sign: error decoding private key to a string: %w", err)
	}

	signerPrivKey, err := ring.Secp256k1().DecodeToScalar(signerPrivKeyBz)
	if err != nil {
		return nil, fmt.Errorf("Sign: error decoding private key to a scalar: %w", err)
	}

	// Sign the request using the session ring and signer's private key
	ringSig, err := sessionRing.Sign(signableBz, signerPrivKey)
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error signing using the ring of application with address %s: %w",
			appRing.Application.Address,
			err,
		)
	}

	// Serialize the signature
	signature, err := ringSig.Serialize()
	if err != nil {
		return nil, fmt.Errorf(
			"Sign: error serializing the signature of application with address %s: %w",
			appRing.Application.Address,
			err,
		)
	}

	// Set the signature on the relay request
	relayRequest.Meta.Signature = signature
	return relayRequest, nil
}
