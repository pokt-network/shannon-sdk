package client

import (
	"errors"
	"net"
	"net/url"
	"time"
)

var (
	errShannonInvalidNodeURL            = errors.New("invalid node URL")
	errShannonInvalidGrpcHostPort       = errors.New("invalid grpc host port")
	errShannonCacheConfigSetForLazyMode = errors.New("session TTL cannot be set when caching is disabled")
)

type (
	// FullNodeConfig is the configuration for the full node used by the GatewayClient.
	FullNodeConfig struct {
		// RPC URL is used to make RPC calls to the full node.
		RpcURL string `yaml:"rpc_url"`
		// GRPCConfig is the configuration for the gRPC connection to the full node.
		GRPCConfig GRPCConfig `yaml:"grpc_config"`
		// CacheConfig configures the caching behavior of the full node, including whether caching is enabled.
		CacheConfig CacheConfig `yaml:"cache_config"`
	}

	// GRPCConfig configures the gRPC connection to the full node.
	GRPCConfig struct {
		// HostPort is the host and port of the full node.
		// eg: localhost:26657
		HostPort string `yaml:"host_port"`
		// UseInsecureGRPCConn determines if the gRPC connection to the full node should use TLS.
		// This is useful for local development.
		UseInsecureGRPCConn bool `yaml:"insecure"`
	}

	// CacheConfig configures the caching behavior of the full node, including whether caching is enabled.
	CacheConfig struct {
		// CachingEnabled determines if the full node should use caching.
		// If set to `false`, the full node will not use caching, and will be returned directly.
		// If set to `true`, the full node will be wrapped in a SturdyC-based cache.
		CachingEnabled bool `yaml:"caching_enabled"`
		// SessionTTL is the time to live for the session cache.
		// Optional. If not set, the default session TTL will be used.
		SessionTTL time.Duration `yaml:"session_ttl"`
	}
)

func (c FullNodeConfig) Validate() error {
	if !isValidURL(c.RpcURL) {
		return errShannonInvalidNodeURL
	}
	if !isValidHostPort(c.GRPCConfig.HostPort) {
		return errShannonInvalidGrpcHostPort
	}
	if err := c.CacheConfig.validate(); err != nil {
		return err
	}
	return nil
}

// defaultSessionCacheTTL is the default time to live for the session cache.
// It should match the protocol's session length.
//
// TODO_NEXT(@commoddity): Session refresh handling should be significantly reworked as part
// of the next changes following PATH PR #297.
// The proposed change is to align session refreshes with actual session expiry time,
// using the session expiry block and the Shannon SDK's block client.
// When this is done, session cache TTL can be removed altogether.
const defaultSessionCacheTTL = 30 * time.Second

// validate validates the cache configuration for the full node.
func (c *CacheConfig) validate() error {
	// Cannot set both lazy mode and cache configuration.
	if !c.CachingEnabled && c.SessionTTL != 0 {
		return errShannonCacheConfigSetForLazyMode
	}
	return nil
}

// hydrateDefaults hydrates the cache configuration with defaults for any fields that are not set.
func (c *CacheConfig) hydrateDefaults() {
	if c.SessionTTL == 0 {
		c.SessionTTL = defaultSessionCacheTTL
	}
}

// isValidURL returns true if the supplied URL string can be parsed into a valid URL accepted by the Shannon SDK.
func isValidURL(urlStr string) bool {
	u, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	if u.Scheme == "" || u.Host == "" {
		return false
	}

	return true
}

// isValidHostPort returns true if the supplied string can be parsed into a host and port combination.
func isValidHostPort(hostPort string) bool {
	host, port, err := net.SplitHostPort(hostPort)

	if err != nil {
		return false
	}

	if host == "" || port == "" {
		return false
	}

	return true
}
