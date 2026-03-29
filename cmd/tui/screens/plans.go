package screens

import (
	"fmt"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/content"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PlansScreen handles plan browsing, viewing, and editing.
type PlansScreen struct {
	// Components.
	list      *components.List
	viewer    *components.Viewer
	editor    *components.Editor
	statusBar *components.StatusBar

	// State.
	layout  Layout
	focus   Focus
	plans   []claudeviewer.PlanSummary
	current *claudeviewer.PlanDetail

	// Dimensions.
	width  int
	height int

	// Styles.
	borderStyle lipgloss.Style
}

// NewPlansScreen creates a new plans screen.
func NewPlansScreen(width, height int) *PlansScreen {
	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	return &PlansScreen{
		list:      components.NewList(nil, panelWidth, contentHeight),
		viewer:    components.NewViewer(panelWidth, contentHeight),
		editor:    components.NewEditor(width-4, contentHeight),
		statusBar: components.NewStatusBar(width),
		layout:    LayoutSplit,
		focus:     FocusList,
		width:     width,
		height:    height,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
	}
}

// Init initializes the screen.
func (s *PlansScreen) Init() tea.Cmd {
	s.statusBar.SetLoading("Loading plans...")
	return nil // Plans loaded via PlansLoadedMsg from App.
}

// Update handles messages.
func (s *PlansScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case PlansLoadedMsg:
		s.plans = msg.Plans
		s.updateListItems()
		s.statusBar.Clear()
		if len(s.plans) > 0 {
			return s, s.loadPlanDetail(s.plans[0].FileName)
		}
		return s, nil

	case PlanDetailLoadedMsg:
		s.current = msg.Detail
		s.viewer.SetContent(content.NewPlanContent(msg.Detail))
		s.editor.SetContent(msg.Detail.Content)
		s.statusBar.Clear()
		return s, nil

	case SaveResultMsg:
		s.statusBar.Clear()
		if msg.Error != nil {
			s.statusBar.SetError(msg.Error.Error())
		} else if msg.Result.HasConflict {
			s.statusBar.SetError("Conflict: Plan was modified externally")
		} else if msg.Result.Success {
			s.statusBar.SetSuccess("Plan saved successfully")
			s.focus = FocusContent
			s.editor.Blur()
			// Reload the plan.
			if s.current != nil {
				return s, s.loadPlanDetail(s.current.FileName)
			}
		}
		return s, nil

	case SyncResultMsg:
		s.statusBar.Clear()
		if msg.Error != nil {
			s.statusBar.SetError("Sync failed: " + msg.Error.Error())
		} else {
			s.statusBar.SetSuccess(fmt.Sprintf("Synced %d plans", msg.Count))
		}
		return s, nil

	case ErrorMsg:
		s.statusBar.SetError(msg.Error.Error())
		return s, nil
	}

	// Update active component.
	var cmd tea.Cmd
	switch s.focus {
	case FocusList:
		cmd = s.list.Update(msg)
	case FocusContent:
		cmd = s.viewer.Update(msg)
	case FocusEditor:
		cmd = s.editor.Update(msg)
	}

	return s, cmd
}

// handleKey processes key input based on current focus.
func (s *PlansScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()

	switch s.focus {
	case FocusList:
		return s.handleListKey(key, msg)
	case FocusContent:
		return s.handleContentKey(key, msg)
	case FocusEditor:
		return s.handleEditorKey(key, msg)
	}

	return s, nil
}

// handleListKey handles keys in list focus mode.
func (s *PlansScreen) handleListKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "v":
		// Switch to fullscreen view.
		if s.current != nil {
			s.layout = LayoutFullscreen
			s.focus = FocusContent
		}
		return s, nil

	case "e":
		// Enter edit mode.
		if s.current != nil {
			s.layout = LayoutFullscreen
			s.focus = FocusEditor
			return s, s.editor.Focus()
		}
		return s, nil

	case "s":
		// Sync plans.
		s.statusBar.SetLoading("Syncing plans...")
		return s, s.syncPlans()

	case "j", "down", "k", "up":
		// Navigate list.
		cmd := s.list.Update(msg)
		// Load selected plan.
		if item := s.list.SelectedItem(); item != nil {
			if plan, ok := item.Data().(claudeviewer.PlanSummary); ok {
				return s, tea.Batch(cmd, s.loadPlanDetail(plan.FileName))
			}
		}
		return s, cmd
	}

	return s, s.list.Update(msg)
}

// handleContentKey handles keys in content view mode.
func (s *PlansScreen) handleContentKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "esc":
		// Back to split view.
		s.layout = LayoutSplit
		s.focus = FocusList
		s.viewer.GotoTop()
		return s, nil

	case "e":
		// Enter edit mode.
		if s.current != nil {
			s.focus = FocusEditor
			return s, s.editor.Focus()
		}
		return s, nil

	case "r":
		// Toggle render mode.
		s.viewer.ToggleRenderMode()
		return s, nil

	case "v":
		// View versions - check if versions exist first.
		if s.current != nil {
			s.statusBar.SetLoading("Checking versions...")
			return s, func() tea.Msg {
				return RequestVersionsScreenMsg{PlanName: s.current.FileName}
			}
		}
		return s, nil

	case "g":
		s.viewer.GotoTop()
		return s, nil

	case "G":
		s.viewer.GotoBottom()
		return s, nil
	}

	return s, s.viewer.Update(msg)
}

// handleEditorKey handles keys in editor mode.
func (s *PlansScreen) handleEditorKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "esc":
		// Cancel editing, back to content view.
		s.focus = FocusContent
		s.editor.Blur()
		s.editor.Reset()
		return s, nil

	case "ctrl+s":
		// Save.
		if s.current != nil {
			s.statusBar.SetLoading("Saving...")
			return s, s.savePlan()
		}
		return s, nil
	}

	return s, s.editor.Update(msg)
}

// View renders the screen.
func (s *PlansScreen) View() string {
	s.updateStatusBarHelp()

	var mainContent string

	switch s.layout {
	case LayoutFullscreen:
		if s.focus == FocusEditor {
			mainContent = s.renderFullscreenEditor()
		} else {
			mainContent = s.renderFullscreenViewer()
		}
	default:
		mainContent = s.renderSplitView()
	}

	return mainContent
}

// renderSplitView renders the two-panel layout.
func (s *PlansScreen) renderSplitView() string {
	panelWidth := (s.width - 3) / 2
	contentHeight := s.height - 4

	// Update component sizes.
	s.list.SetSize(panelWidth-4, contentHeight-4)
	s.viewer.SetSize(panelWidth-4, contentHeight-4)

	leftPanel := s.borderStyle.
		Width(panelWidth).
		Height(contentHeight).
		Render(s.list.View())

	rightPanel := s.borderStyle.
		Width(panelWidth).
		Height(contentHeight).
		Render(s.viewer.View())

	divider := s.renderDivider(contentHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, divider, rightPanel)
}

// renderFullscreenViewer renders fullscreen content view.
func (s *PlansScreen) renderFullscreenViewer() string {
	contentHeight := s.height - 4
	s.viewer.SetSize(s.width-4, contentHeight-4)

	return s.borderStyle.
		Width(s.width).
		Height(contentHeight).
		Render(s.viewer.View())
}

// renderFullscreenEditor renders fullscreen editor.
func (s *PlansScreen) renderFullscreenEditor() string {
	contentHeight := s.height - 4
	s.editor.SetSize(s.width-4, contentHeight-4)

	return s.borderStyle.
		Width(s.width).
		Height(contentHeight).
		Render(s.editor.View())
}

// renderDivider renders a vertical divider.
func (s *PlansScreen) renderDivider(height int) string {
	divider := ""
	for i := 0; i < height; i++ {
		divider += "│"
		if i < height-1 {
			divider += "\n"
		}
	}
	return divider
}

// SetSize updates screen dimensions.
func (s *PlansScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.statusBar.SetWidth(width)
}

// ShortHelp returns key binding help.
func (s *PlansScreen) ShortHelp() string {
	switch s.focus {
	case FocusList:
		return fmt.Sprintf("j/k: navigate | v: view | e: edit | s: sync | ?: help | q: quit | Plans: %d", len(s.plans))
	case FocusContent:
		mode := "RAW"
		if s.viewer.RenderMode() == components.RenderModeHTML {
			mode = "HTML"
		}
		return fmt.Sprintf("j/k: scroll | g/G: top/bottom | r: render (%s) | e: edit | v: versions | esc: back", mode)
	case FocusEditor:
		modified := ""
		if s.editor.IsModified() {
			modified = " [MODIFIED]"
		}
		return fmt.Sprintf("ctrl+s: save | esc: cancel%s", modified)
	}
	return ""
}

// updateStatusBarHelp updates the status bar with current help text.
func (s *PlansScreen) updateStatusBarHelp() {
	s.statusBar.SetHelp(s.ShortHelp())
}

// updateListItems updates the list with current plans.
func (s *PlansScreen) updateListItems() {
	items := make([]components.ListItem, len(s.plans))
	for i, plan := range s.plans {
		items[i] = components.NewListItem(
			plan.Title,
			fmt.Sprintf("%s | %d min read", plan.ModifiedAt.Format("2006-01-02"), plan.ReadingTime),
			plan,
		)
	}
	s.list.SetItems(items)
}

// Command helpers.

func (s *PlansScreen) loadPlanDetail(fileName string) tea.Cmd {
	s.statusBar.SetLoading("Loading plan...")
	return func() tea.Msg {
		return LoadPlanDetailMsg{FileName: fileName}
	}
}

func (s *PlansScreen) savePlan() tea.Cmd {
	return func() tea.Msg {
		return SavePlanMsg{
			FileName: s.current.FileName,
			Content:  s.editor.Content(),
			Modified: s.current.ModifiedAt,
		}
	}
}

func (s *PlansScreen) syncPlans() tea.Cmd {
	return func() tea.Msg {
		return SyncPlansMsg{}
	}
}

// Message types for plans screen.

// PlansLoadedMsg is sent when plans are loaded.
type PlansLoadedMsg struct {
	Plans []claudeviewer.PlanSummary
}

// PlanDetailLoadedMsg is sent when plan detail is loaded.
type PlanDetailLoadedMsg struct {
	Detail *claudeviewer.PlanDetail
}

// LoadPlanDetailMsg requests loading a plan detail.
type LoadPlanDetailMsg struct {
	FileName string
}

// SavePlanMsg requests saving a plan.
type SavePlanMsg struct {
	FileName string
	Content  string
	Modified time.Time
}

// SyncPlansMsg requests syncing plans.
type SyncPlansMsg struct{}

// SaveResultMsg is sent when save completes.
type SaveResultMsg struct {
	Result *claudeviewer.UpdatePlanResult
	Error  error
}

// SyncResultMsg is sent when sync completes.
type SyncResultMsg struct {
	Count int
	Error error
}

// ErrorMsg is sent on error.
type ErrorMsg struct {
	Error error
}

// RequestVersionsScreenMsg requests checking versions before navigation.
type RequestVersionsScreenMsg struct {
	PlanName string
}

// VersionsNavigationResultMsg carries the result of version check.
type VersionsNavigationResultMsg struct {
	PlanName string
	Versions []claudeviewer.PlanVersionDetail
}
