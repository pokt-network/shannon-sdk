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
type cachingFullNode struct {
	logger polylog.Logger

	// Underlying node for protocol data fetches
	lazyFullNode *LazyFullNode

	// Session cache (5 min on Beta TestNet, see #275)
	// TODO_MAINNET_MIGRATION(@Olshansk): Revisit after mainnet
	sessionCache *sturdyc.Client[sessiontypes.Session]

	// The account public key cache; used to cache account public keys indefinitely.
	// It has an infinite TTL and is populated only once on startup.
	accountPubKeyCache *sturdyc.Client[cryptotypes.PubKey]
}

// NewCachingFullNode wraps a LazyFullNode with:
//   - Session cache: refreshes early to avoid thundering herd/latency spikes
//   - Account public key cache: indefinite cache for account data
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

	// Create the account cache, which is effectively infinite
	// caching for the lifetime of the application.
	// No need to configure early refreshes with SturdyC.
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

// GetApp is a NoOp (apps fetched only at startup; relaying fetches sessions for app/session sync).
func (cfn *cachingFullNode) GetApp(ctx context.Context, appAddr string) (*apptypes.Application, error) {
	return nil, fmt.Errorf("GetApp is a NoOp in the caching full node")
}

// GetSession returns (and auto-refreshes) the session for a service/app from cache.
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

// getSessionCacheKey builds a unique cache key for session: <prefix>:<serviceID>:<appAddr>
func getSessionCacheKey(serviceID sdk.ServiceID, appAddr string) string {
	return fmt.Sprintf("%s:%s:%s", sessionCacheKeyPrefix, serviceID, appAddr)
}

// GetAccountPubKey returns the account public key for the given address.
// The cache has no TTL, so the public key is cached indefinitely.
//
// The `fetchFn` param of `GetOrFetch` is only called once per address on startup.
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

// ValidateRelayResponse: passthrough to underlying node.
func (cfn *cachingFullNode) ValidateRelayResponse(
	ctx context.Context,
	supplierAddr sdk.SupplierAddress,
	responseBz []byte,
) (*servicetypes.RelayResponse, error) {
	return cfn.lazyFullNode.ValidateRelayResponse(ctx, supplierAddr, responseBz)
}

// IsHealthy: passthrough to underlying node.
// TODO_IMPROVE(@commoddity):
//   - Add smarter health checks (e.g. verify cached apps/sessions)
//   - Currently always true (cache fills as needed)
func (cfn *cachingFullNode) IsHealthy() bool {
	return cfn.lazyFullNode.IsHealthy()
}
