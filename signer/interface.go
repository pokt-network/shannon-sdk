package signer

type Signer interface {
	GetPrivateKeyHex() string
}
