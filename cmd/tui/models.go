package tui

import "errors"

// Custom errors used by the TUI.
var (
	ErrNoPlanSelected   = errors.New("no plan selected")
	ErrConflictDetected = errors.New("conflict detected: plan was modified externally")
	ErrUpdateFailed     = errors.New("failed to update plan")
)
