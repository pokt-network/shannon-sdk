package sdk

import (
	"fmt"

	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
)

// Signer is used to sign Relay Requests.
type Signer struct {
	PrivateKeyHex string
}

// Note: Sign returns a pointer instead of directly setting the signature on the input relay request.
// This is done to avoid having an implicit output.
// Ideally, the function should accept a struct rather than a pointer, and also return an updated struct instead of a pointer.
//
// Sign signs the given relay request using the signer's private key
// and the application's ring signature.
func (s *Signer) Sign(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	// TODO_IMPROVE: this input argument should be changed to an interface.
	app ApplicationRing,
	queryHeight int64,
) (*servicetypes.RelayRequest, error) {
	appRing, err := app.GetRing(ctx, queryHeight)
	if err != nil {
		return nil, fmt.Errorf("Sign: error getting a ring for application address %s: %w", app.Address, err)
	}

	signableBz, err := relayRequest.GetSignableBytesHash()
	if err != nil {
		return nil, fmt.Errorf("Sign: error getting signable bytes hash from the relay request: %w", err)
	}

	// TODO_DISCUSS: should the Signer struct store the private key as scalar instead?
	// This would reduce the number of steps required for processing each Relay Request.
	signerPrivKeyBz, err := hex.DecodeString(s.PrivateKeyHex)
	if err != nil {
		return nil, fmt.Errorf("Sign: error decoding private key to a string: %w", err)
	}

	signerPrivKey, err := ring.Secp256k1().DecodeToScalar(signerPrivKeyBz)
	if err != nil {
		return nil, fmt.Errorf("Sign: error decoding private key to a scalar: %w", err)
	}

	ringSig, err := appRing.Sign(signableBz, signerPrivKey)
	if err != nil {
		return nil, fmt.Errorf("Sign: error signing using the ring of application with address %s: %w", app.Address, err)
	}

	signature, err := ringSig.Serialize()
	if err != nil {
		return nil, fmt.Errorf("Sign: error serializing the signature of application with address %s: %w", app.Address, err)
	}

	relayRequest.Meta.Signature = signature
	return relayRequest, nil
}
