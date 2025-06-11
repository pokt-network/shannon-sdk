package fullnode

import (
	"context"
	"fmt"
	"math"
	"time"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/pokt-network/poktroll/pkg/polylog"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	servicetypes "github.com/pokt-network/poktroll/x/service/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	"github.com/viccon/sturdyc"

	sdk "github.com/pokt-network/shannon-sdk"
)

// ---------------- Cache Configuration ----------------
const (
	// Retry base delay for exponential backoff on failed refreshes
	retryBaseDelay = 100 * time.Millisecond

	// cacheCapacity: Maximum number of entries the cache can hold across all shards.
	// This is the total capacity, not per-shard. When capacity is exceeded, the cache
	// will evict a percentage of the least recently used entries from each shard.
	// 100k entries should handle a large number of apps and sessions for most deployments.
	//
	// TODO_TECHDEBT(@commoddity): Revisit cache capacity based on real-world usage patterns.
	// Consider making this configurable and potentially different for apps vs sessions.
	cacheCapacity = 100_000

	// numShards: Number of independent cache shards for concurrent access.
	// SturdyC divides the cache into multiple shards to reduce lock contention and
	// improve performance under concurrent read/write operations. Each shard operates
	// independently with its own mutex, allowing parallel operations across shards.
	// 10 shards provides good balance between concurrency and memory overhead.
	numShards = 10

	// evictionPercentage: Percentage of entries to evict from each shard when capacity is reached.
	// When a shard reaches its capacity limit, this percentage of the least recently used (LRU)
	// entries will be removed to make space for new entries. 10% provides incremental cleanup
	// without causing large memory spikes during eviction cycles.
	// SturdyC also runs background eviction jobs to remove expired entries automatically.
	evictionPercentage = 10

	// minEarlyRefreshPercentage: Minimum percentage of the TTL before the cache early refresh may start.
	// For a 30-second TTL, this means refresh can start at 22.5 seconds (75% of 30s).
	minEarlyRefreshPercentage = 0.75

	// maxEarlyRefreshPercentage: Maximum percentage of the TTL before the cache early refresh may start.
	// For a 30-second TTL, this means refresh will definitely start by 27 seconds (90% of 30s),
	// giving a 3-second buffer before expiry to ensure items never exceed 30 seconds old.
	maxEarlyRefreshPercentage = 0.9
)

// pubKeyCacheTTL: No TTL for the account public key cache since account data never changes.
//
// time.Duration(math.MaxInt64) equals ~292 years, which is effectively infinite.
const pubKeyCacheTTL = time.Duration(math.MaxInt64)

// getCacheDelays gets the delays for the SturdyC Early Refresh Strategy.
//
// "Refreshing" in SturdyC means proactively fetching fresh data in the background
// BEFORE the cached entry expires. This prevents cache misses and eliminates latency
// spikes by ensuring hot data is always available immediately.
//
// Cache refresh timing is 75-90% of TTL (e.g. 22.5-27 seconds for 30-second TTL).
// This spread is to avoid overloading the full node with too many simultaneous requests.
//
// Reference: https://github.com/viccon/sturdyc?tab=readme-ov-file#early-refreshes
func getCacheDelays(ttl time.Duration) (min, max time.Duration) {
	minFloat := float64(ttl) * minEarlyRefreshPercentage
	maxFloat := float64(ttl) * maxEarlyRefreshPercentage

	// Round to the nearest second
	min = time.Duration(minFloat/float64(time.Second)+0.5) * time.Second
	max = time.Duration(maxFloat/float64(time.Second)+0.5) * time.Second
	return
}

// Use cache prefixes to avoid collisions with other cache keys.
// This is a simple way to namespace the cache keys.
const (
	sessionCacheKeyPrefix       = "session"
	accountPubKeyCacheKeyPrefix = "pubkey"
)

var _ sdk.FullNode = &cachingFullNode{}

// cachingFullNode implements the FullNode interface by wrapping a LazyFullNode
// and caching results to improve performance with automatic refresh-ahead.
//
// Early Refresh Strategy:
// Uses SturdyC's early refresh to prevent thundering herd and eliminate latency spikes.
// Background refreshes happen before entries expire, so GetApp/GetSession never block.
//
// Example times (values may change):
//   - 30s TTL, refresh at 22.5-27s (75-90% of TTL)
//
// Benefits: Zero-latency reads for active traffic, thundering herd protection,
// automatic load balancing, and graceful degradation.
//
// Docs reference: https://github.com/viccon/sturdyc
type cachingFullNode struct {
	logger polylog.Logger

	// Use a LazyFullNode as the underlying node
	// for fetching data from the protocol.
	lazyFullNode *LazyFullNode

	// As of #275, on Beta TestNet, sessions are 5 minutes.
	//
	// TODO_MAINNET_MIGRATION(@Olshansk): Revisit these values after mainnet migration to ensure no race conditions.
	sessionCache *sturdyc.Client[sessiontypes.Session]

	// The account public key cache.
	accountPubKeyCache *sturdyc.Client[cryptotypes.PubKey]
}

// NewCachingFullNode creates a new CachingFullNode that wraps a LazyFullNode with caching layers.
//
// The caching layers are:
//   - Session cache: Gets sessions from the cache or calls the lazyFullNode.GetSession()
//     (Apps are sourced from the Session struct, so no need to cache them.)
//   - Account cache: Used in the `cachingPoktNodeAccountFetcher` to cache account data indefinitely.
//
// The caching layers are configured with early refreshes to prevent thundering herd and eliminate latency spikes.
func NewCachingFullNode(
	logger polylog.Logger,
	lazyFullNode *LazyFullNode,
	cacheConfig CacheConfig,
) (*cachingFullNode, error) {
	// Set default session TTL if not set
	cacheConfig.hydrateDefaults()

	// Log cache configuration
	logger.Debug().
		Str("cache_config_session_ttl", cacheConfig.SessionTTL.String()).
		Msgf("cachingFullNode - Cache Configuration")

	// Configure session cache with early refreshes
	sessionMinRefreshDelay, sessionMaxRefreshDelay := getCacheDelays(cacheConfig.SessionTTL)

	// Create the session cache with early refreshes
	sessionCache := sturdyc.New[sessiontypes.Session](
		cacheCapacity,
		numShards,
		cacheConfig.SessionTTL,
		evictionPercentage,
		// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#early-refreshes
		sturdyc.WithEarlyRefreshes(
			sessionMinRefreshDelay,
			sessionMaxRefreshDelay,
			cacheConfig.SessionTTL,
			retryBaseDelay,
		),
	)

	// Create the account cache, which is used to cache account responses from the full node.
	// This cache is effectively infinite caching for the lifetime of the application,
	// so no need to configure early refreshes with SturdyC.
	accountPubKeyCache := sturdyc.New[cryptotypes.PubKey](
		cacheCapacity,
		numShards,
		pubKeyCacheTTL,
		evictionPercentage,
	)

	// Initialize the caching full node with the modified lazy full node
	return &cachingFullNode{
		logger:             logger,
		lazyFullNode:       lazyFullNode,
		sessionCache:       sessionCache,
		accountPubKeyCache: accountPubKeyCache,
	}, nil
}

// GetApp is a NoOp in the caching full node.
// Apps are fetched on startup from the remote full node using the LazyFullNode.
// During relaying, only sessions are fetched to ensure apps and sessions are always in sync.
func (cfn *cachingFullNode) GetApp(ctx context.Context, appAddr string) (*apptypes.Application, error) {
	return nil, fmt.Errorf("GetApp is a NoOp in the caching full node")
}

// GetSession returns the session for the given service and app, using a cached version if available.
// The cache will automatically refresh the session in the background before it expires.
func (cfn *cachingFullNode) GetSession(
	ctx context.Context,
	serviceID sdk.ServiceID,
	appAddr string,
) (sessiontypes.Session, error) {
	// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#get-or-fetch
	return cfn.sessionCache.GetOrFetch(
		ctx,
		getSessionCacheKey(serviceID, appAddr),
		func(fetchCtx context.Context) (sessiontypes.Session, error) {
			cfn.logger.Debug().Str("session_key", getSessionCacheKey(serviceID, appAddr)).Msgf(
				"[cachingFullNode.GetSession] Making request to full node",
			)
			return cfn.lazyFullNode.GetSession(fetchCtx, serviceID, appAddr)
		},
	)
}

// getSessionCacheKey returns the cache key for the given service and app address.
// It uses the sessionCacheKeyPrefix, service ID, and app address to create a unique key.
//
// eg. "session:eth:pokt1up7zlytnmvlsuxzpzvlrta95347w322adsxslw"
func getSessionCacheKey(serviceID sdk.ServiceID, appAddr string) string {
	return fmt.Sprintf("%s:%s:%s", sessionCacheKeyPrefix, serviceID, appAddr)
}

// GetAccountPubKey returns the account public key for the given address.
// The cache has no TTL, so the public key is cached indefinitely.
func (cfn *cachingFullNode) GetAccountPubKey(
	ctx context.Context,
	address string,
) (pubKey cryptotypes.PubKey, err error) {
	// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#get-or-fetch
	return cfn.accountPubKeyCache.GetOrFetch(
		ctx,
		getAccountPubKeyCacheKey(address),
		func(fetchCtx context.Context) (cryptotypes.PubKey, error) {
			cfn.logger.Debug().Str("account_key", getAccountPubKeyCacheKey(address)).Msgf(
				"[cachingFullNode.GetPubKeyFromAddress] Making request to full node",
			)
			return cfn.lazyFullNode.GetAccountPubKey(fetchCtx, address)
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

// ValidateRelayResponse delegates to the underlying node.
func (cfn *cachingFullNode) ValidateRelayResponse(
	supplierAddr sdk.SupplierAddress,
	responseBz []byte,
) (*servicetypes.RelayResponse, error) {
	return cfn.lazyFullNode.ValidateRelayResponse(supplierAddr, responseBz)
}

// IsHealthy delegates to the underlying node.
//
// TODO_IMPROVE(@commoddity):
//   - Implement a more sophisticated health check
//   - Check for the presence of cached apps and sessions (when the TODO_IMPROVE at the top of this file is addressed)
//   - For now, always returns true because the cache is populated incrementally as new apps and sessions are requested.
func (cfn *cachingFullNode) IsHealthy() bool {
	return cfn.lazyFullNode.IsHealthy()
}
