package client

// import (
// 	"context"
// 	"sync"
// 	"time"

// 	"github.com/pokt-network/poktroll/pkg/polylog"
// 	sessiontypes "github.com/pokt-network/poktroll/x/session/types"
// )

// // sessionRefreshMonitor handles intelligent block-based session refresh monitoring
// type sessionRefreshMonitor struct {
// 	logger             polylog.Logger
// 	onchainDataFetcher OnchainDataFetcher

// 	// Session refresh callback - called when all sessions need to be refreshed
// 	refreshCallback func()

// 	// Current session end height being monitored
// 	currentSessionEndHeight int64
// 	sessionEndHeightMu      sync.RWMutex

// 	// Block monitoring control
// 	blockMonitorCtx    context.Context
// 	blockMonitorCancel context.CancelFunc
// 	blockMonitorMu     sync.Mutex
// 	isMonitoring       bool
// }

// // newSessionRefreshMonitor creates a new session refresh monitor
// func newSessionRefreshMonitor(
// 	logger polylog.Logger,
// 	onchainDataFetcher OnchainDataFetcher,
// 	refreshCallback func(),
// ) *sessionRefreshMonitor {
// 	blockMonitorCtx, blockMonitorCancel := context.WithCancel(context.Background())

// 	return &sessionRefreshMonitor{
// 		logger:                  logger.With("component", "session_refresh_monitor"),
// 		onchainDataFetcher:      onchainDataFetcher,
// 		refreshCallback:         refreshCallback,
// 		currentSessionEndHeight: 0,
// 		blockMonitorCtx:         blockMonitorCtx,
// 		blockMonitorCancel:      blockMonitorCancel,
// 	}
// }

// // start begins the background block monitoring goroutine
// func (srm *sessionRefreshMonitor) start() {
// 	go srm.startBlockMonitoring()
// }

// // stop stops the block monitoring goroutine
// func (srm *sessionRefreshMonitor) stop() {
// 	srm.logger.Info().Msg("Stopping session refresh monitor")
// 	if srm.blockMonitorCancel != nil {
// 		srm.blockMonitorCancel()
// 	}
// }

// // updateSessionEndHeight updates the global session end height from a fetched session
// func (srm *sessionRefreshMonitor) updateSessionEndHeight(session sessiontypes.Session) {
// 	if session.Header == nil {
// 		srm.logger.Warn().
// 			Msg("Session header is nil, cannot update session end height")
// 		return
// 	}

// 	sessionEndHeight := session.Header.SessionEndBlockHeight

// 	srm.sessionEndHeightMu.Lock()
// 	srm.currentSessionEndHeight = sessionEndHeight
// 	srm.sessionEndHeightMu.Unlock()

// 	srm.logger.Debug().
// 		Int64("session_end_height", sessionEndHeight).
// 		Msg("Updated global session end height for monitoring")
// }

// // startBlockMonitoring starts the background block monitoring goroutine
// func (srm *sessionRefreshMonitor) startBlockMonitoring() {
// 	srm.logger.Info().Msg("Starting block monitoring for intelligent session refresh")

// 	for {
// 		select {
// 		case <-srm.blockMonitorCtx.Done():
// 			srm.logger.Info().Msg("Block monitoring stopped")
// 			return
// 		case <-time.After(blockTime / 2): // Check every half block time initially
// 			srm.checkAndHandleSessionRefresh()
// 		}
// 	}
// }

// // checkAndHandleSessionRefresh checks if we need to start intensive polling or refresh sessions
// func (srm *sessionRefreshMonitor) checkAndHandleSessionRefresh() {
// 	srm.sessionEndHeightMu.RLock()
// 	targetHeight := srm.currentSessionEndHeight
// 	srm.sessionEndHeightMu.RUnlock()

// 	if targetHeight == 0 {
// 		// No sessions to monitor yet
// 		return
// 	}

// 	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
// 	defer cancel()

// 	currentHeight, err := srm.onchainDataFetcher.LatestBlockHeight(ctx)
// 	if err != nil {
// 		srm.logger.Error().
// 			Err(err).
// 			Msg("Failed to get current block height for session monitoring")
// 		return
// 	}

// 	srm.logger.Debug().
// 		Int64("current_height", currentHeight).
// 		Int64("target_height", targetHeight).
// 		Msg("Checking session end proximity")

// 	// If we're at or past the session end height, refresh all sessions immediately
// 	if currentHeight >= targetHeight {
// 		srm.logger.Info().
// 			Int64("current_height", currentHeight).
// 			Int64("session_end_height", targetHeight).
// 			Msg("Session end height reached, refreshing all sessions")
// 		srm.refreshAllSessions()
// 		srm.resetSessionMonitoring()
// 		return
// 	}

// 	// If we're at the block before session end, start intensive polling
// 	if currentHeight == targetHeight-1 {
// 		srm.logger.Info().
// 			Int64("current_height", currentHeight).
// 			Int64("session_end_height", targetHeight).
// 			Msg("Block before session end detected, starting intensive polling")
// 		srm.startIntensivePolling(targetHeight)
// 	}
// }

// // startIntensivePolling starts intensive polling when approaching session end
// func (srm *sessionRefreshMonitor) startIntensivePolling(targetHeight int64) {
// 	srm.blockMonitorMu.Lock()
// 	if srm.isMonitoring {
// 		srm.blockMonitorMu.Unlock()
// 		return // Already in intensive monitoring mode
// 	}
// 	srm.isMonitoring = true
// 	srm.blockMonitorMu.Unlock()

// 	srm.logger.Info().
// 		Int64("target_height", targetHeight).
// 		Msg("Starting intensive block height polling")

// 	ticker := time.NewTicker(blockPollingInterval)
// 	defer ticker.Stop()

// 	for {
// 		select {
// 		case <-srm.blockMonitorCtx.Done():
// 			return
// 		case <-ticker.C:
// 			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 			currentHeight, err := srm.onchainDataFetcher.LatestBlockHeight(ctx)
// 			cancel()

// 			if err != nil {
// 				srm.logger.Error().
// 					Err(err).
// 					Msg("Failed to get current block height during intensive polling")
// 				continue
// 			}

// 			srm.logger.Debug().
// 				Int64("current_height", currentHeight).
// 				Int64("target_height", targetHeight).
// 				Msg("Intensive polling check")

// 			if currentHeight >= targetHeight {
// 				srm.logger.Info().
// 					Int64("current_height", currentHeight).
// 					Int64("session_end_height", targetHeight).
// 					Msg("Session end height reached during intensive polling, refreshing all sessions")

// 				srm.refreshAllSessions()
// 				srm.resetSessionMonitoring()
// 				return
// 			}
// 		}
// 	}
// }

// // refreshAllSessions triggers the session refresh callback
// func (srm *sessionRefreshMonitor) refreshAllSessions() {
// 	srm.logger.Info().
// 		Int64("session_end_height", srm.currentSessionEndHeight).
// 		Msg("Triggering session refresh callback")

// 	if srm.refreshCallback != nil {
// 		srm.refreshCallback()
// 	}
// }

// // resetSessionMonitoring resets the monitoring state for the next session cycle
// func (srm *sessionRefreshMonitor) resetSessionMonitoring() {
// 	srm.blockMonitorMu.Lock()
// 	srm.isMonitoring = false
// 	srm.blockMonitorMu.Unlock()

// 	srm.sessionEndHeightMu.Lock()
// 	srm.currentSessionEndHeight = 0
// 	srm.sessionEndHeightMu.Unlock()

// 	srm.logger.Info().Msg("Reset session monitoring for next session cycle")
// }
