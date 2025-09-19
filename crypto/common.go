package crypto

import (
	"context"
	"encoding/hex"
	"fmt"

	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	"github.com/pokt-network/ring-go"
)

// commonSign provides the shared signing logic for all crypto backends.
// This eliminates duplication between different secp256k1 implementations.
func commonSign(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	appRing ApplicationRing,
	privateKey PrivateKey,
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

	// Convert private key to ring-go scalar format
	signerPrivKeyBz, err := hex.DecodeString(privateKey.Hex())
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