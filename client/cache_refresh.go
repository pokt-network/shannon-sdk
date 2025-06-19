package client

import (
	"context"
	"sync"
	"time"

	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
	sdk "github.com/pokt-network/shannon-sdk"
	"github.com/viccon/sturdyc"
)

// sessionKeyInfo holds metadata for active sessions during background refresh
type sessionKeyInfo struct {
	serviceID sdk.ServiceID
	appAddr   string
}

// sessionRefreshState holds the state for block-based session monitoring
//
// Session refresh lifecycle:
//  1. Normal monitoring: Check every 15 seconds
//  2. Intensive polling: 1-second checks starting at SessionEndBlockHeight
//  3. Cache refresh: Triggered at SessionEndBlockHeight+1
//  4. Background refresh: New cache populated while old cache serves requests
//  5. Atomic swap: New cache replaces old cache with zero downtime
//
// Documentation: https://github.com/viccon/sturdyc
type sessionRefreshState struct {
	currentSessionEndHeight int64
	sessionEndHeightMu      sync.RWMutex

	blockMonitorMu sync.Mutex
	isMonitoring   bool

	// Active session tracking for background refresh
	activeSessionKeys map[string]sessionKeyInfo
	activeSessionMu   sync.RWMutex
}

// ============================================================================
// Session Monitoring Lifecycle
// ============================================================================

// startSessionMonitoring begins background block monitoring
func (gcc *GatewayClientCache) startSessionMonitoring() {
	gcc.logger.Debug().
		Dur("check_interval", blockTime/2).
		Msg("Starting session monitoring background process")

	go gcc.monitorBlockHeights()
}

// updateSessionEndHeight updates the global session end height from a fetched session
func (gcc *GatewayClientCache) updateSessionEndHeight(session sessiontypes.Session) {
	if session.Header == nil {
		gcc.logger.Warn().Msg("Session header is nil, cannot update session end height")
		return
	}

	sessionEndHeight := session.Header.SessionEndBlockHeight

	gcc.sessionRefreshState.sessionEndHeightMu.Lock()
	previousHeight := gcc.sessionRefreshState.currentSessionEndHeight
	gcc.sessionRefreshState.currentSessionEndHeight = sessionEndHeight
	gcc.sessionRefreshState.sessionEndHeightMu.Unlock()

	if previousHeight != sessionEndHeight {
		gcc.logger.Debug().
			Int64("previous_session_end_height", previousHeight).
			Int64("new_session_end_height", sessionEndHeight).
			Msg("Updated session end height for monitoring")
	}
}

// ============================================================================
// Background Monitoring Logic
// ============================================================================

// monitorBlockHeights runs the main monitoring loop that checks every 15 seconds
func (gcc *GatewayClientCache) monitorBlockHeights() {
	checkCount := 0
	for {
		time.Sleep(blockTime / 2) // Check every 15 seconds
		checkCount++

		gcc.logger.Debug().
			Int("check_count", checkCount).
			Msg("Background monitoring check")

		gcc.checkAndHandleSessionRefresh()
	}
}

// checkAndHandleSessionRefresh determines if session refresh is needed and takes action
func (gcc *GatewayClientCache) checkAndHandleSessionRefresh() {
	targetHeight := gcc.getCurrentSessionEndHeight()
	if targetHeight == 0 {
		gcc.logger.Debug().Msg("No sessions to monitor yet")
		return
	}

	currentHeight, err := gcc.getCurrentBlockHeight()
	if err != nil {
		gcc.logger.Error().
			Err(err).
			Int64("session_end_height", targetHeight).
			Msg("Failed to get current block height")
		return
	}

	gcc.logger.Debug().
		Int64("current_height", currentHeight).
		Int64("session_end_height", targetHeight).
		Int64("blocks_until_session_end", targetHeight-currentHeight).
		Msg("Checking session end proximity")

	// Refresh immediately if we're past SessionEndBlockHeight + 1
	if currentHeight >= targetHeight+1 {
		gcc.logger.Debug().
			Int64("current_height", currentHeight).
			Int64("session_end_height", targetHeight).
			Msg("SessionEndBlockHeight+1 reached, refreshing sessions")

		gcc.refreshAllSessions()
		gcc.resetMonitoring()
		return
	}

	// Start intensive polling if we've reached SessionEndBlockHeight
	if currentHeight >= targetHeight && gcc.tryStartIntensiveMonitoring() {
		gcc.logger.Debug().
			Int64("session_end_height", targetHeight).
			Msg("Starting intensive polling for SessionEndBlockHeight+1")

		gcc.runIntensivePolling(targetHeight)
	}
}

// tryStartIntensiveMonitoring attempts to start intensive monitoring (prevents duplicates)
func (gcc *GatewayClientCache) tryStartIntensiveMonitoring() bool {
	gcc.sessionRefreshState.blockMonitorMu.Lock()
	defer gcc.sessionRefreshState.blockMonitorMu.Unlock()

	if gcc.sessionRefreshState.isMonitoring {
		return false // Already monitoring intensively
	}

	gcc.sessionRefreshState.isMonitoring = true
	return true
}

// runIntensivePolling performs 1-second polling until SessionEndBlockHeight+1 is reached
func (gcc *GatewayClientCache) runIntensivePolling(targetHeight int64) {
	ticker := time.NewTicker(blockPollingInterval)
	defer ticker.Stop()

	pollCount := 0
	for {
		<-ticker.C
		pollCount++

		if gcc.shouldRefreshNow(targetHeight, pollCount) {
			gcc.refreshAllSessions()
			gcc.resetMonitoring()
			return
		}
	}
}

// shouldRefreshNow checks if we've reached SessionEndBlockHeight+1 during intensive polling
func (gcc *GatewayClientCache) shouldRefreshNow(targetHeight int64, pollCount int) bool {
	currentHeight, err := gcc.getCurrentBlockHeight()
	if err != nil {
		gcc.logger.Error().
			Err(err).
			Int64("session_end_height", targetHeight).
			Int("poll_count", pollCount).
			Msg("Failed to get current block height during intensive polling")
		return false
	}

	gcc.logger.Debug().
		Int64("current_height", currentHeight).
		Int64("target_refresh_height", targetHeight+1).
		Int64("blocks_until_refresh", (targetHeight+1)-currentHeight).
		Int("poll_count", pollCount).
		Msg("Intensive polling check")

	if currentHeight >= targetHeight+1 {
		gcc.logger.Debug().
			Int64("current_height", currentHeight).
			Int64("session_end_height", targetHeight).
			Int("total_polls", pollCount).
			Msg("SessionEndBlockHeight+1 reached, refreshing sessions")
		return true
	}

	return false
}

// refreshAllSessions triggers background refresh of all active sessions
func (gcc *GatewayClientCache) refreshAllSessions() {
	activeKeys := gcc.getActiveSessionKeys()
	if len(activeKeys) == 0 {
		gcc.logger.Debug().Msg("No active sessions to refresh")
		return
	}

	gcc.logger.Info().
		Int("session_count", len(activeKeys)).
		Msg("Starting background session refresh")

	gcc.refreshSessionsInBackground(activeKeys)
}

// resetMonitoring resets monitoring state for the next session cycle
func (gcc *GatewayClientCache) resetMonitoring() {
	gcc.sessionRefreshState.blockMonitorMu.Lock()
	gcc.sessionRefreshState.isMonitoring = false
	gcc.sessionRefreshState.blockMonitorMu.Unlock()

	gcc.sessionRefreshState.sessionEndHeightMu.Lock()
	gcc.sessionRefreshState.currentSessionEndHeight = 0
	gcc.sessionRefreshState.sessionEndHeightMu.Unlock()

	gcc.logger.Debug().Msg("Reset session monitoring for next cycle")
}

// ============================================================================
// Session Key Tracking - For background refresh
// ============================================================================

// trackActiveSession records a session key for background refresh tracking
func (gcc *GatewayClientCache) trackActiveSession(sessionKey string, serviceID sdk.ServiceID, appAddr string) {
	gcc.sessionRefreshState.activeSessionMu.Lock()
	defer gcc.sessionRefreshState.activeSessionMu.Unlock()

	gcc.sessionRefreshState.activeSessionKeys[sessionKey] = sessionKeyInfo{
		serviceID: serviceID,
		appAddr:   appAddr,
	}

	gcc.logger.Debug().
		Str("session_key", sessionKey).
		Msg("Tracking session for background refresh")
}

// getActiveSessionKeys returns a thread-safe copy of all active session keys
func (gcc *GatewayClientCache) getActiveSessionKeys() map[string]sessionKeyInfo {
	gcc.sessionRefreshState.activeSessionMu.RLock()
	defer gcc.sessionRefreshState.activeSessionMu.RUnlock()

	activeKeys := make(map[string]sessionKeyInfo, len(gcc.sessionRefreshState.activeSessionKeys))
	for key, info := range gcc.sessionRefreshState.activeSessionKeys {
		activeKeys[key] = info
	}
	return activeKeys
}

// refreshSessionsInBackground creates a new cache with fresh sessions and atomically swaps it
func (gcc *GatewayClientCache) refreshSessionsInBackground(activeKeys map[string]sessionKeyInfo) {
	go func() {
		gcc.logger.Debug().
			Int("session_count", len(activeKeys)).
			Msg("Creating new cache with fresh sessions")

		// Create a new empty cache to populate with fresh sessions
		newCache := getCache[sessiontypes.Session]()

		// Populate the new cache with fresh sessions based on the active keys
		successCount, errorCount := gcc.populateNewCache(newCache, activeKeys)

		// Atomically swap to the new cache
		gcc.sessionCacheMu.Lock()
		gcc.sessionCache = newCache
		gcc.sessionCacheMu.Unlock()

		gcc.logger.Info().
			Int("total_sessions", len(activeKeys)).
			Int("successful_refreshes", successCount).
			Int("failed_refreshes", errorCount).
			Msg("Background session refresh completed")
	}()
}

// populateNewCache fetches all sessions concurrently and populates the new cache
func (gcc *GatewayClientCache) populateNewCache(newCache *sturdyc.Client[sessiontypes.Session], activeKeys map[string]sessionKeyInfo) (int, int) {
	var wg sync.WaitGroup
	var successCount, errorCount int
	var countMu sync.Mutex

	for sessionKey, keyInfo := range activeKeys {
		wg.Add(1)
		go func(key string, info sessionKeyInfo) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			_, err := newCache.GetOrFetch(ctx, key, func(fetchCtx context.Context) (sessiontypes.Session, error) {
				session, err := gcc.onchainDataFetcher.GetSession(fetchCtx, info.serviceID, info.appAddr)
				if err == nil {
					gcc.updateSessionEndHeight(session)
				}
				return session, err
			})

			countMu.Lock()
			if err != nil {
				errorCount++
				gcc.logger.Warn().Err(err).Str("session_key", key).Msg("Failed to refresh session")
			} else {
				successCount++
			}
			countMu.Unlock()
		}(sessionKey, keyInfo)
	}

	wg.Wait()
	return successCount, errorCount
}

// ============================================================================
// HELPER METHODS - Thread-safe getters and utilities
// ============================================================================

// getCurrentSessionEndHeight safely gets the current session end height
func (gcc *GatewayClientCache) getCurrentSessionEndHeight() int64 {
	gcc.sessionRefreshState.sessionEndHeightMu.RLock()
	defer gcc.sessionRefreshState.sessionEndHeightMu.RUnlock()
	return gcc.sessionRefreshState.currentSessionEndHeight
}

// getCurrentBlockHeight gets the current blockchain height with a 10-second timeout
func (gcc *GatewayClientCache) getCurrentBlockHeight() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return gcc.onchainDataFetcher.LatestBlockHeight(ctx)
}
