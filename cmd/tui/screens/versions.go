package screens

import (
	"fmt"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/content"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// VersionsScreen handles version history browsing and viewing.
type VersionsScreen struct {
	// Components.
	list      *components.List
	viewer    *components.Viewer
	statusBar *components.StatusBar

	// State.
	layout   Layout
	focus    Focus
	planName string
	versions []claudeviewer.PlanVersionDetail
	current  *claudeviewer.PlanVersionDetail

	// Dimensions.
	width  int
	height int

	// Styles.
	borderStyle lipgloss.Style
}

// NewVersionsScreen creates a new versions screen.
func NewVersionsScreen(planName string, width, height int) *VersionsScreen {
	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	return &VersionsScreen{
		list:      components.NewList(nil, panelWidth, contentHeight),
		viewer:    components.NewViewer(panelWidth, contentHeight),
		statusBar: components.NewStatusBar(width),
		layout:    LayoutSplit,
		focus:     FocusList,
		planName:  planName,
		width:     width,
		height:    height,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
	}
}

// NewVersionsScreenWithData creates a versions screen with pre-loaded data.
func NewVersionsScreenWithData(planName string, versions []claudeviewer.PlanVersionDetail, width, height int) *VersionsScreen {
	s := NewVersionsScreen(planName, width, height)
	s.versions = versions
	s.updateListItems()
	if len(versions) > 0 {
		s.current = &s.versions[0]
		s.viewer.SetContent(content.NewVersionContent(s.current))
	}
	return s
}

// Init initializes the screen.
func (s *VersionsScreen) Init() tea.Cmd {
	// If versions are already loaded, no need to fetch.
	if len(s.versions) > 0 {
		return nil
	}
	s.statusBar.SetLoading("Loading versions...")
	return func() tea.Msg {
		return LoadVersionsMsg{PlanName: s.planName}
	}
}

// Update handles messages.
func (s *VersionsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case VersionsLoadedMsg:
		s.versions = msg.Versions
		s.updateListItems()
		s.statusBar.Clear()
		if len(s.versions) > 0 {
			s.current = &s.versions[0]
			s.viewer.SetContent(content.NewVersionContent(s.current))
		}
		return s, nil

	case VersionErrorMsg:
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
	}
	return s, cmd
}

// handleKey processes key input based on current focus.
func (s *VersionsScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()

	switch s.focus {
	case FocusList:
		return s.handleListKey(key, msg)
	case FocusContent:
		return s.handleContentKey(key, msg)
	}
	return s, nil
}

// handleListKey handles keys in list focus mode.
func (s *VersionsScreen) handleListKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "esc":
		// Go back to plans screen (pop).
		return s, func() tea.Msg {
			return PopScreenMsg{}
		}
	case "v":
		// Switch to fullscreen view.
		if s.current != nil {
			s.layout = LayoutFullscreen
			s.focus = FocusContent
		}
		return s, nil
	case "r":
		// Restore selected version.
		if s.current != nil {
			return s, s.restoreVersion()
		}
		return s, nil
	case "j", "down", "k", "up":
		// Navigate list.
		cmd := s.list.Update(msg)
		// Update current version.
		if item := s.list.SelectedItem(); item != nil {
			if version, ok := item.Data().(claudeviewer.PlanVersionDetail); ok {
				s.current = &version
				s.viewer.SetContent(content.NewVersionContent(s.current))
			}
		}
		return s, cmd
	}

	return s, s.list.Update(msg)
}

// handleContentKey handles keys in content view mode.
func (s *VersionsScreen) handleContentKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "esc":
		// Back to split view.
		s.layout = LayoutSplit
		s.focus = FocusList
		s.viewer.GotoTop()
		return s, nil

	case "r":
		// Restore selected version.
		if s.current != nil {
			return s, s.restoreVersion()
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

// View renders the screen.
func (s *VersionsScreen) View() string {
	s.updateStatusBarHelp()

	var mainContent string

	switch s.layout {
	case LayoutFullscreen:
		mainContent = s.renderFullscreenViewer()
	default:
		mainContent = s.renderSplitView()
	}

	return mainContent
}

// renderSplitView renders the two-panel layout.
func (s *VersionsScreen) renderSplitView() string {
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
func (s *VersionsScreen) renderFullscreenViewer() string {
	contentHeight := s.height - 4
	s.viewer.SetSize(s.width-4, contentHeight-4)

	return s.borderStyle.
		Width(s.width).
		Height(contentHeight).
		Render(s.viewer.View())
}

// renderDivider renders a vertical divider.
func (s *VersionsScreen) renderDivider(height int) string {
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
func (s *VersionsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.statusBar.SetWidth(width)
}

// ShortHelp returns key binding help.
func (s *VersionsScreen) ShortHelp() string {
	switch s.focus {
	case FocusList:
		return fmt.Sprintf("j/k: navigate | v: view | r: restore | esc: back | Versions: %d", len(s.versions))
	case FocusContent:
		return "j/k: scroll | g/G: top/bottom | r: restore | esc: back"
	}
	return ""
}

// updateStatusBarHelp updates the status bar with current help text.
func (s *VersionsScreen) updateStatusBarHelp() {
	s.statusBar.SetHelp(s.ShortHelp())
}

// updateListItems updates the list with current versions.
func (s *VersionsScreen) updateListItems() {
	items := make([]components.ListItem, len(s.versions))
	for i, version := range s.versions {
		items[i] = components.NewListItem(
			fmt.Sprintf("Version %d", version.VersionNumber),
			fmt.Sprintf("%s | %d min read", version.CreatedAt.Format("2006-01-02 15:04"), version.ReadingTime),
			version,
		)
	}
	s.list.SetItems(items)
}

// Command helpers.

func (s *VersionsScreen) restoreVersion() tea.Cmd {
	return func() tea.Msg {
		return RestoreVersionMsg{
			PlanName:      s.planName,
			VersionNumber: s.current.VersionNumber,
		}
	}
}

// Message types for versions screen.

// LoadVersionsMsg requests loading versions.
type LoadVersionsMsg struct {
	PlanName string
}

// VersionsLoadedMsg is sent when versions are loaded.
type VersionsLoadedMsg struct {
	Versions []claudeviewer.PlanVersionDetail
}

// VersionErrorMsg is sent on error.
type VersionErrorMsg struct {
	Error error
}

// PopScreenMsg requests popping the current screen.
type PopScreenMsg struct{}

// RestoreVersionMsg requests restoring a version.
type RestoreVersionMsg struct {
	PlanName      string
	VersionNumber int64
}

// RestoreResultMsg is sent when restore completes.
type RestoreResultMsg struct {
	Success  bool
	PlanName string
	Error    error
}
