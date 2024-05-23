package signer

var _ Signer = (*signer)(nil)

type signer struct {
	privateKeyHex string
}

func NewSigner(privateKeyHex string) (Signer, error) {
	return &signer{
		privateKeyHex: privateKeyHex,
	}, nil
}

func (s *signer) GetPrivateKeyHex() string {
	return s.privateKeyHex
}
