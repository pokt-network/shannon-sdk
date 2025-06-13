package fullnode

import (
	"errors"
	"net"
	"net/url"
	"time"
)

var (
	ErrShannonInvalidNodeUrl            = errors.New("invalid node URL")
	ErrShannonInvalidGrpcHostPort       = errors.New("invalid grpc host port")
	ErrShannonCacheConfigSetForLazyMode = errors.New("cache config cannot be set for lazy mode")
)

type FullNodeConfig struct {
	RpcURL     string     `yaml:"rpc_url"`
	GRPCConfig GRPCConfig `yaml:"grpc_config"`

	// LazyMode, if set to true, will disable all caching of onchain data. For
	// example, this disables caching of apps and sessions.
	LazyMode bool `yaml:"lazy_mode" default:"true"`

	// Configuration options for the cache when LazyMode is false
	CacheConfig CacheConfig `yaml:"cache_config"`
}

func (c FullNodeConfig) Validate() error {
	if !isValidURL(c.RpcURL) {
		return ErrShannonInvalidNodeUrl
	}
	if !isValidHostPort(c.GRPCConfig.HostPort) {
		return ErrShannonInvalidGrpcHostPort
	}
	if err := c.CacheConfig.validate(c.LazyMode); err != nil {
		return err
	}
	return nil
}

type CacheConfig struct {
	SessionTTL time.Duration `yaml:"session_ttl"`
}

// TODO_NEXT(@commoddity): Session refresh handling should be significantly reworked as part of the next changes following PATH PR #297.
//
// The proposed change is to align session refreshes with actual session expiry time,
// using the session expiry block and the Shannon SDK's block client.
// When this is done, session cache TTL can be removed altogether.
//
// Session TTL should match the protocol's session length.
const defaultSessionCacheTTL = 30 * time.Second

func (c *CacheConfig) validate(lazyMode bool) error {
	if lazyMode && c.SessionTTL == 0 {
		return ErrShannonCacheConfigSetForLazyMode
	}
	return nil
}

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
