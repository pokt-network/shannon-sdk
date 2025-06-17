package client

import (
	"context"
	"fmt"
	"math"
	"time"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/pokt-network/poktroll/pkg/polylog"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	"github.com/viccon/sturdyc"

	sdk "github.com/pokt-network/shannon-sdk"
)

// ---------------- Cache Configuration ----------------
const (
	// Retry base delay for exponential backoff on failed refreshes
	retryBaseDelay = 100 * time.Millisecond

	// cacheCapacity:
	//   - Max entries across all shards (not per-shard)
	//   - Exceeding capacity triggers LRU eviction per shard
	//   - 100k supports most large deployments
	//   - TODO_TECHDEBT(@commoddity): Revisit based on real-world usage; consider making configurable
	cacheCapacity = 100_000

	// numShards:
	//   - Number of independent cache shards for concurrency
	//   - Reduces lock contention, improves parallelism
	//   - 10 is a good balance for most workloads
	numShards = 10

	// evictionPercentage:
	//   - % of LRU entries evicted per shard when full
	//   - 10% = incremental cleanup, avoids memory spikes
	//   - SturdyC also evicts expired entries in background
	evictionPercentage = 10

	// TODO_TECHDEBT(@commoddity): See Issue #291 for improvements to refresh logic
	// minEarlyRefreshPercentage:
	//   - Earliest point (as % of TTL) to start background refresh
	//   - 0.75 = 75% of TTL (e.g. 22.5s for 30s TTL)
	minEarlyRefreshPercentage = 0.75

	// maxEarlyRefreshPercentage:
	//   - Latest point (as % of TTL) to start background refresh
	//   - 0.9 = 90% of TTL (e.g. 27s for 30s TTL)
	//   - Ensures refresh always completes before expiry
	maxEarlyRefreshPercentage = 0.9
)

// pubKeyCacheTTL: No TTL for the account public key cache since account data never changes.
//
// time.Duration(math.MaxInt64) equals ~292 years, which is effectively infinite.
const pubKeyCacheTTL = time.Duration(math.MaxInt64)

// getCacheDelays returns the min/max delays for SturdyC's Early Refresh strategy.
// - Proactively refreshes cache before expiry (prevents misses/latency spikes)
// - Refresh window: 75-90% of TTL (e.g. 22.5-27s for 30s TTL)
// - Spreads requests to avoid thundering herd
// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#early-refreshes
func getCacheDelays(ttl time.Duration) (min, max time.Duration) {
	minFloat := float64(ttl) * minEarlyRefreshPercentage
	maxFloat := float64(ttl) * maxEarlyRefreshPercentage

	// Round to the nearest second
	min = time.Duration(minFloat/float64(time.Second)+0.5) * time.Second
	max = time.Duration(maxFloat/float64(time.Second)+0.5) * time.Second
	return
}

// Prefix for cache keys to avoid collisions with other keys.
const (
	sessionCacheKeyPrefix       = "session"
	accountPubKeyCacheKeyPrefix = "pubkey"
)

// cachingFullNode wraps a LazyFullNode with SturdyC-based caching.
// - Early refresh: background updates before expiry (prevents thundering herd/latency spikes)
// - Example: 30s TTL, refresh at 22.5–27s (75–90%)
// - Benefits: zero-latency reads, graceful degradation, auto load balancing
// Docs: https://github.com/viccon/sturdyc
type fullNodeWithCache struct {
	logger polylog.Logger

	// Underlying node for protocol data fetches
	underlyingFullNode *fullNode

	// Session cache (5 min on Beta TestNet, see #275)
	// TODO_MAINNET_MIGRATION(@Olshansk): Revisit after mainnet
	sessionCache *sturdyc.Client[sessiontypes.Session]

	// The account public key cache; used to cache account public keys indefinitely.
	// It has an infinite TTL and is populated only once on startup.
	accountPubKeyCache *sturdyc.Client[cryptotypes.PubKey]
}

// newFullNodeWithCache wraps a fullNode with:
//   - Session cache: refreshes early to avoid thundering herd/latency spikes
//   - Account public key cache: indefinite cache for account data
func newFullNodeWithCache(
	logger polylog.Logger,
	underlyingFullNode *fullNode,
	sessionTTL time.Duration,
) (*fullNodeWithCache, error) {
	logger = logger.With("full_node_type", "cachingFfullNodeWithCacheullNode")

	// Log cache configuration
	logger.Debug().
		Str("cache_config_session_ttl", sessionTTL.String()).
		Msgf("Cache Configuration")

	// Configure session cache with early refreshes
	sessionMinRefreshDelay, sessionMaxRefreshDelay := getCacheDelays(sessionTTL)

	// Create the session cache with early refreshes
	sessionCache := sturdyc.New[sessiontypes.Session](
		cacheCapacity,
		numShards,
		sessionTTL,
		evictionPercentage,
		// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#early-refreshes
		sturdyc.WithEarlyRefreshes(
			sessionMinRefreshDelay,
			sessionMaxRefreshDelay,
			sessionTTL,
			retryBaseDelay,
		),
	)

	// Create the account cache, which is effectively infinite
	// caching for the lifetime of the application.
	// No need to configure early refreshes with SturdyC.
	accountPubKeyCache := sturdyc.New[cryptotypes.PubKey](
		cacheCapacity,
		numShards,
		pubKeyCacheTTL,
		evictionPercentage,
	)

	return &fullNodeWithCache{
		logger:             logger,
		underlyingFullNode: underlyingFullNode,
		sessionCache:       sessionCache,
		accountPubKeyCache: accountPubKeyCache,
	}, nil
}

// GetApp passthrough to the underlying full node to ensure the request is always a remote request to the full node.
// Apps are fetched only at startup; relaying fetches sessions for app/session sync).
func (fnc *fullNodeWithCache) GetApp(ctx context.Context, appAddr string) (*apptypes.Application, error) {
	return fnc.underlyingFullNode.GetApp(ctx, appAddr)
}

// GetSession returns (and auto-refreshes) the session for a service/app from cache.
func (fnc *fullNodeWithCache) GetSession(
	ctx context.Context,
	serviceID sdk.ServiceID,
	appAddr string,
) (sessiontypes.Session, error) {
	// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#get-or-fetch
	return fnc.sessionCache.GetOrFetch(
		ctx,
		getSessionCacheKey(serviceID, appAddr),
		func(fetchCtx context.Context) (sessiontypes.Session, error) {
			fnc.logger.Debug().
				Str("session_key", getSessionCacheKey(serviceID, appAddr)).
				Msgf("GetSession: Making request to full node for service %s", serviceID)
			return fnc.underlyingFullNode.GetSession(fetchCtx, serviceID, appAddr)
		},
	)
}

// getSessionCacheKey builds a unique cache key for session: <prefix>:<serviceID>:<appAddr>
func getSessionCacheKey(serviceID sdk.ServiceID, appAddr string) string {
	return fmt.Sprintf("%s:%s:%s", sessionCacheKeyPrefix, serviceID, appAddr)
}

// GetAccountPubKey returns the account public key for the given address.
// The getAccountPubKey has no TTL, so the public key is cached indefinitely.
//
// The `fetchFn` param of `GetOrFetch` is only called once per address on startup.
func (fnc *fullNodeWithCache) getAccountPubKey(
	ctx context.Context,
	address string,
) (pubKey cryptotypes.PubKey, err error) {
	// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#get-or-fetch
	return fnc.accountPubKeyCache.GetOrFetch(
		ctx,
		getAccountPubKeyCacheKey(address),
		func(fetchCtx context.Context) (cryptotypes.PubKey, error) {
			fnc.logger.Debug().
				Str("account_key", getAccountPubKeyCacheKey(address)).
				Msgf("GetAccountPubKey: Making request to full node")
			return fnc.underlyingFullNode.getAccountPubKey(fetchCtx, address)
		},
	)
}

// getAccountPubKeyCacheKey returns the cache key for the given account address.
// It uses the accountPubKeyCacheKeyPrefix and the account address to create a unique key.
//
// eg. "pubkey:pokt1up7zlytnmvlsuxzpzvlrta95347w322adsxslw"
func getAccountPubKeyCacheKey(address string) string {
	return fmt.Sprintf("%s:%s", accountPubKeyCacheKeyPrefix, address)
}

// IsHealthy: passthrough to underlying node.
// TODO_IMPROVE(@commoddity):
//   - Add smarter health checks (e.g. verify cached apps/sessions)
//   - Currently always true (cache fills as needed)
func (fnc *fullNodeWithCache) IsHealthy() bool {
	return fnc.underlyingFullNode.IsHealthy()
}
