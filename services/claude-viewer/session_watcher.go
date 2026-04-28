package claudeviewer

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SessionWatchResult contains the result of a session watch operation.
type SessionWatchResult struct {
	SessionUUID string
	NewMessages []JSONLMessage
	Error       error
}

// SessionWatcher monitors a Claude session JSONL file for changes.
type SessionWatcher struct {
	sessionUUID   string
	filePath      string
	pollInterval  time.Duration
	lastLineCount int

	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup

	resultChan chan SessionWatchResult
	stopOnce   sync.Once
}

// NewSessionWatcher creates a new session watcher.
func NewSessionWatcher(sessionUUID, filePath string, pollInterval time.Duration) *SessionWatcher {
	if pollInterval == 0 {
		pollInterval = 500 * time.Millisecond // Default 500ms
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &SessionWatcher{
		sessionUUID:  sessionUUID,
		filePath:     filePath,
		pollInterval: pollInterval,
		ctx:          ctx,
		cancel:       cancel,
		resultChan:   make(chan SessionWatchResult, 10), // Buffered channel
	}
}

// Start begins watching the session file for changes.
// Returns a channel that receives new messages when detected.
func (sw *SessionWatcher) Start() <-chan SessionWatchResult {
	// Get initial line count
	count, err := CountJSONLLines(sw.filePath)
	if err != nil {
		// File might not exist yet, start from 0
		sw.lastLineCount = 0
	} else {
		sw.lastLineCount = count
	}

	sw.wg.Add(1)
	go sw.watch()

	return sw.resultChan
}

// Stop stops the session watcher and closes the result channel.
func (sw *SessionWatcher) Stop() {
	sw.stopOnce.Do(func() {
		sw.cancel()
		sw.wg.Wait()
		close(sw.resultChan)
	})
}

// watch is the main polling loop.
func (sw *SessionWatcher) watch() {
	defer sw.wg.Done()

	ticker := time.NewTicker(sw.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-sw.ctx.Done():
			return
		case <-ticker.C:
			sw.checkForUpdates()
		}
	}
}

// checkForUpdates checks if new messages have been added to the JSONL file.
func (sw *SessionWatcher) checkForUpdates() {
	currentCount, err := CountJSONLLines(sw.filePath)
	if err != nil {
		// File might have been deleted or is temporarily unavailable
		// Don't send error for transient issues
		return
	}

	// Check if new lines have been added
	if currentCount > sw.lastLineCount {
		// Read new messages starting from the last known line
		newMessages, err := ParseJSONLFileFromOffset(sw.filePath, sw.lastLineCount)
		if err != nil {
			sw.resultChan <- SessionWatchResult{
				SessionUUID: sw.sessionUUID,
				Error:       fmt.Errorf("failed to parse new messages: %w", err),
			}
			return
		}

		// Update last line count
		sw.lastLineCount = currentCount

		// Send result if we have new messages
		if len(newMessages) > 0 {
			sw.resultChan <- SessionWatchResult{
				SessionUUID: sw.sessionUUID,
				NewMessages: newMessages,
			}
		}
	}
}

// SessionWatchManager manages multiple session watchers.
type SessionWatchManager struct {
	watchers map[string]*SessionWatcher
	mu       sync.RWMutex
}

// NewSessionWatchManager creates a new watch manager.
func NewSessionWatchManager() *SessionWatchManager {
	return &SessionWatchManager{
		watchers: make(map[string]*SessionWatcher),
	}
}

// StartWatching starts watching a session.
// Returns a channel that receives updates for this session.
func (swm *SessionWatchManager) StartWatching(sessionUUID, filePath string, pollInterval time.Duration) (<-chan SessionWatchResult, error) {
	swm.mu.Lock()
	defer swm.mu.Unlock()

	// Check if already watching
	if _, exists := swm.watchers[sessionUUID]; exists {
		return nil, fmt.Errorf("already watching session %s", sessionUUID)
	}

	// Create and start watcher
	watcher := NewSessionWatcher(sessionUUID, filePath, pollInterval)
	resultChan := watcher.Start()

	swm.watchers[sessionUUID] = watcher

	return resultChan, nil
}

// StopWatching stops watching a session.
func (swm *SessionWatchManager) StopWatching(sessionUUID string) error {
	swm.mu.Lock()
	defer swm.mu.Unlock()

	watcher, exists := swm.watchers[sessionUUID]
	if !exists {
		return fmt.Errorf("not watching session %s", sessionUUID)
	}

	watcher.Stop()
	delete(swm.watchers, sessionUUID)

	return nil
}

// StopAll stops all active watchers.
func (swm *SessionWatchManager) StopAll() {
	swm.mu.Lock()
	defer swm.mu.Unlock()

	for sessionUUID, watcher := range swm.watchers {
		watcher.Stop()
		delete(swm.watchers, sessionUUID)
	}
}

// IsWatching checks if a session is currently being watched.
func (swm *SessionWatchManager) IsWatching(sessionUUID string) bool {
	swm.mu.RLock()
	defer swm.mu.RUnlock()

	_, exists := swm.watchers[sessionUUID]
	return exists
}

// GetActiveWatchers returns a list of session UUIDs currently being watched.
func (swm *SessionWatchManager) GetActiveWatchers() []string {
	swm.mu.RLock()
	defer swm.mu.RUnlock()

	uuids := make([]string, 0, len(swm.watchers))
	for uuid := range swm.watchers {
		uuids = append(uuids, uuid)
	}

	return uuids
}
