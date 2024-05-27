package client

import "github.com/pokt-network/shannon-sdk/sdk"

var _ sdk.Signer = (*signer)(nil)

// signer is a Signer simple implementation that uses a cached private key.
type signer struct {
	privateKeyHex string
}

// NewSigner creates a new signer with the provided private key.
func NewSigner(privateKeyHex string) (sdk.Signer, error) {
	return &signer{
		privateKeyHex: privateKeyHex,
	}, nil
}

// GetPrivateKeyHex returns the private key of the signer in hex format.
func (s *signer) GetPrivateKeyHex() string {
	return s.privateKeyHex
}
