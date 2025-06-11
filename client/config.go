package client

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

const (
	// Shannon uses secp256k1 key schemes (the cosmos default)
	// secp256k1 keys are 32 bytes -> 64 hexadecimal characters
	// Ref: https://docs.cosmos.network/v0.45/basics/accounts.html
	shannonPrivateKeyLengthHex = 64
	// secp256k1 keys are 20 bytes, but are then bech32 encoded -> 43 bytes
	// Ref: https://docs.cosmos.network/main/build/spec/addresses/bech32
	shannonAddressLengthBech32 = 43
)

var (
	ErrShannonInvalidGatewayPrivateKey                = errors.New("invalid shannon gateway private key")
	ErrShannonInvalidGatewayAddress                   = errors.New("invalid shannon gateway address")
	ErrShannonUnsupportedGatewayMode                  = errors.New("invalid shannon gateway mode")
	ErrShannonCentralizedGatewayModeRequiresOwnedApps = errors.New("shannon Centralized gateway mode requires at-least 1 owned app")
)

// TODO_NEXT(@commoddity): Move gateway config to SDK gateway client package
type (
	GatewayConfig struct {
		GatewayMode             GatewayMode `yaml:"gateway_mode"`
		GatewayAddress          string      `yaml:"gateway_address"`
		GatewayPrivateKeyHex    string      `yaml:"gateway_private_key_hex"`
		OwnedAppsPrivateKeysHex []string    `yaml:"owned_apps_private_keys_hex"`
	}

	// TODO_TECHDEBT(@adshmh): Move this and related helpers into a new `grpc` package.ß
	GRPCConfig struct {
		HostPort          string        `yaml:"host_port"`
		Insecure          bool          `yaml:"insecure"`
		BackoffBaseDelay  time.Duration `yaml:"backoff_base_delay"`
		BackoffMaxDelay   time.Duration `yaml:"backoff_max_delay"`
		MinConnectTimeout time.Duration `yaml:"min_connect_timeout"`
		KeepAliveTime     time.Duration `yaml:"keep_alive_time"`
		KeepAliveTimeout  time.Duration `yaml:"keep_alive_timeout"`
	}
)

func (gc GatewayConfig) Validate() error {
	if len(gc.GatewayPrivateKeyHex) != shannonPrivateKeyLengthHex {
		return ErrShannonInvalidGatewayPrivateKey
	}
	if len(gc.GatewayAddress) != shannonAddressLengthBech32 {
		return ErrShannonInvalidGatewayAddress
	}
	if !strings.HasPrefix(gc.GatewayAddress, "pokt1") {
		return ErrShannonInvalidGatewayAddress
	}

	if !slices.Contains(supportedGatewayModes(), gc.GatewayMode) {
		return fmt.Errorf("%w: %s", ErrShannonUnsupportedGatewayMode, gc.GatewayMode)
	}

	if gc.GatewayMode == GatewayModeCentralized && len(gc.OwnedAppsPrivateKeysHex) == 0 {
		return ErrShannonCentralizedGatewayModeRequiresOwnedApps
	}

	for index, privKey := range gc.OwnedAppsPrivateKeysHex {
		if len(privKey) != shannonPrivateKeyLengthHex {
			return fmt.Errorf("%w: invalid owned app private key at index: %d", ErrShannonInvalidGatewayPrivateKey, index)
		}
	}

	return nil
}
