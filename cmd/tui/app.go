// Package tui cli utility to interact with the plans
package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository"

	tea "github.com/charmbracelet/bubbletea"
)

// App represents the TUI application state and logic.
type App struct {
	service UnifiedService
	ctx     context.Context

	// State
	State AppState

	// Dimensions
	Width  int
	Height int
}

// UnifiedService is the interface the TUI expects from the service.
type UnifiedService interface {
	ListAllPlansWithReadingTime(ctx context.Context) ([]claudeviewer.PlanSummary, error)
	GetPlanDetailByFileName(ctx context.Context, fileName string) (*claudeviewer.PlanDetail, error)
	UpdatePlan(ctx context.Context, req claudeviewer.UpdatePlanRequest) (*claudeviewer.UpdatePlanResult, error)
	SearchPlansWithReadingTime(ctx context.Context, query string) ([]claudeviewer.PlanSummary, error)
	SavePlanVersion(ctx context.Context, planName, content string) error
	GetPlanVersionHistory(ctx context.Context, planName string, offset, limit int64) ([]repository.PlanVersion, error)
	RestorePlanVersion(ctx context.Context, planName string, versionNumber int64) error
	GetStringValue(ctx context.Context, variableName string) (string, bool, error)
	GetBooleanValue(ctx context.Context, variableName string) (bool, bool, error)
	GetNumberValue(ctx context.Context, variableName string) (float64, bool, error)
	GetDateTimeValue(ctx context.Context, variableName string) (time.Time, bool, error)
	SetSetting(ctx context.Context, varName, varType string, values claudeviewer.SettingValues) error
	SyncPlans(ctx context.Context) (int, error)
	RenderMarkdown(content string) (string, error)
}

// New creates a new TUI application.
func New(service UnifiedService) *App {
	return &App{
		service: service,
		ctx:     context.Background(),
		State:   NewAppState(),
	}
}

// Init initializes the app and loads initial data.
func (a *App) Init() tea.Cmd {
	return a.loadPlans()
}

// Update processes messages and updates state.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return a.handleKeyInput(msg)
	case tea.WindowSizeMsg:
		a.Width = msg.Width
		a.Height = msg.Height
		return a, nil
	case PlansLoadedMsg:
		a.State.IsLoading = false
		a.State.Plans = msg.Plans
		a.State.SelectedIndex = 0
		if len(a.State.Plans) > 0 {
			return a, a.loadPlanDetail(a.State.Plans[0].FileName)
		}
		return a, nil
	case PlanDetailLoadedMsg:
		a.State.IsLoading = false
		a.State.CurrentPlan = msg.Detail
		a.State.TextArea.SetValue(msg.Detail.Content)
		a.State.Modified = false
		return a, nil
	case SaveResultMsg:
		// Handle save result (async completion)
		a.State.IsLoading = false
		if msg.Error != nil {
			a.State.ErrorMessage = msg.Error.Error()
		} else if msg.Result.HasConflict {
			a.State.ErrorMessage = "Conflict: Plan was modified externally"
		} else if msg.Result.Success {
			a.State.Mode = TwoPanelMode
			a.State.TextArea.Blur()
			a.State.Modified = false
			a.State.SuccessMessage = "Plan saved and synced successfully"
			// Reload the plan to show any server updates
			return a, a.loadPlanDetail(a.State.CurrentPlan.FileName)
		}
		return a, nil
	case SyncPlansResultMsg:
		a.State.IsLoading = false
		if msg.Error != nil {
			a.State.ErrorMessage = "Failed to sync plans: " + msg.Error.Error()
		} else {
			a.State.TextArea.Blur()
			a.State.Modified = false
			a.State.SuccessMessage = msg.Message
			return a, nil
		}
		return a, nil
	case ErrorMsg:
		a.State.IsLoading = false
		a.State.ErrorMessage = msg.Error.Error()
		return a, nil
	case SuccessMsg:
		a.State.SuccessMessage = msg.Message
		return a, nil
	}

	return a, nil
}

// View renders the UI.
func (a *App) View() string {
	return renderUI(a)
}

// handleKeyInput processes keyboard input.
func (a *App) handleKeyInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch a.State.Mode {
	case EditMode:
		// Only intercept special keys in edit mode
		switch msg.String() {
		case "ctrl+s":
			a.State.IsLoading = true
			a.State.LoadingMessage = "Saving and syncing plan..."
			return a, a.savePlanAndSync()
		case "esc":
			a.State.Mode = TwoPanelMode
			a.State.TextArea.Blur()
			return a, nil
		default:
			// All other keys go to textarea
			var cmd tea.Cmd
			a.State.TextArea, cmd = a.State.TextArea.Update(msg)
			a.State.Modified = true
			return a, cmd
		}
	case FullscreenViewMode:
		switch msg.String() {
		case "esc":
			a.State.Mode = TwoPanelMode
			a.State.ViewportScrollOffset = 0 // Reset scroll on exit
			return a, nil
		case "r":
			// Toggle markdown rendering
			a.State.ViewportRenderMarkdown = !a.State.ViewportRenderMarkdown
			a.State.ViewportScrollOffset = 0 // Reset scroll when toggling render
			return a, nil
		case "e":
			// Switch from fullscreen view to edit mode
			if a.State.CurrentPlan != nil {
				a.State.Mode = EditMode
				a.State.TextArea.Focus()
				a.State.Modified = false
			}
			return a, nil
		case "j", "down":
			// Scroll down in fullscreen view
			contentLines := len(a.viewportContentLines())
			contentHeight := a.Height - 4 // -4 for header and status
			maxScroll := contentLines - contentHeight
			if maxScroll > 0 && a.State.ViewportScrollOffset < maxScroll {
				a.State.ViewportScrollOffset++
			}
			return a, nil
		case "k", "up":
			// Scroll up in fullscreen view
			if a.State.ViewportScrollOffset > 0 {
				a.State.ViewportScrollOffset--
			}
			return a, nil
		case "g":
			// Jump to top
			a.State.ViewportScrollOffset = 0
			return a, nil
		case "G":
			// Jump to bottom
			contentLines := len(a.viewportContentLines())
			contentHeight := a.Height - 4
			maxScroll := contentLines - contentHeight
			if maxScroll > 0 {
				a.State.ViewportScrollOffset = maxScroll
			}
			return a, nil
		}
	case TwoPanelMode:
		switch msg.String() {
		case "q", "ctrl+c":
			return a, tea.Quit
		case "?":
			a.State.ShowHelp = !a.State.ShowHelp
			return a, nil
		case "e":
			if a.State.Mode == TwoPanelMode && a.State.CurrentPlan != nil {
				a.State.Mode = EditMode
				a.State.TextArea.Focus()
				a.State.Modified = false
			}
			return a, nil
		case "v":
			if a.State.Mode == TwoPanelMode && a.State.CurrentPlan != nil {
				a.State.Mode = FullscreenViewMode
			}
			return a, nil
		case "j", "down":
			if a.State.SelectedIndex < len(a.State.Plans)-1 {
				a.State.SelectedIndex++
				if len(a.State.Plans) > a.State.SelectedIndex {
					return a, a.loadPlanDetail(a.State.Plans[a.State.SelectedIndex].FileName)
				}
			}
			return a, nil
		case "k", "up":
			if a.State.SelectedIndex > 0 {
				a.State.SelectedIndex--
				return a, a.loadPlanDetail(a.State.Plans[a.State.SelectedIndex].FileName)
			}
			return a, nil
		case "s":
			a.State.IsLoading = true
			a.State.LoadingMessage = "Syncing plans..."
			return a, a.syncPlans()
		}
	default:
		return a, nil
	}
	return a, nil
}

// loadPlans loads all plans from the service.
func (a *App) loadPlans() tea.Cmd {
	a.State.IsLoading = true
	a.State.LoadingMessage = "Loading plans..."
	return func() tea.Msg {
		plans, err := a.service.ListAllPlansWithReadingTime(a.ctx)
		if err != nil {
			return ErrorMsg{Error: err}
		}
		return PlansLoadedMsg{Plans: plans}
	}
}

// loadPlanDetail loads a specific plan's details.
func (a *App) loadPlanDetail(fileName string) tea.Cmd {
	a.State.IsLoading = true
	a.State.LoadingMessage = "Loading plan..."
	return func() tea.Msg {
		detail, err := a.service.GetPlanDetailByFileName(a.ctx, fileName)
		if err != nil {
			return ErrorMsg{Error: err}
		}
		return PlanDetailLoadedMsg{Detail: detail}
	}
}

// savePlanAndSync saves the plan and syncs to source directory.
func (a *App) savePlanAndSync() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
		defer cancel()

		if a.State.CurrentPlan == nil {
			return ErrorMsg{Error: ErrNoPlanSelected}
		}

		// Create update request with textarea content
		req := claudeviewer.UpdatePlanRequest{
			FileName:         a.State.CurrentPlan.FileName,
			NewContent:       a.State.TextArea.Value(),
			LastModifiedTime: a.State.CurrentPlan.ModifiedAt,
			Force:            true,
		}

		// Update the plan (syncs automatically)
		result, err := a.service.UpdatePlan(ctx, req)
		if err != nil {
			return SaveResultMsg{Error: err}
		}

		// Return result as message
		return SaveResultMsg{Result: result, Error: nil}
	}
}

func (a *App) syncPlans() tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
		defer cancel()

		// Update the plan (syncs automatically)
		result, err := a.service.SyncPlans(ctx)
		if err != nil {
			return SyncPlansResultMsg{Message: "Failed to sync plans", Error: err}
		}

		return SyncPlansResultMsg{Message: fmt.Sprintf("Synced %d plans", result), Error: nil}
	}
}

// viewportContentLines returns the split lines of the current plan for scrolling.
func (a *App) viewportContentLines() []string {
	if a.State.CurrentPlan == nil {
		return []string{}
	}

	// Split the content into lines
	lines := strings.Split(a.State.CurrentPlan.Content, "\n")

	// Add metadata lines at the top
	metaLine := fmt.Sprintf("📅 Modified: %s | ⏱️ Reading Time: %d min | Size: %d bytes",
		a.State.CurrentPlan.ModifiedAt.Format("2006-01-02"),
		a.State.CurrentPlan.ReadingTime,
		a.State.CurrentPlan.FileSize)

	allLines := []string{
		a.State.CurrentPlan.Title,
		metaLine,
		"",
	}
	allLines = append(allLines, lines...)

	return allLines
}

// getViewportLines is an alias for viewportContentLines for use in view.go
func (a *App) getViewportLines() []string {
	if a.State.ViewportRenderMarkdown {
		return a.getRenderedMarkdownLines()
	}
	return a.viewportContentLines()
}

// stripHTMLTags removes HTML tags from a string, leaving only the content
func stripHTMLTags(html string) string {
	// Remove common HTML tags
	replacements := map[string]string{
		"<p>":           "",
		"</p>":          "\n",
		"<br>":          "\n",
		"<br/>":         "\n",
		"<br />":        "\n",
		"<strong>":      "",
		"</strong>":     "",
		"<b>":           "",
		"</b>":          "",
		"<em>":          "",
		"</em>":         "",
		"<i>":           "",
		"</i>":          "",
		"<u>":           "",
		"</u>":          "",
		"<code>":        "",
		"</code>":       "",
		"<pre>":         "",
		"</pre>":        "",
		"<h1>":          "\n",
		"</h1>":         "\n",
		"<h2>":          "\n",
		"</h2>":         "\n",
		"<h3>":          "\n",
		"</h3>":         "\n",
		"<h4>":          "\n",
		"</h4>":         "\n",
		"<h5>":          "\n",
		"</h5>":         "\n",
		"<h6>":          "\n",
		"</h6>":         "\n",
		"<ul>":          "",
		"</ul>":         "",
		"<ol>":          "",
		"</ol>":         "",
		"<li>":          "  • ",
		"</li>":         "\n",
		"<table>":       "",
		"</table>":      "",
		"<tr>":          "",
		"</tr>":         "\n",
		"<td>":          "",
		"</td>":         " | ",
		"<th>":          "",
		"</th>":         " | ",
		"<thead>":       "",
		"</thead>":      "",
		"<tbody>":       "",
		"</tbody>":      "",
		"<blockquote>":  "> ",
		"</blockquote>": "\n",
		"<div>":         "",
		"</div>":        "\n",
		"<span>":        "",
		"</span>":       "",
		"<a href=\"":    "[",
		"\">":           "](link)",
		"</a>":          "",
		"&lt;":          "<",
		"&gt;":          ">",
		"&amp;":         "&",
		"&quot;":        "\"",
		"&#39;":         "'",
		"<hr>":          "─────────────────────────",
		"<hr/>":         "─────────────────────────",
		"<hr />":        "─────────────────────────",
	}

	result := html
	for tag, replacement := range replacements {
		result = strings.ReplaceAll(result, tag, replacement)
	}

	return result
}

// getRenderedMarkdownLines returns rendered HTML markdown lines for display
func (a *App) getRenderedMarkdownLines() []string {
	if a.State.CurrentPlan == nil {
		return []string{}
	}

	// Render the markdown to HTML
	html, err := a.service.RenderMarkdown(a.State.CurrentPlan.Content)
	if err != nil {
		// Fall back to plain text if rendering fails
		return a.viewportContentLines()
	}

	// Strip HTML tags to make it readable
	cleanText := stripHTMLTags(html)

	// Split the cleaned text into lines
	lines := strings.Split(cleanText, "\n")

	// Remove empty lines and clean up spacing
	var cleanLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			cleanLines = append(cleanLines, trimmed)
		}
	}

	// Add metadata at the top
	metaLine := fmt.Sprintf("📅 Modified: %s | ⏱️ Reading Time: %d min | Size: %d bytes",
		a.State.CurrentPlan.ModifiedAt.Format("2006-01-02"),
		a.State.CurrentPlan.ReadingTime,
		a.State.CurrentPlan.FileSize)

	allLines := []string{
		a.State.CurrentPlan.Title,
		metaLine,
		"",
	}
	allLines = append(allLines, cleanLines...)

	return allLines
}
