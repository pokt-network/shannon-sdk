package client

import "errors"

var (
	// Centralized gateway mode: Error getting onchain data for app
	ErrProtocolContextSetupCentralizedAppFetchErr = errors.New("error getting onchain data for app owned by the gateway")
	// Centralized gateway mode app does not delegate to the gateway.
	ErrProtocolContextSetupCentralizedAppDelegation = errors.New("centralized gateway mode app does not delegate to the gateway")
	// Centralized gateway mode: no active sessions could be retrieved for the service.
	ErrProtocolContextSetupCentralizedNoSessions = errors.New("no active sessions could be retrieved for the service")
	// Delegated gateway mode: could not fetch session for app from the full node
	ErrProtocolContextSetupFetchSession = errors.New("error getting a session from the full node for app")
	// Delegated gateway mode: app is not staked for the service.
	ErrProtocolContextSetupAppNotStaked = errors.New("app is not staked for the service")
	// Delegated gateway mode: gateway does not have delegation for the app.
	ErrProtocolContextSetupAppDoesNotDelegate = errors.New("gateway does not have delegation for app")
)
