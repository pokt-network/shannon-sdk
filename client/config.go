package client

import (
	"net"
	"net/url"
)

// GatewayClientConfig holds all configuration needed for Shannon SDK gateway clients.
type GatewayClientConfig struct {
	// GRPCConfig configures the connection to the Shannon full node
	GRPCConfig GRPCConfig `yaml:"grpc_config"`
	// CacheConfig configures the caching behavior and session refresh strategy
	CacheConfig CacheConfig `yaml:"cache_config"`
}

// Validate ensures all configuration values are valid and compatible
func (c GatewayClientConfig) Validate() error {
	if err := c.GRPCConfig.Validate(); err != nil {
		return err
	}
	return nil
}

// GRPCConfig configures the connection to a Shannon full node
type GRPCConfig struct {
	// RpcURL for making RPC calls (e.g., "http://localhost:26657")
	RpcURL string `yaml:"rpc_url"`
	// HostPort for gRPC connections (e.g., "localhost:9090")
	HostPort string `yaml:"host_port"`
	// UseInsecureGRPCConn disables TLS for local development
	UseInsecureGRPCConn bool `yaml:"insecure"`
}

// Validate ensures the gRPC configuration is valid and reachable
func (c GRPCConfig) Validate() error {
	if !isValidURL(c.RpcURL) {
		return errShannonInvalidNodeURL
	}
	if !isValidHostPort(c.HostPort) {
		return errShannonInvalidGrpcHostPort
	}
	return nil
}

// Default configuration values
var defaultUseCache = true

// CacheConfig controls session caching and refresh behavior
type CacheConfig struct {
	// UseCache enables/disables the caching layer entirely
	UseCache *bool `yaml:"use_cache"`
}

// hydrateDefaults fills in any missing configuration with sensible defaults
func (c *CacheConfig) hydrateDefaults() {
	if c.UseCache == nil {
		c.UseCache = &defaultUseCache
	}
}

// isValidURL validates that a string can be parsed as a complete URL
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

// isValidHostPort validates that a string represents a valid host:port combination
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
