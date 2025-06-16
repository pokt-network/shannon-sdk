package client

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/pokt-network/poktroll/pkg/crypto/rings"
	"github.com/pokt-network/poktroll/x/application/types"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	"github.com/pokt-network/ring-go"
)

// publicKeyFetcher allows fetching the public key for a given address.
// Satisfied by a full node, either with or without caching.
type publicKeyFetcher interface {
	GetAccountPubKey(ctx context.Context, address string) (cryptotypes.PubKey, error)
}

// relaySigner holds the application or gateway's private key used to sign Relay Requests
// and the public key fetcher to fetch the public key for the application.
//
// TODO_TECHDEBT(@commoddity): Investigate an alternative to requiring the public key fetcher.
type relaySigner struct {
	PrivateKeyHex    string
	PublicKeyFetcher publicKeyFetcher
}

// SignRelayRequest signs the given relay request using the signer's private key and the application's ring.
//
// - Returns a pointer instead of directly setting the signature on the input relay request to avoid implicit output.
// - Ideally, the function should accept a struct rather than a pointer, and also return an updated struct instead of a pointer.
func (s *relaySigner) SignRelayRequest(
	ctx context.Context,
	relayRequest *servicetypes.RelayRequest,
	app apptypes.Application,
) (*servicetypes.RelayRequest, error) {
	appRing := ApplicationRing{
		Application:      app,
		publicKeyFetcher: s.PublicKeyFetcher,
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

// TODO_IN_THIS_PR(@commoddity): add detailed godoc comment explaining the purpose of this struct.
type ApplicationRing struct {
	types.Application
	publicKeyFetcher
}

// GetRing returns the ring for the application until the current session end height.
//
// - Ring is created using the application's public key and the public keys of gateways currently delegated from the application
// - Returns error if PublicKeyFetcher is not set or any pubkey fetch fails
func (a ApplicationRing) GetRing(
	ctx context.Context,
	sessionEndHeight uint64,
) (addressRing *ring.Ring, err error) {
	if a.publicKeyFetcher == nil {
		return nil, errors.New("GetRing: Public Key Fetcher not set")
	}

	currentGatewayAddresses := rings.GetRingAddressesAtSessionEndHeight(&a.Application, sessionEndHeight)

	ringAddresses := make([]string, 0)
	ringAddresses = append(ringAddresses, a.Application.Address)

	if len(currentGatewayAddresses) == 0 {
		ringAddresses = append(ringAddresses, a.Application.Address)
	} else {
		ringAddresses = append(ringAddresses, currentGatewayAddresses...)
	}

	ringPubKeys := make([]cryptotypes.PubKey, 0, len(ringAddresses))
	for _, address := range ringAddresses {
		pubKey, err := a.publicKeyFetcher.GetAccountPubKey(ctx, address)
		if err != nil {
			return nil, err
		}
		ringPubKeys = append(ringPubKeys, pubKey)
	}

	return rings.GetRingFromPubKeys(ringPubKeys)
}
