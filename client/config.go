package client

import (
	"net"
	"net/url"
	"time"
)

// GatewayClientConfig is the configuration for the full node used by the GatewayClientCache.
type GatewayClientConfig struct {
	// GRPCConfig is the configuration for the gRPC connection to the full node.
	GRPCConfig GRPCConfig `yaml:"grpc_config"`
	// CacheConfig configures the caching behavior of the GatewayClientCache, including whether caching is enabled.
	CacheConfig CacheConfig `yaml:"cache_config"`
}

// Validate validates the GatewayClientConfig, both the GRPCConfig and the CacheConfig.
func (c GatewayClientConfig) Validate() error {
	if err := c.GRPCConfig.Validate(); err != nil {
		return err
	}
	if err := c.CacheConfig.Validate(); err != nil {
		return err
	}
	return nil
}

// GRPCConfig configures the gRPC connection to the full node.
type GRPCConfig struct {
	// RPC URL is used to make RPC calls to the full node.
	RpcURL string `yaml:"rpc_url"`
	// HostPort is the host and port of the full node.
	// eg: localhost:26657
	HostPort string `yaml:"host_port"`
	// UseInsecureGRPCConn determines if the gRPC connection to the full node should use TLS.
	// This is useful for local development.
	UseInsecureGRPCConn bool `yaml:"insecure"`
}

func (c GRPCConfig) Validate() error {
	if !isValidURL(c.RpcURL) {
		return errShannonInvalidNodeURL
	}
	if !isValidHostPort(c.HostPort) {
		return errShannonInvalidGrpcHostPort
	}
	return nil
}

// defaultUseCache is the default value for the use cache flag.
// It is set to true by default.
var defaultUseCache = true

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

// CacheConfig configures the caching behavior of the GatewayClientCache.
type CacheConfig struct {
	// UseCache determines if the cache should be used.
	// If set to `false`, the cache will not be used.
	UseCache *bool `yaml:"use_cache"`
	// SessionTTL is the time to live for the session cache.
	// Optional. If not set, the default session TTL will be used.
	SessionTTL time.Duration `yaml:"session_ttl"`
	// EarlyRefreshEnabled determines if the cache should be refreshed early.
	// If set to `true`, the cache will be refreshed early.
	EarlyRefreshEnabled *bool `yaml:"early_refresh_enabled"`
}

func (c CacheConfig) Validate() error {
	if c.UseCache != nil && !*c.UseCache && c.SessionTTL != 0 {
		return errShannonCacheConfigSetForLazyMode
	}
	return nil
}

// hydrateDefaults hydrates the cache configuration with defaults for any fields that are not set.
func (c *CacheConfig) hydrateDefaults() {
	if c.UseCache == nil {
		c.UseCache = &defaultUseCache
	}
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
