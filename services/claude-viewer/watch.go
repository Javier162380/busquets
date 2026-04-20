package claudeviewer

import (
	"context"
	"sync"
	"time"
)

// WatchResult represents the result of a background sync operation.
type WatchResult struct {
	Count int   // Number of plans synced
	Error error // Error if sync failed
}

// WatchManager manages background plan synchronization.
type WatchManager struct {
	service    *Service
	mu         sync.RWMutex
	running    bool
	cancel     context.CancelFunc
	resultChan chan WatchResult
	interval   time.Duration
}

// NewWatchManager creates a new watch manager.
func NewWatchManager(service *Service) *WatchManager {
	return &WatchManager{
		service:    service,
		resultChan: make(chan WatchResult, 10), // buffered to prevent blocking
	}
}

// Start begins watching for plan changes at the configured interval.
func (wm *WatchManager) Start(ctx context.Context, interval time.Duration) error {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if wm.running {
		return nil // Already running
	}

	wm.interval = interval
	ctx, cancel := context.WithCancel(ctx)
	wm.cancel = cancel
	wm.running = true

	go wm.watchLoop(ctx)

	return nil
}

// Stop stops the watch manager.
func (wm *WatchManager) Stop() {
	wm.mu.Lock()
	defer wm.mu.Unlock()

	if !wm.running {
		return
	}

	if wm.cancel != nil {
		wm.cancel()
	}
	wm.running = false
}

// IsRunning returns true if watch mode is currently active.
func (wm *WatchManager) IsRunning() bool {
	wm.mu.RLock()
	defer wm.mu.RUnlock()
	return wm.running
}

// ResultChannel returns the channel for receiving watch results.
func (wm *WatchManager) ResultChannel() <-chan WatchResult {
	return wm.resultChan
}

// UpdateInterval changes the sync interval (takes effect on next tick).
func (wm *WatchManager) UpdateInterval(interval time.Duration) {
	wm.mu.Lock()
	defer wm.mu.Unlock()
	wm.interval = interval
}

// watchLoop runs the periodic sync in a goroutine.
func (wm *WatchManager) watchLoop(ctx context.Context) {
	ticker := time.NewTicker(wm.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Check if interval changed
			wm.mu.RLock()
			currentInterval := wm.interval
			wm.mu.RUnlock()

			// Reset ticker if interval changed
			if currentInterval != wm.interval {
				ticker.Reset(currentInterval)
			}

			// Perform sync
			count, err := wm.service.SyncPlans(ctx)

			// Send result (non-blocking with buffered channel)
			select {
			case wm.resultChan <- WatchResult{Count: count, Error: err}:
			default:
				// Channel full, skip this result
			}
		}
	}
}
