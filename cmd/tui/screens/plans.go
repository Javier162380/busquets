package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/content"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/types"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PlansScreen handles plan browsing, viewing, and editing.
type PlansScreen struct {
	// Components.
	list      *components.List
	viewer    *components.Viewer
	editor    *components.Editor
	searchBar *components.SearchBar
	tagModal  *components.TagModal
	tagFilter *components.TagFilter

	// State.
	layout       types.Layout
	focus        types.Focus
	plans        []claudeviewer.PlanSummary
	current      *claudeviewer.PlanDetail
	searchQuery  string   // Current active search query (empty = show all).
	tagFilters   []string // Active tag filters.
	showingModal bool     // Whether tag modal is shown.

	// Dimensions.
	width  int
	height int

	// Styles.
	borderStyle lipgloss.Style

	// Theme
	isDarkModeEnabled bool
}

// NewPlansScreen creates a new plans screen.
func NewPlansScreen(width, height int, isDarkModeEnabled bool) *PlansScreen {
	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	return &PlansScreen{
		list:      components.NewList(nil, panelWidth, contentHeight),
		viewer:    components.NewViewer(panelWidth, contentHeight),
		editor:    components.NewEditor(width-4, contentHeight),
		searchBar: components.NewSearchBar(panelWidth),
		tagModal:  components.NewTagModal(),
		tagFilter: components.NewTagFilter(panelWidth),
		layout:    types.LayoutSplit,
		focus:     types.FocusList,
		width:     width,
		height:    height,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
		isDarkModeEnabled: isDarkModeEnabled,
	}
}

// Init initializes the screen.
func (s *PlansScreen) Init() tea.Cmd {
	return nil // Plans loaded via PlansLoadedMsg from App.
}

// Update handles messages.
func (s *PlansScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	// Handle tag modal if showing.
	if s.showingModal {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			cmd := s.tagModal.Update(msg)
			return s, cmd
		case components.SavePlanTagsMsg:
			s.showingModal = false
			return s, func() tea.Msg {
				return SavePlanTagsMsg{
					FileName: msg.FileName,
					Tags:     msg.Tags,
				}
			}
		}
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case PlansLoadedMsg:
		s.plans = msg.Plans
		s.updateListItems()
		if len(s.plans) > 0 {
			return s, s.loadPlanDetail(s.plans[0].FileName)
		}
		return s, nil

	case PlanDetailLoadedMsg:
		s.current = msg.Detail
		viewerWidth := s.getViewerWidth()
		s.viewer.SetContent(content.NewPlanContent(msg.Detail, s.isDarkModeEnabled, s.focus, viewerWidth))
		s.editor.SetContent(msg.Detail.Content)
		return s, nil

	case SaveResultMsg:
		if msg.Error == nil && msg.Result.Success && !msg.Result.HasConflict {
			s.focus = types.FocusContent
			s.editor.Blur()
			// Reload the plan.
			if s.current != nil {
				return s, s.loadPlanDetail(s.current.FileName)
			}
		}
		return s, nil

	case OpenTagModalMsg:
		// Load all tags and current plan tags, then open modal.
		return s, func() tea.Msg {
			return LoadTagsForModalMsg{FileName: msg.FileName}
		}

	case TagsLoadedMsg:
		// Open modal with loaded tags.
		s.tagModal.Open(msg.FileName, msg.PlanTags, msg.AllTags)
		s.showingModal = true
		return s, nil

	case SavePlanTagsMsg:
		// Save tags via service layer.
		return s, func() tea.Msg {
			return SetPlanTagsMsg{
				FileName: msg.FileName,
				Tags:     msg.Tags,
			}
		}

	case FilterByTagsMsg:
		// Apply tag filter.
		return s, func() tea.Msg {
			return SearchPlansWithTagsMsg{
				Query:    s.searchQuery,
				Tags:     msg.Tags,
				MatchAll: msg.MatchAll,
			}
		}
	}

	// Update active component.
	var cmd tea.Cmd
	switch s.focus {
	case types.FocusList:
		cmd = s.list.Update(msg)
	case types.FocusContent:
		cmd = s.viewer.Update(msg)
	case types.FocusEditor:
		cmd = s.editor.Update(msg)
	default:
		return s, cmd
	}
	return s, cmd
}

// handleKey processes key input based on current focus.
func (s *PlansScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()

	switch s.focus {
	case types.FocusList:
		return s.handleListKey(key, msg)
	case types.FocusContent:
		return s.handleContentKey(key, msg)
	case types.FocusEditor:
		return s.handleEditorKey(key, msg)
	case types.FocusSearch:
		return s.handleSearchKey(key, msg)
	case types.FocusTagFilter:
		return s.handleTagFilterKey(key, msg)
	}

	return s, nil
}

// handleListKey handles keys in list focus mode.
func (s *PlansScreen) handleListKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "/":
		s.focus = types.FocusSearch
		return s, s.searchBar.Focus()

	case "T":
		// Open tag filter.
		s.focus = types.FocusTagFilter
		return s, s.tagFilter.Focus()

	case "m":
		// Open tag modal for current plan.
		if s.current != nil {
			return s, func() tea.Msg {
				return OpenTagModalMsg{FileName: s.current.FileName}
			}
		}
		return s, nil

	case "c":
		// Clear search or tag filter.
		if s.searchQuery != "" || len(s.tagFilters) > 0 {
			s.searchQuery = ""
			s.tagFilters = nil
			s.searchBar.Reset()
			s.tagFilter.Reset()
			return s, func() tea.Msg {
				return ClearSearchMsg{}
			}
		}
		return s, nil

	case "v":
		if s.current != nil {
			s.layout = types.LayoutFullscreen
			s.focus = types.FocusContent
			// Regenerate content with fullscreen width
			viewerWidth := s.getViewerWidth()
			s.viewer.SetContent(content.NewPlanContent(s.current, s.isDarkModeEnabled, s.focus, viewerWidth))
		}
		return s, nil

	case "e":
		if s.current != nil {
			s.layout = types.LayoutFullscreen
			s.focus = types.FocusEditor
			return s, s.editor.Focus()
		}
		return s, nil

	case "s":
		return s, s.syncPlans()

	case "r":
		return s, s.rsyncPlans()

	case "S":
		return s, func() tea.Msg {
			return OpenSettingsMsg{}
		}

	case "C":
		return s, func() tea.Msg {
			return OpenConnectorsMsg{}
		}

	case "tab":
		if s.current != nil {
			s.focus = types.FocusContent
		}
		return s, nil

	case "j", "down", "k", "up":
		cmd := s.list.Update(msg)
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
	case "tab":
		// Switch back to list panel (left side) in split view.
		if s.layout == types.LayoutSplit {
			s.focus = types.FocusList
			return s, nil
		}
		return s, nil

	case "esc":
		// Back to list (split view) or back to split view (fullscreen).
		if s.layout == types.LayoutFullscreen {
			s.layout = types.LayoutSplit
			s.focus = types.FocusList
			s.viewer.GotoTop()
			// Regenerate content with split view width
			if s.current != nil {
				viewerWidth := s.getViewerWidth()
				s.viewer.SetContent(content.NewPlanContent(s.current, s.isDarkModeEnabled, s.focus, viewerWidth))
			}
		} else {
			s.focus = types.FocusList
		}
		return s, nil

	case "e":
		// Enter edit mode.
		if s.current != nil {
			s.focus = types.FocusEditor
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
			return s, func() tea.Msg {
				return RequestVersionsScreenMsg{PlanName: s.current.FileName}
			}
		}
		return s, nil

	case "t":
		// Transmit to connector.
		if s.current != nil {
			return s, func() tea.Msg {
				return SendToConnectorMsg{PlanFileName: s.current.FileName}
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
		s.focus = types.FocusContent
		s.editor.Blur()
		s.editor.Reset()
		return s, nil

	case "ctrl+s":
		// Save.
		if s.current != nil {
			return s, s.savePlan()
		}
		return s, nil
	}

	return s, s.editor.Update(msg)
}

// handleSearchKey handles keys in search mode.
func (s *PlansScreen) handleSearchKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
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
			return SearchPlansMsg{Query: query}
		}
	}

	// Pass other keys to search bar for input.
	return s, s.searchBar.Update(msg)
}

// handleTagFilterKey handles keys in tag filter mode.
func (s *PlansScreen) handleTagFilterKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "esc":
		// Cancel tag filter input, back to list.
		s.focus = types.FocusList
		s.tagFilter.Blur()
		return s, nil

	case "enter":
		// Apply tag filter.
		tags := s.tagFilter.Tags()
		s.tagFilters = tags
		s.focus = types.FocusList
		s.tagFilter.Blur()
		return s, func() tea.Msg {
			return FilterByTagsMsg{
				Tags:     tags,
				MatchAll: s.tagFilter.MatchAll(),
			}
		}

	case "ctrl+t":
		// Toggle AND/OR mode.
		s.tagFilter.ToggleMatchMode()
		return s, nil
	}

	// Pass other keys to tag filter for input.
	return s, s.tagFilter.Update(msg)
}

// View renders the screen.
func (s *PlansScreen) View() string {
	var mainContent string

	switch s.layout {
	case types.LayoutFullscreen:
		if s.focus == types.FocusEditor {
			mainContent = s.renderFullscreenEditor()
		} else {
			mainContent = s.renderFullscreenViewer()
		}
	default:
		mainContent = s.renderSplitView()
	}

	// Overlay tag modal if showing.
	if s.showingModal {
		modal := s.tagModal.View()

		// Overlay modal on top of main content by placing it centered
		overlay := lipgloss.Place(
			s.width,
			s.height,
			lipgloss.Center,
			lipgloss.Center,
			modal,
		)

		// Combine main content with overlay
		return s.overlayContent(mainContent, overlay)
	}

	return mainContent
}

// renderSplitView renders the two-panel layout.
func (s *PlansScreen) renderSplitView() string {
	panelWidth := (s.width - 3) / 2
	contentHeight := s.height - 4

	// Adjust list height if search bar or tag filter is active.
	listHeight := contentHeight - 4
	if s.searchBar.IsActive() {
		listHeight -= 3 // Make room for search bar.
	}
	if s.tagFilter.IsActive() {
		listHeight -= 3 // Make room for tag filter.
	}

	// Update component sizes.
	s.list.SetSize(panelWidth-4, listHeight)
	s.viewer.SetSize(panelWidth-4, contentHeight-4)
	s.searchBar.SetWidth(panelWidth - 4)
	s.tagFilter.SetWidth(panelWidth - 4)

	// Build left panel content.
	leftContent := s.list.View()
	if s.searchBar.IsActive() {
		leftContent = lipgloss.JoinVertical(lipgloss.Left, leftContent, s.searchBar.View())
	}
	if s.tagFilter.IsActive() {
		leftContent = lipgloss.JoinVertical(lipgloss.Left, leftContent, s.tagFilter.View())
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
}

// UpdateDarkMode updates the dark mode setting and regenerates content.
func (s *PlansScreen) UpdateDarkMode(enabled bool) {
	s.isDarkModeEnabled = enabled
	// Regenerate current content with new theme
	if s.current != nil {
		viewerWidth := s.getViewerWidth()
		s.viewer.SetContent(content.NewPlanContent(s.current, s.isDarkModeEnabled, s.focus, viewerWidth))
	}
}

// getViewerWidth calculates the current viewer width based on layout.
func (s *PlansScreen) getViewerWidth() int {
	if s.layout == types.LayoutFullscreen {
		// Fullscreen: full width minus border padding
		return s.width - 4
	}
	// Split view: half width minus divider and border padding
	panelWidth := (s.width - 3) / 2
	return panelWidth - 4
}

// ShortHelp returns key binding help.
func (s *PlansScreen) ShortHelp() string {
	switch s.focus {
	case types.FocusList:
		searchHelp := "/: search | T: tags"
		if s.searchQuery != "" || len(s.tagFilters) > 0 {
			activeFilters := ""
			if s.searchQuery != "" {
				activeFilters = s.searchQuery
			}
			if len(s.tagFilters) > 0 {
				tagStr := fmt.Sprintf("tags:%v", s.tagFilters)
				if activeFilters != "" {
					activeFilters += " | " + tagStr
				} else {
					activeFilters = tagStr
				}
			}
			searchHelp = fmt.Sprintf("/: search | T: tags | c: clear [%s]", activeFilters)
		}
		return fmt.Sprintf("j/k: navigate | m: manage tags | tab: content | v: fullscreen | e: edit | s: sync | S: settings | r: rsync | C: connectors | %s | Plans: %d", searchHelp, len(s.plans))
	case types.FocusContent:
		mode := "RAW"
		if s.viewer.RenderMode() == components.RenderModeGlamour {
			mode = "RENDERED"
		}
		if s.layout == types.LayoutSplit {
			return fmt.Sprintf("j/k: scroll | g/G: top/bottom | r: render (%s)	 | tab: list | esc: back", mode)
		}
		return fmt.Sprintf("j/k: scroll | g/G: top/bottom | r: render (%s) | e: edit | v: versions | t: transmit | esc: back", mode)
	case types.FocusEditor:
		modified := ""
		if s.editor.IsModified() {
			modified = " [MODIFIED]"
		}
		return fmt.Sprintf("ctrl+s: save | esc: cancel%s", modified)
	case types.FocusSearch:
		return "enter: search | esc: cancel"
	case types.FocusTagFilter:
		mode := "OR"
		if s.tagFilter.MatchAll() {
			mode = "AND"
		}
		return fmt.Sprintf("enter: filter | ctrl+t: toggle mode (%s) | esc: cancel", mode)
	}
	return ""
}

// IsInputMode returns true when capturing text input.
func (s *PlansScreen) IsInputMode() bool {
	return s.focus == types.FocusEditor || s.focus == types.FocusSearch || s.focus == types.FocusTagFilter || s.showingModal
}

// overlayContent overlays the modal on top of the main content.
func (s *PlansScreen) overlayContent(base, overlay string) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure both have the same number of lines
	maxLines := len(baseLines)
	if len(overlayLines) > maxLines {
		maxLines = len(overlayLines)
	}

	result := make([]string, maxLines)
	for i := 0; i < maxLines; i++ {
		var baseLine, overlayLine string
		if i < len(baseLines) {
			baseLine = baseLines[i]
		}
		if i < len(overlayLines) {
			overlayLine = overlayLines[i]
		}

		// If overlay line is not empty/whitespace, use it; otherwise use base
		if strings.TrimSpace(overlayLine) != "" {
			result[i] = overlayLine
		} else {
			result[i] = baseLine
		}
	}

	return strings.Join(result, "\n")
}

// updateListItems updates the list with current plans.
func (s *PlansScreen) updateListItems() {
	items := make([]components.ListItem, len(s.plans))
	for i, plan := range s.plans {
		// Build description with tags.
		desc := fmt.Sprintf("%s | %d min read", plan.ModifiedAt.Format("2006-01-02"), plan.ReadingTime)
		if len(plan.Tags) > 0 {
			tagNames := make([]string, len(plan.Tags))
			for j, tag := range plan.Tags {
				tagNames[j] = tag.Name
			}
			desc += fmt.Sprintf(" | [%s]", strings.Join(tagNames, ", "))
		}
		items[i] = components.NewListItem(
			plan.Title,
			desc,
			plan,
		)
	}
	s.list.SetItems(items)
}

// Command helpers.

func (s *PlansScreen) loadPlanDetail(fileName string) tea.Cmd {
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

func (s *PlansScreen) rsyncPlans() tea.Cmd {
	return func() tea.Msg {
		return RSyncPlansMsg{}
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

// RSyncPlansMsg request resync plans from the viewer directory back into the LLM directory.
type RSyncPlansMsg struct{}

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

// RSyncResultMsg is sent when rsync completes.
type RSyncResultMsg struct {
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

// SearchPlansMsg requests searching plans.
type SearchPlansMsg struct {
	PlanName string
	Query    string
}

// ClearSearchMsg requests clearing search and loading all plans.
type ClearSearchMsg struct{}

// SendToConnectorMsg requests sending current plan to connector.
type SendToConnectorMsg struct {
	PlanFileName string
}

// SendToConnectorResultMsg is the result of sending to connector.
type SendToConnectorResultMsg struct {
	Success bool
	Error   error
}

// OpenTagModalMsg requests opening the tag modal for a plan.
type OpenTagModalMsg struct {
	FileName string
}

// LoadTagsForModalMsg requests loading tags for the modal.
type LoadTagsForModalMsg struct {
	FileName string
}

// TagsLoadedMsg contains loaded tags for the modal.
type TagsLoadedMsg struct {
	FileName string
	PlanTags []dto.Tag
	AllTags  []dto.Tag
}

// SavePlanTagsMsg requests saving tags for a plan.
type SavePlanTagsMsg struct {
	FileName string
	Tags     []string
}

// SetPlanTagsMsg requests setting tags via service layer.
type SetPlanTagsMsg struct {
	FileName string
	Tags     []string
}

// FilterByTagsMsg requests filtering plans by tags.
type FilterByTagsMsg struct {
	Tags     []string
	MatchAll bool
}

// SearchPlansWithTagsMsg requests searching plans with tag filter.
type SearchPlansWithTagsMsg struct {
	Query    string
	Tags     []string
	MatchAll bool
}
