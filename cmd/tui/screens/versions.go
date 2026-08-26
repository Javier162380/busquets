package screens

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Javier162380/busquets/cmd/tui/components"
	"github.com/Javier162380/busquets/cmd/tui/content"
	"github.com/Javier162380/busquets/cmd/tui/messages"
	"github.com/Javier162380/busquets/cmd/tui/styles"
	"github.com/Javier162380/busquets/cmd/tui/types"
	"github.com/Javier162380/busquets/services/busquets"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/pmezard/go-difflib/difflib"
)

// VersionsScreen handles version history browsing and viewing.
type VersionsScreen struct {
	// Components.
	list               *components.List
	viewer             *components.Viewer
	searchBar          *components.SearchBar
	contentSearchModal *components.InputModal[string]
	inputModal         *components.InputModal[int]

	// State.
	layout             types.Layout
	focus              types.Focus
	activeModal        types.ModalState
	planID             int64
	planName           string
	searchQuery        string
	versions           []busquets.PlanVersionDetail
	current            *busquets.PlanVersionDetail
	pendingSearchError error // Set by the content-search onSubmit callback when a query has no matches; consumed by handleContentSearchModalUpdate.

	// Diff state.
	baseVersion  *busquets.PlanVersionDetail // marked diff base, nil if none marked
	diffViewport viewport.Model              // scrollable rendered diff (colored)
	diffPlain    string                      // same diff, no ANSI — used for clipboard copy

	// Dimensions.
	width  int
	height int

	// Styles.
	borderStyle lipgloss.Style

	isDarkModeEnabled bool
}

// NewVersionsScreen creates a new versions screen.
func NewVersionsScreen(planID int64, planName, markdownRenderedTheme string, width, height int, isDarkModeEnabled, renderMarkdownByDefault bool) *VersionsScreen {
	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	v := &VersionsScreen{
		list:               components.NewList(nil, panelWidth, contentHeight, true),
		viewer:             components.NewViewer(panelWidth, contentHeight, markdownRenderedTheme),
		searchBar:          components.NewSearchBar(panelWidth),
		contentSearchModal: components.NewInputModal[string](),
		inputModal:         components.NewInputModal[int](),
		layout:             types.LayoutSplit,
		focus:              types.FocusList,
		planID:             planID,
		planName:           planName,
		width:              width,
		height:             height,
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
func NewVersionsScreenWithData(planID int64, planName, markdownRenderedTheme string, versions []busquets.PlanVersionDetail, width, height int, isDarkModeEnabled, renderMarkdownByDefault bool) *VersionsScreen {
	s := NewVersionsScreen(planID, planName, markdownRenderedTheme, width, height, isDarkModeEnabled, renderMarkdownByDefault)
	s.versions = versions
	s.updateListItems()
	if len(versions) > 0 {
		s.current = &s.versions[0]
		viewerWidth := s.getViewerWidth()
		s.viewer.SetContent(content.NewVersionContent(s.current, viewerWidth))
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
	switch s.activeModal {
	case types.ModalContentSearch:
		return s.handleContentSearchModalUpdate(msg)
	case types.ModalTextInput:
		return s.handleInputModalUpdate(msg)
	default:
		// ModalNone (the only other value VersionsScreen ever sets): handle normally below
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case messages.VersionsLoadedMsg:
		s.versions = msg.Versions
		s.updateListItems()
		if len(s.versions) > 0 {
			s.current = &s.versions[0]
			viewerWidth := s.getViewerWidth()
			s.viewer.SetContent(content.NewVersionContent(s.current, viewerWidth))
		}
		return s, nil

	case messages.ThemeChangedMsg:
		s.UpdateDarkMode(msg.DarkMode)
		return s, nil

	case messages.RenderMarkDownByDefaultMsg:
		s.RenderedMarkdownByDefault(msg.Enabled)
		return s, nil

	case messages.MarkdownRenderedThemeChangedMsg:
		s.RenderedMarkdownTheme(msg.Theme)
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
	case types.FocusDiff:
		return s.handleDiffKey(key, msg)
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
			s.viewer.SetContent(content.NewVersionContent(s.current, viewerWidth))
		}
		return s, nil
	case "R":
		// Restore selected version.
		if s.current != nil {
			return s, s.restoreVersion()
		}
		return s, nil
	case "m":
		s.toggleBaseVersion()
		return s, nil
	case "d":
		return s, s.diffAgainstBase()
	case "r":
		s.viewer.ToggleRenderMode()
	case "l":
		s.viewer.ToggleLineNumbers()
	case "j", "down", "k", "up":
		// Navigate list.
		cmd := s.list.Update(msg)
		// Update current version.
		if item := s.list.SelectedItem(); item != nil {
			if version, ok := item.Data().(busquets.PlanVersionDetail); ok {
				s.current = &version
				viewerWidth := s.getViewerWidth()
				s.viewer.SetContent(content.NewVersionContent(s.current, viewerWidth))
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
		s.viewer.ClearSearch()
		s.layout = types.LayoutSplit
		s.focus = types.FocusList
		s.viewer.GotoTop()
		// Regenerate content with split view width
		if s.current != nil {
			viewerWidth := s.getViewerWidth()
			s.viewer.SetContent(content.NewVersionContent(s.current, viewerWidth))
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
	case "l":
		s.viewer.ToggleLineNumbers()
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

	case "/":
		// Search within the currently displayed version content. Uses N/P
		// rather than vim's n/N for consistency with PlansScreen's content
		// search, where lowercase n is already bound to "new comment".
		// Fullscreen only: split view keeps the content pane visible after
		// focus moves to the list, and only Esc-from-fullscreen clears the
		// search, so a split-view search would leave stale highlights
		// showing behind the list.
		if s.layout != types.LayoutFullscreen {
			return s, nil
		}
		s.activeModal = types.ModalContentSearch
		s.contentSearchModal.SetSize(min(40, s.width-4), min(8, s.height-2))
		s.contentSearchModal.Open(
			"Search content", "text to find", nil,
			func(raw string) (string, error) { return raw, nil },
			func(query string) {
				s.viewer.Search(query)
				if !s.viewer.HasMatches() {
					s.pendingSearchError = fmt.Errorf("no matches for %q", query)
				}
			},
		)
		return s, nil

	case "N":
		if s.layout != types.LayoutFullscreen {
			return s, nil
		}
		s.viewer.SearchNext()
		return s, nil

	case "P":
		if s.layout != types.LayoutFullscreen {
			return s, nil
		}
		s.viewer.SearchPrev()
		return s, nil

	case "ctrl+u":
		if s.layout != types.LayoutFullscreen {
			return s, nil
		}
		s.viewer.ClearSearch()
		return s, nil

	case "ctrl+l":
		// Jump to a specific line, same pattern as PlansScreen's viewer
		// go-to-line. Available regardless of layout — a jump has no
		// highlight state that could linger visibly in split view.
		maxLine := s.viewer.LineCount()
		s.activeModal = types.ModalTextInput
		s.inputModal.SetSize(min(40, s.width-4), min(8, s.height-2))
		s.inputModal.Open(
			fmt.Sprintf("Go to line (1-%d)", maxLine),
			"line number",
			func(r rune) bool { return r >= '0' && r <= '9' },
			strconv.Atoi,
			func(n int) {
				if n > 0 {
					s.viewer.ScrollToContentLine(n)
				}
			},
		)
		return s, nil
	}

	return s, s.viewer.Update(msg)
}

// handleDiffKey handles keys while the fullscreen diff view (FocusDiff) is
// showing, reached from FocusList via "d". Marking a base and diffing are
// list-only actions — the content view is for reading one version at a
// time, not comparing two.
func (s *VersionsScreen) handleDiffKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "esc":
		s.layout = types.LayoutSplit
		s.focus = types.FocusList
		return s, nil
	case "g":
		s.diffViewport.GotoTop()
		return s, nil
	case "G":
		s.diffViewport.GotoBottom()
		return s, nil
	case "c":
		if s.baseVersion != nil && s.current != nil {
			return s, s.copyDiff()
		}
		return s, nil
	}

	var cmd tea.Cmd
	s.diffViewport, cmd = s.diffViewport.Update(msg)
	return s, cmd
}

// handleContentSearchModalUpdate routes messages while the content search
// modal is active. Mirrors PlansScreen.handleContentSearchModalUpdate:
// Esc is intercepted before delegating to the modal so it also clears the
// viewer's active search highlight, since InputModal.Close() only closes
// the input box itself.
func (s *VersionsScreen) handleContentSearchModalUpdate(msg tea.Msg) (Screen, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.Type == tea.KeyEsc {
		s.viewer.ClearSearch()
	}

	cmd := s.contentSearchModal.Update(msg)
	if !s.contentSearchModal.IsActive() {
		s.activeModal = types.ModalNone
	}

	if s.pendingSearchError != nil {
		err := s.pendingSearchError
		s.pendingSearchError = nil
		return s, tea.Batch(cmd, func() tea.Msg { return messages.ContentSearchErrorMsg{Error: err} })
	}
	return s, cmd
}

// handleInputModalUpdate routes messages while the generic int-input modal
// (currently just go-to-line) is active. Mirrors PlansScreen's own
// handleInputModalUpdate.
func (s *VersionsScreen) handleInputModalUpdate(msg tea.Msg) (Screen, tea.Cmd) {
	cmd := s.inputModal.Update(msg)
	if !s.inputModal.IsActive() {
		s.activeModal = types.ModalNone
	}
	return s, cmd
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
	if s.focus == types.FocusDiff {
		return s.renderDiffView()
	}

	var mainContent string

	switch s.layout {
	case types.LayoutFullscreen:
		mainContent = s.renderFullscreenViewer()
	default:
		mainContent = s.renderSplitView()
	}

	switch s.activeModal {
	case types.ModalContentSearch:
		// height-1, not height: App.View() appends a status-bar row below
		// this screen's own View() output, so a Top-aligned overlay placed
		// against the full height overflows the terminal by one row —
		// scrolling the box's own top border (row 0) off screen.
		overlay := lipgloss.Place(s.width, s.height-1, lipgloss.Left, lipgloss.Top, s.contentSearchModal.View())
		return overlayContent(mainContent, overlay)

	case types.ModalTextInput:
		overlay := lipgloss.Place(s.width, s.height, lipgloss.Center, lipgloss.Center, s.inputModal.View())
		return overlayContent(mainContent, overlay)
	default:
		return mainContent
	}
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

// renderDiffView renders the fullscreen unified-diff view. Mirrors
// renderFullscreenViewer's sizing so the diff occupies the same footprint as
// a fullscreen version view.
func (s *VersionsScreen) renderDiffView() string {
	contentHeight := s.height - 4
	s.diffViewport.Width = s.width - 4
	s.diffViewport.Height = contentHeight - 4

	return s.borderStyle.
		Width(s.width).
		Height(contentHeight).
		Render(s.diffViewport.View())
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
		s.viewer.SetContent(content.NewVersionContent(s.current, viewerWidth))
	}
}

// RenderedMarkdownByDefault upddates the renderned markdown by default mesasage.
func (s *VersionsScreen) RenderedMarkdownByDefault(enabled bool) {
	renderMode := components.RenderModeRaw
	if enabled {
		renderMode = components.RenderModeGlamour
	}

	if s.viewer.RenderMode() != renderMode {
		s.viewer.SetRenderMode(renderMode)
		if s.current != nil {
			s.viewer.SetContent(content.NewVersionContent(s.current, s.getViewerWidth()))
		}
	}
}

// RenderedMarkdownTheme modifies a markdown rendered theme.
func (s *VersionsScreen) RenderedMarkdownTheme(theme string) {
	s.viewer.SetMarkdownTheme(theme)
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
	if s.focus == types.FocusDiff {
		return "j/k, ↑/↓: scroll | g/G: top/bottom | c: copy diff | esc: back"
	}

	mode := "RAW"
	if s.viewer.RenderMode() == components.RenderModeGlamour {
		mode = "RENDERED"
	}
	lines := "OFF"
	if s.viewer.ShowLineNumbers() {
		lines = "ON"
	}
	switch s.focus {
	case types.FocusList:
		searchHelp := "/: search"
		if s.searchQuery != "" {
			searchHelp = fmt.Sprintf("/: search | ctrl+l: clear [%s]", s.searchQuery)
		}
		baseHelp := "m: mark base"
		if s.baseVersion != nil {
			baseHelp = fmt.Sprintf("m: unmark base [v%d] | d: diff vs base", s.baseVersion.VersionNumber)
		}
		return fmt.Sprintf("down/up: navigate | g/G: top/bottom | tab: content | v: view | R: restore | %s | r: render (%s) | l: lines (%s) | c: copy | %s | esc: back | Versions: %d", baseHelp, mode, lines, searchHelp, len(s.versions))
	case types.FocusContent:
		if s.layout != types.LayoutFullscreen {
			return fmt.Sprintf("down/up: scroll | g/G: top/bottom | tab: list | R: restore | r: render (%s) | l: lines (%s) | c: copy | ctrl+l: go to line | esc: back", mode, lines)
		}
		contentSearchHelp := "/: search"
		if query := s.viewer.SearchQuery(); query != "" {
			current, total := s.viewer.SearchStatus()
			contentSearchHelp = fmt.Sprintf("/: search | N/P: next/prev match | ctrl+u: clear [%s] | Hits %d/%d", query, current, total)
		}
		return fmt.Sprintf("down/up: scroll | g/G: top/bottom | R: restore | r: render (%s) | l: lines (%s) | c: copy | ctrl+l: go to line | %s | esc: back", mode, lines, contentSearchHelp)
	case types.FocusSearch:
		return "enter: search | esc: cancel"
	default:
		return ""
	}
}

// IsInputMode returns true when capturing text input.
func (s *VersionsScreen) IsInputMode() bool {
	return s.focus == types.FocusSearch || s.activeModal != types.ModalNone
}

// EditorMode ...
func (s *VersionsScreen) EditorMode() bool {
	return s.focus == types.FocusEditor
}

// updateListItems updates the list with current versions.
func (s *VersionsScreen) updateListItems() {
	items := make([]components.ListItem, len(s.versions))
	for i, version := range s.versions {
		title := fmt.Sprintf("Version %d", version.VersionNumber)
		if s.baseVersion != nil && s.baseVersion.VersionNumber == version.VersionNumber {
			title = "● " + title
		}
		items[i] = components.NewListItem(
			title,
			fmt.Sprintf("%s | %d min read | Path: %s", version.CreatedAt.Format("2006-01-02 15:04"), version.ReadingTime, version.PlanVersion.FilePath),
			version,
		)
	}
	s.list.SetItems(items)
}

// Command helpers.

func (s *VersionsScreen) restoreVersion() tea.Cmd {
	return func() tea.Msg {
		return messages.RestoreVersionMsg{
			PlanID:        s.planID,
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

// toggleBaseVersion marks s.current as the diff base, moves the mark to it
// if a different version was already marked, or unmarks it if it's already
// the base. No-op when nothing is selected.
func (s *VersionsScreen) toggleBaseVersion() {
	if s.current == nil {
		return
	}
	if s.baseVersion != nil && s.baseVersion.VersionNumber == s.current.VersionNumber {
		s.baseVersion = nil
	} else {
		base := *s.current
		s.baseVersion = &base
	}
	s.updateListItems()
}

// diffAgainstBase builds a unified diff of the marked base version against
// s.current and switches into the fullscreen diff view. Returns an
// messages.ErrorMsg command (routed to the status bar by App) when no base
// is marked or the base and current version are the same.
func (s *VersionsScreen) diffAgainstBase() tea.Cmd {
	if s.baseVersion == nil {
		return func() tea.Msg {
			return messages.ErrorMsg{Error: fmt.Errorf("mark a base version first (m)")}
		}
	}
	if s.current == nil {
		return nil
	}
	if s.baseVersion.VersionNumber == s.current.VersionNumber {
		return func() tea.Msg {
			return messages.ErrorMsg{Error: fmt.Errorf("select a different version to diff against the base")}
		}
	}

	diffText, err := buildUnifiedDiff(s.baseVersion, s.current)
	if err != nil {
		return func() tea.Msg {
			return messages.ErrorMsg{Error: fmt.Errorf("failed to build diff: %w", err)}
		}
	}
	s.diffPlain = diffText
	s.diffViewport = viewport.New(s.width-4, s.height-8)
	s.diffViewport.SetContent(colorizeDiff(s.diffPlain))
	s.focus = types.FocusDiff
	return nil
}

func (s *VersionsScreen) copyDiff() tea.Cmd {
	return func() tea.Msg {
		return messages.CopyToClipboardMsg{
			Text:  s.diffPlain,
			Label: fmt.Sprintf("Diff v%d→v%d", s.baseVersion.VersionNumber, s.current.VersionNumber),
		}
	}
}

// buildUnifiedDiff renders a git-diff-style unified diff of base's content
// against target's.
func buildUnifiedDiff(base, target *busquets.PlanVersionDetail) (string, error) {
	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(base.Content),
		B:        difflib.SplitLines(target.Content),
		FromFile: fmt.Sprintf("Version %d", base.VersionNumber),
		ToFile:   fmt.Sprintf("Version %d", target.VersionNumber),
		Context:  3,
	}
	return difflib.GetUnifiedDiffString(ud)
}

// colorizeDiff applies git-diff-style coloring to a unified diff: green
// additions, red removals, bold file headers, accent-colored hunk headers.
func colorizeDiff(diff string) string {
	rawLines := strings.Split(diff, "\n")
	lines := make([]string, len(rawLines))
	for i, line := range rawLines {
		switch {
		case strings.HasPrefix(line, "+++"), strings.HasPrefix(line, "---"):
			lines[i] = styles.DiffHeaderStyle.Render(line)
		case strings.HasPrefix(line, "@@"):
			lines[i] = styles.DiffHunkStyle.Render(line)
		case strings.HasPrefix(line, "+"):
			lines[i] = styles.DiffAddStyle.Render(line)
		case strings.HasPrefix(line, "-"):
			lines[i] = styles.DiffRemoveStyle.Render(line)
		default:
			lines[i] = line
		}
	}
	return strings.Join(lines, "\n")
}

// Message types for versions screen.
