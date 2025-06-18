package client

import (
	"net"
	"net/url"
	"time"
)

type (
	// FullNodeConfig is the configuration for the full node used by the GatewayClientCache.
	FullNodeConfig struct {
		// RPC URL is used to make RPC calls to the full node.
		RpcURL string `yaml:"rpc_url"`
		// GRPCConfig is the configuration for the gRPC connection to the full node.
		GRPCConfig GRPCConfig `yaml:"grpc_config"`
		// CacheConfig configures the caching behavior of the GatewayClientCache, including whether caching is enabled.
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

	// CacheConfig configures the caching behavior of the GatewayClientCache, including whether caching is enabled.
	CacheConfig struct {
		// SessionTTL is the time to live for the session cache.
		// Optional. If not set, the default session TTL will be used.
		SessionTTL time.Duration `yaml:"session_ttl"`
		// EarlyRefreshEnabled determines if the cache should be refreshed early.
		// If set to `true`, the cache will be refreshed early.
		EarlyRefreshEnabled *bool `yaml:"early_refresh_enabled"`
	}
)

func (c FullNodeConfig) Validate() error {
	if !isValidURL(c.RpcURL) {
		return errShannonInvalidNodeURL
	}
	if !isValidHostPort(c.GRPCConfig.HostPort) {
		return errShannonInvalidGrpcHostPort
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

// defaultEarlyRefreshEnabled is the default value for the early refresh enabled flag.
// It is set to true by default.
var defaultEarlyRefreshEnabled = true

// hydrateDefaults hydrates the cache configuration with defaults for any fields that are not set.
func (c *CacheConfig) hydrateDefaults() {
	if c.SessionTTL == 0 {
		c.SessionTTL = defaultSessionCacheTTL
	}
	if c.EarlyRefreshEnabled == nil {
		c.EarlyRefreshEnabled = &defaultEarlyRefreshEnabled
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
