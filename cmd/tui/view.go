package tui

import (
	"fmt"
	"strings"

	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	"github.com/charmbracelet/lipgloss"
)

// Styling constants.
var (
	// Primary colors
	accentColor   = lipgloss.Color("212") // Magenta
	activeStyle   = lipgloss.NewStyle().Foreground(accentColor).Bold(true)
	inactiveStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))

	// Message styles
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("160")).Bold(true) // Dark red
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)  // Green
	loadingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("81")).Bold(true)  // Cyan
	helpStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("250")).Italic(true)

	// UI colors
	headerBg    = lipgloss.Color("235") // Dark gray
	borderColor = lipgloss.Color("238") // Darker gray
	mutedColor  = lipgloss.Color("244") // Gray
)

// renderUI renders the complete TUI.
func renderUI(a *App) string {
	if a.Width == 0 || a.Height == 0 {
		return "Loading..."
	}

	if a.State.ShowHelp {
		return renderHelpScreen(a)
	}

	// Main layout
	header := renderHeader(a)
	status := renderStatus(a)
	mainContent := renderMainContent(a)

	// Combine with proper spacing
	return fmt.Sprintf("%s\n%s\n%s", header, mainContent, status)
}

// renderHeader renders the top header bar.
func renderHeader(a *App) string {
	var modeStr string
	modifiedStr := ""

	switch a.State.Mode {
	case TwoPanelMode:
		modeStr = "TWO-PANEL"
	case FullscreenViewMode:
		renderMode := "MARKDOWN"
		if a.State.ViewportRenderMarkdown {
			renderMode = "HTML"
		}
		modeStr = fmt.Sprintf("FULLSCREEN VIEW (%s)", renderMode)
	case EditMode:
		modeStr = "EDIT"
		if a.State.Modified {
			modifiedStr = " ● MODIFIED"
		}
	case HelpMode:
		modeStr = "HELP"
	default:
		modeStr = "UNKNOWN"
	}

	header := fmt.Sprintf(" Claude Plan Viewer  |  Mode: %s%s  |  Plans: %d",
		modeStr, modifiedStr, len(a.State.Plans))

	return lipgloss.NewStyle().
		Width(a.Width).
		Padding(0, 2).
		Background(headerBg).
		Foreground(lipgloss.Color("250")).
		Render(header)
}

// renderStatus renders the bottom status bar.
func renderStatus(a *App) string {
	var statusMsg string

	if a.State.IsLoading {
		// Show loading spinner with color
		spinner := getSpinner()
		statusMsg = loadingStyle.Render(fmt.Sprintf("%s %s", spinner, a.State.LoadingMessage))
	} else if a.State.ErrorMessage != "" {
		statusMsg = errorStyle.Render("✗ " + a.State.ErrorMessage)
		a.State.ErrorMessage = "" // Clear for next render
	} else if a.State.SuccessMessage != "" {
		statusMsg = successStyle.Render("✓ " + a.State.SuccessMessage)
		a.State.SuccessMessage = "" // Clear for next render
	} else {
		switch a.State.Mode {
		case TwoPanelMode:
			statusMsg = fmt.Sprintf("j/k: navigate | e: edit | v: view | ?: help | s: sync | q: quit | Plans: %d", len(a.State.Plans))
		case FullscreenViewMode:
			statusMsg = "j/k: scroll | g: top | G: bottom | r: render | e: edit | Esc: back | ?: help | q: quit"
		case EditMode:
			statusMsg = "Ctrl+S: save & sync | Esc: cancel | ?: help | q: quit"
		}
	}

	return lipgloss.NewStyle().
		Width(a.Width).
		Padding(0, 2).
		Background(headerBg).
		Foreground(mutedColor).
		Render(statusMsg)
}

// renderMainContent renders the appropriate layout based on mode.
func renderMainContent(a *App) string {
	if a.State.Mode == FullscreenViewMode && a.State.CurrentPlan != nil {
		return renderFullscreenView(a)
	}

	if a.State.Mode == EditMode && a.State.CurrentPlan != nil {
		return renderFullscreenEdit(a)
	}

	// Default: two-panel layout (TwoPanelMode)
	panelWidth := (a.Width - 3) / 2 // -3 for borders and spacing
	contentHeight := a.Height - 2   // -4 for header and status

	leftPanel := renderLeftPanel(a, panelWidth, contentHeight)
	rightPanel := renderRightPanel(a, panelWidth, contentHeight)

	// Combine panels horizontally
	return lipgloss.JoinHorizontal(
		lipgloss.Bottom,
		leftPanel,
		renderVerticalDivider(contentHeight),
		rightPanel,
	)
}

// renderLeftPanel renders the plans list panel.
func renderLeftPanel(a *App, width, height int) string {
	content := ""

	if len(a.State.Plans) == 0 {
		content = "No plans found"
	} else {
		for i, plan := range a.State.Plans {
			var line string
			if i == a.State.SelectedIndex {
				line = activeStyle.Render(fmt.Sprintf("❯ %s", plan.Title))
			} else {
				line = inactiveStyle.Render(fmt.Sprintf("  %s", plan.Title))
			}
			content += line + "\n"

			// Stop when we hit height limit
			if i >= height-1 {
				content += inactiveStyle.Render("  ...")
				break
			}
		}
	}

	panel := lipgloss.NewStyle().
		Width(width).
		Height(height).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(content)

	return panel
}

// renderRightPanel renders the plan detail/editor panel.
func renderRightPanel(a *App, width, height int) string {
	if a.State.CurrentPlan == nil {
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Render("Select a plan to view")
	}

	switch a.State.Mode {
	case TwoPanelMode:
		content := renderPlanContent(a.State.CurrentPlan, height)
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Render(content)
	case EditMode:
		// Render the textarea for editing - maximize usable space
		a.State.TextArea.SetWidth(width - 2)
		a.State.TextArea.SetHeight(height - 2)
		return lipgloss.NewStyle().
			Width(width).
			Height(height).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(borderColor).
			Render(a.State.TextArea.View())
	}

	return ""
}

// renderVerticalDivider renders a vertical divider.
func renderVerticalDivider(height int) string {
	lines := make([]string, height)
	for i := range lines {
		lines[i] = "│"
	}
	return strings.Join(lines, "\n")
}

// renderPlanContent renders the plan detail in view mode.
func renderPlanContent(plan *claudeviewer.PlanDetail, height int) string {
	// Truncate content to fit in panel
	lines := strings.Split(plan.Content, "\n")
	if len(lines) > height-3 {
		lines = lines[:height-3]
	}

	title := activeStyle.Render(plan.Title)
	meta := helpStyle.Render(fmt.Sprintf(
		"📅 Modified: %s | ⏱️ Reading Time: %d min | Size: %d bytes",
		plan.ModifiedAt.Format("2006-01-02"),
		plan.ReadingTime,
		plan.FileSize,
	))

	content := strings.Join(lines, "\n")

	return fmt.Sprintf("%s\n%s\n\n%s", title, meta, content)
}

// renderHelpScreen renders the help overlay.
func renderHelpScreen(a *App) string {
	helpText := `
Claude Plan Viewer - Keyboard Shortcuts

NAVIGATION (Two-Panel mode):
  j/k, ↑/↓       Navigate plans list
  e              Enter edit mode
  v              Enter fullscreen view mode

SCROLLING (Fullscreen View mode):
  j/k, ↑/↓       Scroll up/down
  g              Jump to top
  G              Jump to bottom
  r              Toggle markdown rendering (raw vs HTML)

EDITING (Edit mode):
  Ctrl+S         Save and sync changes
  Esc            Cancel without saving

GENERAL:
  ?              Toggle help
  q, Ctrl+C      Quit application

MODES:
  TWO-PANEL      - Default view with list on left, preview on right
  FULLSCREEN VIEW - Full-screen markdown viewing with scroll support
  EDIT           - Full-screen text editor with line numbers
`

	helpBox := lipgloss.NewStyle().
		Width(a.Width-4).
		Padding(2, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("206")).
		Render(helpText)

	return fmt.Sprintf("\n\n%s\n\n Press '?' to close help", helpBox)
}

// getSpinner returns a simple animated spinner character.
func getSpinner() string {
	// Simple rotating spinner
	return "⟳"
}

// renderFullscreenView renders a full-screen markdown view with scrolling support.
func renderFullscreenView(a *App) string {
	contentHeight := a.Height - 4 // -4 for header and status

	// Get all content lines including metadata
	allLines := a.getViewportLines()

	// Apply scrolling offset
	startLine := a.State.ViewportScrollOffset
	endLine := startLine + contentHeight

	// Clamp to valid range
	if endLine > len(allLines) {
		endLine = len(allLines)
	}
	if startLine > len(allLines) {
		startLine = len(allLines) - 1
	}
	if startLine < 0 {
		startLine = 0
	}

	// Get visible lines
	visibleLines := allLines[startLine:endLine]

	// Pad with empty lines if needed
	for len(visibleLines) < contentHeight {
		visibleLines = append(visibleLines, "")
	}

	content := strings.Join(visibleLines, "\n")

	return lipgloss.NewStyle().
		Width(a.Width).
		Height(contentHeight).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(content)
}

// renderFullscreenEdit renders a full-screen text editor.
func renderFullscreenEdit(a *App) string {
	contentHeight := a.Height - 4 // -4 for header and status

	// Set textarea dimensions to use full screen
	a.State.TextArea.SetWidth(a.Width - 4)
	a.State.TextArea.SetHeight(contentHeight - 2)

	return lipgloss.NewStyle().
		Width(a.Width).
		Height(contentHeight).
		Padding(1, 2).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(a.State.TextArea.View())
}
