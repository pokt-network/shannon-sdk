package client

import "github.com/pokt-network/shannon-sdk/sdk"

var _ sdk.Signer = (*signer)(nil)

type signer struct {
	privateKeyHex string
}

func NewSigner(privateKeyHex string) (sdk.Signer, error) {
	return &signer{
		privateKeyHex: privateKeyHex,
	}, nil
}

func (s *signer) GetPrivateKeyHex() string {
	return s.privateKeyHex
}
