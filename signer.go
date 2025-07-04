package sdk

import (
	"context"
	"encoding/hex"
	"fmt"

	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	"github.com/pokt-network/ring-go"
)

// Structs & Interfaces
// --------------------
// Signer holds the application or gateway's private key used to sign Relay Requests.
type Signer struct {
	PrivateKeyHex string
}

// Methods
// -------
// Sign signs the given relay request using the signer's private key and the application's ring.
//
// - Returns a pointer instead of directly setting the signature on the input relay request to avoid implicit output.
// - Ideally, the function should accept a struct rather than a pointer, and also return an updated struct instead of a pointer.
func (s *Signer) Sign(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	appRing ApplicationRing, // TODO_IMPROVE: this input argument should be changed to an interface.
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
