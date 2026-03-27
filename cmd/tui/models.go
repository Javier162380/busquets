package tui

import (
	"errors"

	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	"github.com/charmbracelet/bubbles/textarea"
)

// Mode represents the current UI mode.
type Mode int

const (
	TwoPanelMode       Mode = iota // Plans list + viewer/editor side by side
	FullscreenViewMode             // Full-screen markdown view
	EditMode                       // Full-screen editor
	HelpMode
)

// AppState holds the complete state of the TUI application.
type AppState struct {
	// Plans
	Plans         []claudeviewer.PlanSummary
	CurrentPlan   *claudeviewer.PlanDetail
	SelectedIndex int

	// Editing
	Mode     Mode
	TextArea textarea.Model
	Modified bool // Track if content changed

	// Scrolling in fullscreen view
	ViewportScrollOffset   int  // Line offset for fullscreen view scrolling
	ViewportRenderMarkdown bool // Whether to render markdown in fullscreen view

	// Loading/UI state
	IsLoading      bool   // Show loading indicator
	LoadingMessage string // Loading status message

	// Messages
	ShowHelp       bool
	ErrorMessage   string
	SuccessMessage string
}

// NewAppState creates a new app state.
func NewAppState() AppState {
	ta := textarea.New()
	ta.Placeholder = "Plan content will appear here..."
	ta.ShowLineNumbers = true
	ta.Blur()

	return AppState{
		Mode:     TwoPanelMode,
		TextArea: ta,
	}
}

// Message types for Bubble Tea command results.

// PlansLoadedMsg is sent when plans are loaded from the service.
type PlansLoadedMsg struct {
	Plans []claudeviewer.PlanSummary
}

// PlanDetailLoadedMsg is sent when a plan detail is loaded.
type PlanDetailLoadedMsg struct {
	Detail *claudeviewer.PlanDetail
}

// ErrorMsg is sent when an operation fails.
type ErrorMsg struct {
	Error error
}

// SuccessMsg is sent when an operation succeeds.
type SuccessMsg struct {
	Message string
}

// SaveResultMsg is sent when a plan save completes.
type SaveResultMsg struct {
	Result *claudeviewer.UpdatePlanResult
	Error  error
}

type SyncPlansResultMsg struct {
	Message string
	Error   error
}

// Custom errors.
var (
	ErrNoPlanSelected   = errors.New("no plan selected")
	ErrConflictDetected = errors.New("conflict detected: plan was modified externally")
	ErrUpdateFailed     = errors.New("failed to update plan")
)
