package client

import (
	"context"
	"fmt"
	"math"
	"time"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	accounttypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/pokt-network/poktroll/pkg/polylog"
	apptypes "github.com/pokt-network/poktroll/x/application/types"
	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sdkTypes "github.com/pokt-network/shannon-sdk/types"
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

// Prefix for cache keys to avoid collisions with other keys.
const (
	sessionCacheKeyPrefix       = "session"
	accountPubKeyCacheKeyPrefix = "pubkey"
)

// GatewayClientCache provides a caching layer for the gateway client.
// It is the primary fetching/caching layer that retrieves data from a Shannon full node and caches it using SturdyC.
//
//   - Early refresh: background updates before expiry (prevents thundering herd/latency spikes)
//   - Example: 30s TTL, refresh at 22.5–27s (75–90%)
//   - Benefits: zero-latency reads, graceful degradation, auto load balancing
//
// Docs: https://github.com/viccon/sturdyc
type GatewayClientCache struct {
	logger polylog.Logger

	// Session cache
	// TODO_MAINNET_MIGRATION(@Olshansk): Revisit after mainnet
	// TODO_NEXT(@commoddity): Session refresh handling should be significantly reworked as part of the next changes following PATH PR #297.
	//   The proposed change is to align session refreshes with actual session expiry time,
	//   using the session expiry block and the Shannon SDK's block client.
	//   When this is done, session cache TTL can be removed altogether.
	sessionCache *sturdyc.Client[sessiontypes.Session]

	// The account public key cache; used to cache account public keys indefinitely.
	// It has an infinite TTL and is populated only once on startup.
	accountPubKeyCache *sturdyc.Client[cryptotypes.PubKey]

	// The SDK clients are used by the GatewayClientCache to fetch onchain data from a
	// Shannon full node and store it in the SturdyC cache.

	appClient     *sdk.ApplicationClient
	sessionClient *sdk.SessionClient
	accountClient *sdk.AccountClient
	blockClient   *sdk.BlockClient
}

// NewGatewayClientCache connects to a Shannon full node and creates a GatewayClientCache.
// It uses the full node's RPC URL and gRPC configuration to connect to the full node.
// It creates the SDK clients and SturdyC cache to provide the primary fetching/caching layer.
//
//   - Session cache: refreshes early to avoid thundering herd/latency spikes
//   - Account public key cache: indefinite cache for account data
//   - Application client: used by GatewayClientCache to fetch applications from the full node
//   - Session client: used by GatewayClientCache to fetch sessions from the full node
//   - Account client: used by GatewayClientCache to fetch accounts from the full node
func NewGatewayClientCache(
	logger polylog.Logger,
	rpcURL string,
	fullNodeConfig FullNodeConfig,
) (*GatewayClientCache, error) {
	fullNodeConfig.CacheConfig.hydrateDefaults()

	// Connect to the full node
	grpcConn, err := connectGRPC(
		fullNodeConfig.GRPCConfig.HostPort,
		fullNodeConfig.GRPCConfig.UseInsecureGRPCConn,
	)
	if err != nil {
		return nil, fmt.Errorf("NewGatewayClientCache: error creating new GRPC connection at url %s: %w",
			fullNodeConfig.GRPCConfig.HostPort, err)
	}

	// Create the block client
	blockClient, err := newBlockClient(rpcURL)
	if err != nil {
		return nil, fmt.Errorf("NewGatewayClientCache: error creating new Shannon block client at URL %s: %w", rpcURL, err)
	}

	// Create the session cache with early refreshes
	sessionCache := getCache[sessiontypes.Session](
		fullNodeConfig.CacheConfig.SessionTTL,
		*fullNodeConfig.CacheConfig.EarlyRefreshEnabled,
	)

	// Create the account cache, which is effectively infinite
	// caching for the lifetime of the application.
	accountPubKeyCache := getCache[cryptotypes.PubKey](
		pubKeyCacheTTL,
		false, // Never refresh the account public key cache
	)

	return &GatewayClientCache{
		logger: logger,

		sessionCache:       sessionCache,
		accountPubKeyCache: accountPubKeyCache,

		sessionClient: newSessionClient(grpcConn),
		appClient:     newAppClient(grpcConn),
		accountClient: newAccClient(grpcConn),
		blockClient:   blockClient,
	}, nil
}

// getCache creates a SturdyC cache with the given TTL and early refresh configuration.
//
// If early refresh is enabled, SturdyC will refresh the cache before it
// expires to ensure that the cache is always hot and never blocking.
//
// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#early-refreshes
func getCache[T any](ttl time.Duration, earlyRefreshEnabled bool) *sturdyc.Client[T] {
	if earlyRefreshEnabled {
		// Configure session cache with early refreshes
		minRefreshDelay, maxRefreshDelay := getCacheDelays(ttl)

		// Create the session cache with early refreshes
		// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#early-refreshes
		return sturdyc.New[T](
			cacheCapacity,
			numShards,
			ttl,
			evictionPercentage,
			sturdyc.WithEarlyRefreshes(
				minRefreshDelay,
				maxRefreshDelay,
				ttl,
				retryBaseDelay,
			),
		)
	} else {
		// Create the session cache without early refreshes
		return sturdyc.New[T](
			cacheCapacity,
			numShards,
			ttl,
			evictionPercentage,
		)
	}
}

// getCacheDelays returns the min/max delays for SturdyC's Early Refresh strategy.
//   - Proactively refreshes cache before expiry (prevents misses/latency spikes)
//   - Refresh window: 75-90% of TTL (e.g. 22.5-27s for 30s TTL)
//   - Spreads requests to avoid thundering herd
//
// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#early-refreshes
func getCacheDelays(ttl time.Duration) (min, max time.Duration) {
	minFloat := float64(ttl) * minEarlyRefreshPercentage
	maxFloat := float64(ttl) * maxEarlyRefreshPercentage

	// Round to the nearest second
	min = time.Duration(minFloat/float64(time.Second)+0.5) * time.Second
	max = time.Duration(maxFloat/float64(time.Second)+0.5) * time.Second
	return
}

// GetApp passthrough to the underlying full node to ensure the request is always a remote request to the full node.
// Apps are fetched only at startup by the GatewayClientCache; relaying fetches sessions for app/session sync).
func (gcc *GatewayClientCache) GetApp(ctx context.Context, appAddr string) (apptypes.Application, error) {
	return gcc.appClient.GetApplication(ctx, appAddr)
}

// GetSession returns (and auto-refreshes) the session for a service/app from cache.
func (gcc *GatewayClientCache) GetSession(
	ctx context.Context,
	serviceID sdk.ServiceID,
	appAddr string,
) (sessiontypes.Session, error) {
	// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#get-or-fetch
	return gcc.sessionCache.GetOrFetch(
		ctx,
		getSessionCacheKey(serviceID, appAddr),
		func(fetchCtx context.Context) (sessiontypes.Session, error) {
			gcc.logger.Debug().
				Str("session_key", getSessionCacheKey(serviceID, appAddr)).
				Msgf("GetSession: GatewayClientCache making request to full node for service %s", serviceID)
			return gcc.getSessionFromFullNode(fetchCtx, serviceID, appAddr)
		},
	)
}

// getSessionCacheKey builds a unique cache key for session: <prefix>:<serviceID>:<appAddr>
func getSessionCacheKey(serviceID sdk.ServiceID, appAddr string) string {
	return fmt.Sprintf("%s:%s:%s", sessionCacheKeyPrefix, serviceID, appAddr)
}

// getSessionFromFullNode:
// - Uses the GatewayClientCache's session client to fetch a session for the (serviceID, appAddr) combination.
func (gcc *GatewayClientCache) getSessionFromFullNode(
	ctx context.Context,
	serviceID sdk.ServiceID,
	appAddr string,
) (sessiontypes.Session, error) {
	session, err := gcc.sessionClient.GetSession(
		ctx,
		appAddr,
		string(serviceID),
		0,
	)
	if err != nil {
		return sessiontypes.Session{},
			fmt.Errorf("GetSession: error getting the session for service %s app %s: %w",
				serviceID, appAddr, err,
			)
	}
	if session == nil {
		return sessiontypes.Session{},
			fmt.Errorf("GetSession: got nil session for service %s app %s: %w",
				serviceID, appAddr, err,
			)
	}

	gcc.logger.Debug().Msgf("GetSession: fetched session %s", session)

	return *session, nil
}

// GetAccountPubKey returns the account public key for the given address.
// The account public key cache has no TTL, so the public key is cached indefinitely.
//
// The `fetchFn` param of `GetOrFetch` is only called once per address on startup.
func (gcc *GatewayClientCache) GetAccountPubKey(
	ctx context.Context,
	address string,
) (pubKey cryptotypes.PubKey, err error) {
	// See: https://github.com/viccon/sturdyc?tab=readme-ov-file#get-or-fetch
	return gcc.accountPubKeyCache.GetOrFetch(
		ctx,
		getAccountPubKeyCacheKey(address),
		func(fetchCtx context.Context) (cryptotypes.PubKey, error) {
			gcc.logger.Debug().
				Str("account_key", getAccountPubKeyCacheKey(address)).
				Msgf("GetAccountPubKey: GatewayClientCache making request to full node")
			return gcc.getAccountPubKeyFromFullNode(fetchCtx, address)
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

// getAccountPubKeyFromFullNode returns the public key of the account with the given address.
//
// - Uses the GatewayClientCache's account client to query the account module using the gRPC query client.
func (gcc *GatewayClientCache) getAccountPubKeyFromFullNode(
	ctx context.Context,
	address string,
) (pubKey cryptotypes.PubKey, err error) {
	req := &accounttypes.QueryAccountRequest{Address: address}

	res, err := gcc.accountClient.Account(ctx, req)
	if err != nil {
		return nil, err
	}

	var fetchedAccount types.AccountI
	if err = sdkTypes.QueryCodec.UnpackAny(res.Account, &fetchedAccount); err != nil {
		return nil, err
	}

	return fetchedAccount.GetPubKey(), nil
}

// IsHealthy satisfies the interface required by the ShannonFullNode interface.
// TODO_IMPROVE(@commoddity):
//   - Add smarter health checks (e.g. verify cached apps/sessions)
//   - Currently always true (cache fills as needed)
func (gcc *GatewayClientCache) IsHealthy() bool {
	return true
}
