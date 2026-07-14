package screens

import (
	"fmt"
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/content"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/types"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// VersionsScreen handles version history browsing and viewing.
type VersionsScreen struct {
	// Components.
	list      *components.List
	viewer    *components.Viewer
	searchBar *components.SearchBar

	// State.
	layout      types.Layout
	focus       types.Focus
	planName    string
	searchQuery string
	versions    []claudeviewer.PlanVersionDetail
	current     *claudeviewer.PlanVersionDetail

	// Dimensions.
	width  int
	height int

	// Styles.
	borderStyle lipgloss.Style

	isDarkModeEnabled bool
}

// NewVersionsScreen creates a new versions screen.
func NewVersionsScreen(planName string, width, height int, isDarkModeEnabled, renderMarkdownByDefault bool) *VersionsScreen {
	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	v := &VersionsScreen{
		list:      components.NewList(nil, panelWidth, contentHeight, true),
		viewer:    components.NewViewer(panelWidth, contentHeight),
		searchBar: components.NewSearchBar(panelWidth),
		layout:    types.LayoutSplit,
		focus:     types.FocusList,
		planName:  planName,
		width:     width,
		height:    height,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
		isDarkModeEnabled: isDarkModeEnabled,
	}

	if renderMarkdownByDefault {
		v.viewer.SetRenderMode(components.RenderModeGlamour)
	}

	return v
}

// NewVersionsScreenWithData creates a versions screen with pre-loaded data.
func NewVersionsScreenWithData(planName string, versions []claudeviewer.PlanVersionDetail, width, height int, isDarkModeEnabled, renderMarkdownByDefault bool) *VersionsScreen {
	s := NewVersionsScreen(planName, width, height, isDarkModeEnabled, renderMarkdownByDefault)
	s.versions = versions
	s.updateListItems()
	if len(versions) > 0 {
		s.current = &s.versions[0]
		viewerWidth := s.getViewerWidth()
		s.viewer.SetContent(content.NewVersionContent(s.current, isDarkModeEnabled, s.focus, viewerWidth))
	}
	return s
}

// Init initializes the screen.
func (s *VersionsScreen) Init() tea.Cmd {
	// If versions are already loaded, no need to fetch.
	if len(s.versions) > 0 {
		return nil
	}
	return func() tea.Msg {
		return messages.LoadVersionsMsg{PlanName: s.planName}
	}
}

// Update handles messages.
func (s *VersionsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case messages.VersionsLoadedMsg:
		s.versions = msg.Versions
		s.updateListItems()
		if len(s.versions) > 0 {
			s.current = &s.versions[0]
			viewerWidth := s.getViewerWidth()
			s.viewer.SetContent(content.NewVersionContent(s.current, s.isDarkModeEnabled, s.focus, viewerWidth))
		}
		return s, nil

	case messages.ThemeChangedMsg:
		s.UpdateDarkMode(msg.DarkMode)
		return s, nil

	case messages.RenderMarkDownByDefaultMsg:
		s.RenderedMarkdownByDefault(msg.Enabled)
		return s, nil
	}

	// Update active component.
	var cmd tea.Cmd
	switch s.focus {
	case types.FocusList:
		cmd = s.list.Update(msg)
	case types.FocusContent:
		cmd = s.viewer.Update(msg)
	case types.FocusSearch:
		cmd = s.searchBar.Update(msg)
	default:
		return s, cmd
	}

	return s, cmd
}

// handleKey processes key input based on current focus.
func (s *VersionsScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()

	switch s.focus {
	case types.FocusList:
		return s.handleListKey(key, msg)
	case types.FocusContent:
		return s.handleContentKey(key, msg)
	case types.FocusSearch:
		return s.handleSearchKey(key, msg)
	default:
		return s, nil
	}
}

// handleListKey handles keys in list focus mode.
func (s *VersionsScreen) handleListKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "/":
		s.focus = types.FocusSearch
		return s, s.searchBar.Focus()
	case "tab":
		if s.current != nil {
			s.focus = types.FocusContent
		}
		return s, nil
	case "ctrl+l":
		// Clear search and reload all versions.
		if s.searchQuery != "" {
			s.searchQuery = ""
			s.searchBar.Reset()
			return s, func() tea.Msg {
				return messages.LoadVersionsMsg{PlanName: s.planName}
			}
		}
		return s, nil
	case "c":
		// Copy the selected version's content to the clipboard.
		if s.current != nil {
			return s, s.copyVersion()
		}
		return s, nil
	case "esc":
		return s, func() tea.Msg {
			return messages.PopScreenMsg{}
		}
	case "v":
		if s.current != nil {
			s.layout = types.LayoutFullscreen
			s.focus = types.FocusContent
			// Regenerate content with fullscreen width
			viewerWidth := s.getViewerWidth()
			s.viewer.SetContent(content.NewVersionContent(s.current, s.isDarkModeEnabled, s.focus, viewerWidth))
		}
		return s, nil
	case "R":
		// Restore selected version.
		if s.current != nil {
			return s, s.restoreVersion()
		}
		return s, nil
	case "r":
		s.viewer.ToggleRenderMode()
	case "j", "down", "k", "up":
		// Navigate list.
		cmd := s.list.Update(msg)
		// Update current version.
		if item := s.list.SelectedItem(); item != nil {
			if version, ok := item.Data().(claudeviewer.PlanVersionDetail); ok {
				s.current = &version
				viewerWidth := s.getViewerWidth()
				s.viewer.SetContent(content.NewVersionContent(s.current, s.isDarkModeEnabled, s.focus, viewerWidth))
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
		s.layout = types.LayoutSplit
		s.focus = types.FocusList
		s.viewer.GotoTop()
		// Regenerate content with split view width
		if s.current != nil {
			viewerWidth := s.getViewerWidth()
			s.viewer.SetContent(content.NewVersionContent(s.current, s.isDarkModeEnabled, s.focus, viewerWidth))
		}
		return s, nil
	case "tab":
		// Switch back to list panel (left side) in split view.
		if s.layout == types.LayoutSplit {
			s.focus = types.FocusList
			return s, nil
		}
		return s, nil
	case "R":
		// Restore selected version.
		if s.current != nil {
			return s, s.restoreVersion()
		}
		return s, nil
	case "r":
		s.viewer.ToggleRenderMode()
		return s, nil
	case "c":
		// Copy the selected version's content to the clipboard.
		if s.current != nil {
			return s, s.copyVersion()
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

// handleSearchKey handles keys in search mode.
func (s *VersionsScreen) handleSearchKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "esc":
		// Cancel search input, back to list.
		s.focus = types.FocusList
		s.searchBar.Blur()
		return s, nil

	case "enter":
		// Execute search.
		query := s.searchBar.Value()
		s.searchQuery = query
		s.focus = types.FocusList
		s.searchBar.Blur()
		return s, func() tea.Msg {
			return messages.SearchVersionsMsg{Query: query, PlanName: s.planName}
		}
	}

	// Pass other keys to search bar for input.
	return s, s.searchBar.Update(msg)
}

// View renders the screen.
func (s *VersionsScreen) View() string {
	var mainContent string

	switch s.layout {
	case types.LayoutFullscreen:
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

	// Adjust list height if search bar is active.
	listHeight := contentHeight - 4
	if s.searchBar.IsActive() {
		listHeight -= 3 // Make room for search bar.
	}

	// Update component sizes.
	s.list.SetSize(panelWidth-4, listHeight)
	s.viewer.SetSize(panelWidth-4, contentHeight-4)
	s.searchBar.SetWidth(panelWidth - 4)

	// Build left panel content.
	leftContent := s.list.View()
	if s.searchBar.IsActive() {
		leftContent = lipgloss.JoinVertical(lipgloss.Left, leftContent, s.searchBar.View())
	}

	leftPanel := s.borderStyle.
		Width(panelWidth).
		Height(contentHeight).
		Render(leftContent)

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
	var sb strings.Builder
	for range height {
		sb.WriteString("│\n")
	}
	return strings.TrimSuffix(sb.String(), "\n")
}

// SetSize updates screen dimensions.
func (s *VersionsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// UpdateDarkMode updates the dark mode setting and regenerates content.
func (s *VersionsScreen) UpdateDarkMode(enabled bool) {
	s.isDarkModeEnabled = enabled
	// Regenerate current content with new theme
	if s.current != nil {
		viewerWidth := s.getViewerWidth()
		s.viewer.SetContent(content.NewVersionContent(s.current, s.isDarkModeEnabled, s.focus, viewerWidth))
	}
}

// RenderedMarkdownByDefault upddates the renderned markdown by default mesasage.
func (s *VersionsScreen) RenderedMarkdownByDefault(enabled bool) {
	renderMode := components.RenderModeRaw
	if enabled {
		renderMode = components.RenderModeGlamour
	}

	if s.viewer.RenderMode() != renderMode {
		viewerWidth := s.getViewerWidth()
		s.viewer.SetRenderMode(renderMode)
		s.viewer.SetContent(content.NewVersionContent(s.current, s.isDarkModeEnabled, s.focus, viewerWidth))
	}
}

// getViewerWidth calculates the current viewer width based on layout.
func (s *VersionsScreen) getViewerWidth() int {
	if s.layout == types.LayoutFullscreen {
		// Fullscreen: full width minus border padding
		return s.width - 4
	}
	// Split view: half width minus divider and border padding
	panelWidth := (s.width - 3) / 2
	return panelWidth - 4
}

// ShortHelp returns key binding help.
func (s *VersionsScreen) ShortHelp() string {
	mode := "RAW"
	if s.viewer.RenderMode() == components.RenderModeGlamour {
		mode = "RENDERED"
	}
	switch s.focus {
	case types.FocusList:
		searchHelp := "/: search"
		if s.searchQuery != "" {
			searchHelp = fmt.Sprintf("/: search | ctrl+l: clear [%s]", s.searchQuery)
		}
		return fmt.Sprintf("down/up: navigate | g/G: top/bottom | tab: content | v: view | R: restore | r: render (%s) | c: copy | %s | esc: back | Versions: %d", mode, searchHelp, len(s.versions))
	case types.FocusContent:
		return fmt.Sprintf("down/up: scroll | g/G: top/bottom | tab: list | R: restore | r: render (%s) | c: copy | esc: back", mode)
	case types.FocusSearch:
		return "enter: search | esc: cancel"
	default:
		return ""
	}
}

// IsInputMode returns true when capturing text input.
func (s *VersionsScreen) IsInputMode() bool {
	return s.focus == types.FocusSearch
}

// EditorMode ...
func (s *VersionsScreen) EditorMode() bool {
	return s.focus == types.FocusEditor
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
		return messages.RestoreVersionMsg{
			PlanName:      s.planName,
			VersionNumber: s.current.VersionNumber,
		}
	}
}

func (s *VersionsScreen) copyVersion() tea.Cmd {
	return func() tea.Msg {
		return messages.CopyToClipboardMsg{
			Text:  s.current.Content,
			Label: fmt.Sprintf("Version %d", s.current.VersionNumber),
		}
	}
}

// Message types for versions screen.
