// Package client provides blockchain data fetching and caching for Shannon SDK gateways.
//
// This package contains:
//   - GatewayClient: High-level client for signing relays and validating responses
//   - GatewayClientCache: Intelligent caching layer with block-based session refresh
//   - GRPCClient: Direct gRPC connection to Shannon full nodes
//   - Configuration types for flexible client setup
//
// The caching system uses SturdyC to provide:
//   - Block-aware session refresh (triggers at SessionEndBlockHeight+1)
//   - Zero-downtime cache swaps during session transitions
//   - Stampede protection for concurrent requests
//   - Infinite TTL for account public keys (immutable data)
package client

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/pokt-network/poktroll/pkg/polylog"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	"github.com/viccon/sturdyc"

	sdk "github.com/pokt-network/shannon-sdk"
)

// GatewayClientCache implements OnchainDataFetcher interface.
var _ OnchainDataFetcher = &GatewayClientCache{}

// Cache configuration constants
const (
	// Network timing constants
	blockTime            = 30 * time.Second // Estimated time per block
	blockPollingInterval = 1 * time.Second  // Polling frequency during intensive monitoring
	retryBaseDelay       = 100 * time.Millisecond

	// SturdyC cache configuration
	// Docs: https://github.com/viccon/sturdyc
	cacheCapacity      = 100_000 // Max entries across all shards
	numShards          = 10      // Number of cache shards for concurrency
	evictionPercentage = 10      // Percentage of LRU entries evicted when full

	// Cache key prefixes to avoid collisions
	sessionCacheKeyPrefix       = "session"
	accountPubKeyCacheKeyPrefix = "pubkey"
)

// noTTL represents infinite cache duration (~292 years)
// Used for immutable data like account public keys
const noTTL = time.Duration(math.MaxInt64)

// GatewayClientCache provides intelligent caching for Shannon blockchain data.
//
// Key features:
//   - Block-based session refresh: Monitors SessionEndBlockHeight instead of time-based TTL
//   - Zero-downtime transitions: Creates new cache instances and atomically swaps them
//   - Intelligent polling: Switches to 1-second polling when approaching session end
//   - Stampede protection: SturdyC prevents duplicate requests for the same data
//   - Infinite caching: Account public keys cached forever (immutable data)
//
// Documentation: https://github.com/viccon/sturdyc
type GatewayClientCache struct {
	logger             polylog.Logger
	onchainDataFetcher OnchainDataFetcher

	// Session cache with block-based refresh monitoring
	sessionCache        *sturdyc.Client[sessiontypes.Session]
	sessionCacheMu      sync.RWMutex
	sessionRefreshState *sessionRefreshState

	// Account public key cache with infinite TTL
	accountPubKeyCache *sturdyc.Client[cryptotypes.PubKey]
}

// NewGatewayClientCache creates a new caching layer around an OnchainDataFetcher.
//
// The cache automatically starts background session monitoring and will refresh
// sessions based on blockchain height rather than time-based TTL.
func NewGatewayClientCache(
	logger polylog.Logger,
	dataFetcher OnchainDataFetcher,
	cacheConfig CacheConfig,
) (*GatewayClientCache, error) {
	logger = logger.With("client", "gateway_client_cache")

	// Hydrate default values for cache configuration
	cacheConfig.hydrateDefaults()

	logger.Info().
		Bool("use_cache", *cacheConfig.UseCache).
		Msg("Cache configuration")

	gcc := &GatewayClientCache{
		logger:             logger,
		onchainDataFetcher: dataFetcher,

		sessionCache: getCache[sessiontypes.Session](),
		sessionRefreshState: &sessionRefreshState{
			activeSessionKeys: make(map[string]sessionKeyInfo),
		},

		accountPubKeyCache: getCache[cryptotypes.PubKey](),
	}

	// Start background session monitoring
	gcc.startSessionMonitoring()

	return gcc, nil
}

// getCache creates a SturdyC cache instance with infinite TTL
func getCache[T any]() *sturdyc.Client[T] {
	return sturdyc.New[T](
		cacheCapacity,
		numShards,
		noTTL,
		evictionPercentage,
	)
}

// GetApp fetches application data directly from the full node without caching.
//
// Applications are not cached because:
//   - Only needed during gateway startup for service configuration
//   - Runtime access to applications happens via sessions (which contain the app)
//   - Reduces cache complexity for rarely-accessed data
func (gcc *GatewayClientCache) GetApp(ctx context.Context, appAddr string) (apptypes.Application, error) {
	return gcc.onchainDataFetcher.GetApp(ctx, appAddr)
}

// GetSession returns a session from cache or fetches it from the blockchain.
//
// On cache miss, this method:
//   - Fetches the session from the full node
//   - Updates the global session end height for monitoring
//   - Tracks the session key for background refresh
//   - Caches the session with infinite TTL (refreshed by block monitoring)
//
// SturdyC provides automatic stampede protection for concurrent requests.
func (gcc *GatewayClientCache) GetSession(
	ctx context.Context,
	serviceID sdk.ServiceID,
	appAddr string,
) (sessiontypes.Session, error) {
	sessionKey := getSessionCacheKey(serviceID, appAddr)

	// Get current cache instance with read lock
	gcc.sessionCacheMu.RLock()
	sessionCache := gcc.sessionCache
	gcc.sessionCacheMu.RUnlock()

	// SturdyC GetOrFetch provides stampede protection
	session, err := sessionCache.GetOrFetch(
		ctx,
		sessionKey,
		func(fetchCtx context.Context) (sessiontypes.Session, error) {
			gcc.logger.Debug().
				Str("session_key", sessionKey).
				Msgf("Cache miss - fetching session from full node for service %s", serviceID)

			session, err := gcc.onchainDataFetcher.GetSession(fetchCtx, serviceID, appAddr)
			if err != nil {
				return session, err
			}

			// Register session for block-based monitoring
			gcc.updateSessionEndHeight(session)

			// Track for background refresh during session transitions
			gcc.trackActiveSession(sessionKey, serviceID, appAddr)

			return session, nil
		},
	)

	return session, err
}

// getSessionCacheKey creates a unique cache key: "session:<serviceID>:<appAddr>"
func getSessionCacheKey(serviceID sdk.ServiceID, appAddr string) string {
	return fmt.Sprintf("%s:%s:%s", sessionCacheKeyPrefix, serviceID, appAddr)
}

// GetAccountPubKey returns an account's public key from cache or blockchain.
//
// Account public keys are cached with infinite TTL because they never change.
// The fetchFn is only called once per address during the application lifetime.
func (gcc *GatewayClientCache) GetAccountPubKey(
	ctx context.Context,
	address string,
) (pubKey cryptotypes.PubKey, err error) {
	return gcc.accountPubKeyCache.GetOrFetch(
		ctx,
		getAccountPubKeyCacheKey(address),
		func(fetchCtx context.Context) (cryptotypes.PubKey, error) {
			gcc.logger.Debug().
				Str("account_key", getAccountPubKeyCacheKey(address)).
				Msg("Cache miss - fetching account public key from full node")

			return gcc.onchainDataFetcher.GetAccountPubKey(fetchCtx, address)
		},
	)
}

// getAccountPubKeyCacheKey creates a unique cache key: "pubkey:<address>"
func getAccountPubKeyCacheKey(address string) string {
	return fmt.Sprintf("%s:%s", accountPubKeyCacheKeyPrefix, address)
}

// LatestBlockHeight returns the current blockchain height from the full node.
// This method is not cached as block height changes frequently.
func (gcc *GatewayClientCache) LatestBlockHeight(ctx context.Context) (height int64, err error) {
	return gcc.onchainDataFetcher.LatestBlockHeight(ctx)
}

// IsHealthy reports the health status of the cache.
// Currently always returns true as the cache is populated on-demand.
//
// TODO_IMPROVE: Add meaningful health checks:
//   - Verify cache connectivity
//   - Check session refresh monitoring status
//   - Validate recent successful fetches
func (gcc *GatewayClientCache) IsHealthy() bool {
	return true
}
