// Package client error definitions for Shannon SDK gateway operations.
package client

import "errors"

// Configuration validation errors
var (
	errShannonInvalidNodeURL            = errors.New("invalid node URL")
	errShannonInvalidGrpcHostPort       = errors.New("invalid grpc host port")
	errShannonCacheConfigSetForLazyMode = errors.New("session TTL cannot be set when caching is disabled")
)

// GatewayClient.GetActiveSessions errors
var (
	ErrProtocolContextSetupNoAppAddresses = errors.New("no app addresses provided for service")
	ErrProtocolContextSetupAppFetchErr    = errors.New("error getting onchain data for app owned by the gateway")
	ErrProtocolContextSetupAppDelegation  = errors.New("app does not delegate to the gateway")
	ErrProtocolContextSetupNoSessions     = errors.New("no active sessions could be retrieved for the service")
	ErrProtocolContextSetupAppNotStaked   = errors.New("app is not staked for the service")
)

// GatewayClient.SignRelayRequest errors
var (
	ErrSignRelayRequestAppFetchErr            = errors.New("error getting a ring for application address")
	ErrSignRelayRequestSignableBytesHash      = errors.New("error getting signable bytes hash from the relay request")
	ErrSignRelayRequestSignerPrivKey          = errors.New("error decoding private key to a string")
	ErrSignRelayRequestSignerPrivKeyDecode    = errors.New("error decoding private key to a scalar")
	ErrSignRelayRequestSignerPrivKeySign      = errors.New("error signing the request using the ring of application")
	ErrSignRelayRequestSignerPrivKeySerialize = errors.New("error serializing the signature of application")
)

// GatewayClient.ValidateRelayResponse errors
var (
	ErrValidateRelayResponseUnmarshal     = errors.New("error unmarshalling the relay response")
	ErrValidateRelayResponseValidateBasic = errors.New("error validating the relay response")
	ErrValidateRelayResponseAccountPubKey = errors.New("error getting the account public key")
	ErrValidateRelayResponsePubKeyNil     = errors.New("supplier public key is nil for address")
	ErrValidateRelayResponseSignature     = errors.New("error verifying the supplier's signature")
)

// GRPCClient data fetching errors
var (
	ErrGRPCClientGetAppError           = errors.New("error getting the application")
	ErrGRPCClientGetSessionError       = errors.New("error getting the session for service")
	ErrGRPCClientGetSessionNilSession  = errors.New("got nil session for service")
	ErrGRPCClientGetAccountPubKeyError = errors.New("error getting the account public key")
	ErrGRPCClientGetAccountPubKeyNil   = errors.New("got nil account public key")
)
