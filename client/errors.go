package client

import "errors"

var (
	errShannonInvalidNodeURL            = errors.New("invalid node URL")
	errShannonInvalidGrpcHostPort       = errors.New("invalid grpc host port")
	errShannonCacheConfigSetForLazyMode = errors.New("session TTL cannot be set when caching is disabled")
)

var (
	// could not get onchain data for app
	ErrProtocolContextSetupAppFetchErr = errors.New("error getting onchain data for app owned by the gateway")
	// app does not delegate to the gateway
	ErrProtocolContextSetupAppDelegation = errors.New("app does not delegate to the gateway")
	// no active sessions could be retrieved for the service.
	ErrProtocolContextSetupNoSessions = errors.New("no active sessions could be retrieved for the service")
	// app is not staked for the service.
	ErrProtocolContextSetupAppNotStaked = errors.New("app is not staked for the service")
)
