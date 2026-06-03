package screens

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/content"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/types"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PlansScreen handles plan browsing, viewing, and editing.
type PlansScreen struct {
	// Components.
	list          *components.List
	viewer        *components.Viewer
	editor        *components.Editor
	searchBar     *components.SearchBar
	tagModal      *components.TagModal
	confirmDialog *components.ConfirmModal
	tagFilter     *components.TagFilter
	tagPanel      *components.TagPanel
	commentModal  *components.CommentModal

	// State.
	layout              types.Layout
	focus               types.Focus
	plans               []claudeviewer.PlanSummary            // filtered view shown in list
	allPlans            []claudeviewer.PlanSummary            // full unfiltered source of truth
	allTags             []claudeviewer.Tag                    // all tags in the system (including unassigned)
	tagPlanCounts       map[string]int                        // authoritative plan count per tag from DB
	tagPlanMap          map[string][]claudeviewer.PlanSummary // map tags to a planSummary
	untaggedCount       int                                   // number of plans with no tags assigned
	displayMode         string                                // one of DisplayModePlanContent, DisplayModeTagPlanContent
	current             *claudeviewer.PlanDetail
	searchQuery         string   // Current active search query (empty = show all).
	tagFilters          []string // Active tag filters.
	showingModal        bool     // Whether tag modal is shown.
	showingCommentModal bool     // Whether comment modal is shown.
	showingTLDR         bool     // Whether TLDR popup is shown.
	tldrTitle           string   // Title of the plan being summarized.
	tldrViewport        viewport.Model
	lastKey             string    // Last key pressed in editor (for double-key detection).
	lastKeyTime         time.Time // Time of last key press in editor.

	// Dimensions.
	width  int
	height int

	// Styles.
	borderStyle lipgloss.Style

	// Theme
	isDarkModeEnabled bool
}

// NewPlansScreen creates a new plans screen.
func NewPlansScreen(width, height int, isDarkModeEnabled, renderMarkdownByDefault, focus bool, displayMode string) *PlansScreen {
	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	p := PlansScreen{
		list:          components.NewList(nil, panelWidth, contentHeight, focus),
		viewer:        components.NewViewer(panelWidth, contentHeight),
		editor:        components.NewEditor(width-4, contentHeight),
		searchBar:     components.NewSearchBar(panelWidth),
		tagModal:      components.NewTagModal(),
		confirmDialog: components.NewConfirmModal(),
		commentModal:  components.NewCommentModal(),
		tagFilter:     components.NewTagFilter(panelWidth),
		layout:        types.LayoutSplit,
		focus:         types.FocusList,
		width:         width,
		height:        height,
		displayMode:   displayMode,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
		isDarkModeEnabled: isDarkModeEnabled,
	}

	if renderMarkdownByDefault {
		p.viewer.SetRenderMode(components.RenderModeGlamour)
	}

	if displayMode == claudeviewer.DisplayModeTagPlanContent {
		tagsW, _, _ := p.panelWidths()
		p.tagPanel = components.NewTagPanel(tagsW-4, contentHeight-4)
		p.tagPanel.Focus()
		p.focus = types.FocusTagPanel
	}

	return &p
}

// Init initializes the screen.
func (s *PlansScreen) Init() tea.Cmd {
	return nil // Plans loaded via messages.PlansLoadedMsg from App.
}

// Update handles messages.
func (s *PlansScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	// Handle TLDR popup if showing.
	if s.showingTLDR {
		if key, ok := msg.(tea.KeyMsg); ok {
			switch key.String() {
			case "esc", "q":
				s.showingTLDR = false
				return s, nil
			case "g":
				s.tldrViewport.GotoTop()
				return s, nil
			case "G":
				s.tldrViewport.GotoBottom()
				return s, nil
			}
		}
		var cmd tea.Cmd
		s.tldrViewport, cmd = s.tldrViewport.Update(msg)
		return s, cmd
	}

	// When confirm dialog is active, route keys to it before the tag modal.
	if s.confirmDialog.IsActive() {
		if _, ok := msg.(tea.KeyMsg); ok {
			return s, s.confirmDialog.Update(msg)
		}
	}

	// Handle tag modal if showing.
	if s.showingModal {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			cmd := s.tagModal.Update(msg)
			return s, cmd
		case components.SavePlanTagsMsg:
			s.showingModal = false
			return s, func() tea.Msg {
				return messages.SavePlanTagsMsg{
					FileName:   msg.FileName,
					SyncSource: msg.SyncSource,
					Tags:       msg.Tags,
				}
			}
		case components.TagsLoadedMsg:
			s.tagModal.Open(msg.FileName, msg.SyncSource, msg.PlanTags, msg.AllTags)
			return s, nil
		case components.RequestTagDeleteMsg:
			s.confirmDialog.Open(
				fmt.Sprintf("Delete tag %q? This will remove it from all plans.", msg.TagName),
				msg.OnConfirmFunc,
			)
			return s, nil
		}
		return s, nil
	}

	// Handle comment modal if showing.
	if s.showingCommentModal {
		switch msg := msg.(type) {
		case tea.KeyMsg:
			cmd := s.commentModal.Update(msg)
			if !s.commentModal.IsActive() {
				s.showingCommentModal = false
			}
			return s, cmd
		case messages.CommentsLoadedMsg:
			s.commentModal.SetComments(msg.Comments)
			return s, nil
		case messages.AddCommentMsg, messages.DeleteCommentMsg:
			return s, func() tea.Msg { return msg }
		}
		return s, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case messages.PlansLoadedMsg:
		if !msg.IsFiltered {
			s.allPlans = msg.Plans
		}
		s.plans = msg.Plans
		if s.tagPanel != nil {
			s.rebuildTagPanelEntries()
		}
		s.updateListItems()
		s.list.Select(0)
		var cmds []tea.Cmd
		if len(s.plans) > 0 {
			cmds = append(cmds, s.loadPlanDetail(s.plans[0].FileName, s.plans[0].SyncSource))
		}
		if s.tagPanel != nil && !msg.IsFiltered {
			cmds = append(cmds, func() tea.Msg { return messages.LoadAllTagsForPanelMsg{} })
		}
		return s, tea.Batch(cmds...)

	case messages.AllTagsForPanelLoadedMsg:
		s.allTags = msg.Tags
		s.tagPlanCounts = msg.Counts
		s.untaggedCount = msg.UntaggedCount
		s.tagPlanMap = msg.TagPlanMap
		s.allPlans = msg.AllPlans
		s.rebuildTagPanelEntries()
		if s.tagPanel != nil {
			s.applyTagFilter(s.tagPanel.SelectedTag())
		}
		return s, nil

	case components.CreateTagRequestedMsg:
		return s, func() tea.Msg {
			return messages.CreateTagMsg{Name: msg.Name}
		}

	case messages.PlanDetailLoadedMsg:
		s.current = msg.Detail
		viewerWidth := s.getViewerWidth()
		s.viewer.SetContent(content.NewPlanContent(msg.Detail, s.isDarkModeEnabled, s.focus, viewerWidth))
		s.editor.SetContent(msg.Detail.Content)
		return s, nil

	case messages.SaveResultMsg:
		if msg.Error == nil && msg.Result.Success && !msg.Result.HasConflict {
			s.focus = types.FocusContent
			s.editor.Blur()
			// Reload the plan.
			if s.current != nil {
				return s, s.loadPlanDetail(s.current.FileName, s.current.SyncSource)
			}
		}
		return s, nil

	case messages.TLDRGeneratedMsg:
		popupWidth := s.width * 2 / 3
		popupHeight := s.height / 2
		s.tldrTitle = msg.PlanTitle
		s.tldrViewport = viewport.New(popupWidth-4, popupHeight-4)
		rendered := content.RenderMarkdown(msg.Summary, s.isDarkModeEnabled, popupWidth-4)
		s.tldrViewport.SetContent(rendered)
		s.showingTLDR = true
		return s, nil

	case messages.OpenTagModalMsg:
		// Load all tags and current plan tags, then open modal.
		return s, func() tea.Msg {
			return messages.LoadTagsForModalMsg{FileName: msg.FileName, SyncSource: msg.SyncSource}
		}

	case components.TagsLoadedMsg:
		// Open modal with loaded tags.
		s.tagModal.Open(msg.FileName, msg.SyncSource, msg.PlanTags, msg.AllTags)
		s.showingModal = true
		return s, nil

	case messages.SavePlanTagsMsg:
		// Save tags via service layer.
		return s, func() tea.Msg {
			return messages.SetPlanTagsMsg{
				FileName:   msg.FileName,
				SyncSource: msg.SyncSource,
				Tags:       msg.Tags,
			}
		}

	case messages.FilterByTagsMsg:
		// Apply tag filter.
		return s, func() tea.Msg {
			return messages.SearchPlansWithTagsMsg{
				Query:    s.searchQuery,
				Tags:     msg.Tags,
				MatchAll: msg.MatchAll,
			}
		}

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
	case types.FocusEditor:
		cmd = s.editor.Update(msg)
	case types.FocusTagPanel:
		if s.tagPanel != nil {
			cmd = s.tagPanel.Update(msg)
		}
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
	case types.FocusTagPanel:
		return s.handleTagPanelKey(key, msg)
	default:
	}

	return s, nil
}

// handleTagPanelKey handles keys while the tag panel has focus.
func (s *PlansScreen) handleTagPanelKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	if s.tagPanel == nil {
		return s, nil
	}

	// While the creation input is open, delegate all keys to the panel.
	if s.tagPanel.IsCreating() {
		return s, s.tagPanel.Update(msg)
	}

	switch key {
	case "down":
		tag := s.tagPanel.MoveDown()
		return s, s.applyTagFilter(tag)

	case "up":
		tag := s.tagPanel.MoveUp()
		return s, s.applyTagFilter(tag)

	case "tab":
		s.tagPanel.Blur()
		s.focus = types.FocusList
		s.list.Focus()
		return s, nil

	case "/":
		s.tagPanel.Blur()
		s.focus = types.FocusSearch
		return s, s.searchBar.Focus()

	case "n":
		return s, s.tagPanel.StartCreating()
	}

	return s, nil
}

// applyTagFilter filters the plan list by tag. Uses tagPlanMap for instant in-memory
// filtering when available; falls back to async service calls while the map loads.
func (s *PlansScreen) applyTagFilter(tag string) tea.Cmd {
	if tag == "" {
		s.plans = s.allPlans
		s.updateListItems()
		s.list.Select(0)
		if len(s.plans) > 0 {
			return s.loadPlanDetail(s.plans[0].FileName, s.plans[0].SyncSource)
		}
		return nil
	}

	if s.tagPlanMap != nil {
		var mapKey string
		if tag == components.UntaggedSentinel {
			mapKey = "" // untagged plans are stored under "" in the service map
		} else {
			mapKey = tag
		}
		s.plans = s.tagPlanMap[mapKey]
		if s.plans == nil {
			s.plans = []claudeviewer.PlanSummary{}
		}
		s.updateListItems()
		s.list.Select(0)
		if len(s.plans) > 0 {
			return s.loadPlanDetail(s.plans[0].FileName, s.plans[0].SyncSource)
		}
		return nil
	}

	// Map not yet loaded — fall back to async service call.
	if tag == components.UntaggedSentinel {
		return func() tea.Msg { return messages.LoadUntaggedPlansMsg{} }
	}
	return func() tea.Msg {
		return messages.SearchPlansWithTagsMsg{Tags: []string{tag}, MatchAll: false}
	}
}

// handleListKey handles keys in list focus mode.
func (s *PlansScreen) handleListKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "/":
		s.focus = types.FocusSearch
		return s, s.searchBar.Focus()

	case "T":
		s.focus = types.FocusTagFilter
		return s, s.tagFilter.Focus()

	case "m":
		if s.current != nil {
			return s, func() tea.Msg {
				return messages.OpenTagModalMsg{FileName: s.current.FileName, SyncSource: s.current.SyncSource}
			}
		}
		return s, nil

	case "n":
		if s.current != nil {
			s.showingCommentModal = true
			s.commentModal.SetSize(s.width*3/4, s.height*3/4)
			return s, func() tea.Msg {
				return messages.OpenCommentModalMsg{FileName: s.current.FileName, SyncSource: s.current.SyncSource}
			}
		}
		return s, nil

	case "c":
		if s.searchQuery != "" || len(s.tagFilters) > 0 {
			s.searchQuery = ""
			s.tagFilters = nil
			s.searchBar.Reset()
			s.tagFilter.Reset()
			return s, func() tea.Msg {
				return messages.ClearSearchMsg{}
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

	case "d":
		return s, s.dumpPlans()

	case "S":
		return s, func() tea.Msg {
			return messages.OpenSettingsMsg{}
		}

	case "C":
		return s, func() tea.Msg {
			return messages.OpenConnectorsMsg{}
		}

	case "X":
		if s.current != nil {
			if s.showingTLDR {
				return s, func() tea.Msg {
					return messages.RegenerateTLDRMsg{FileName: s.current.FileName, SyncSource: s.current.SyncSource}
				}
			}
			return s, func() tea.Msg {
				return messages.GenerateTLDRMsg{FileName: s.current.FileName, SyncSource: s.current.SyncSource}
			}
		}
		return s, nil

	case "tab":
		s.focus = types.FocusContent
		s.list.Blur()
		return s, nil

	case "shift+tab":
		if s.displayMode == claudeviewer.DisplayModeTagPlanContent && s.tagPanel != nil {
			s.focus = types.FocusTagPanel
			s.tagPanel.Focus()
			s.list.Blur()
		}
		return s, nil

	case "down", "up":
		cmd := s.list.Update(msg)
		if item := s.list.SelectedItem(); item != nil {
			if plan, ok := item.Data().(claudeviewer.PlanSummary); ok {
				return s, tea.Batch(cmd, s.loadPlanDetail(plan.FileName, plan.SyncSource))
			}
		}
		return s, cmd

	case "j", "k":
		now := time.Now()
		if s.lastKey == key && now.Sub(s.lastKeyTime) < 500*time.Millisecond {
			currentItem := s.list.SelectedIndex()
			var nextItemIndex int
			if key == "j" {
				nextItemIndex = min(currentItem+10, s.list.ItemCount()-1)
			} else {
				nextItemIndex = max(0, currentItem-10)
			}
			s.list.Select(nextItemIndex)
			cmd := s.list.Update(msg)
			if item := s.list.SelectedItem(); item != nil {
				if plan, ok := item.Data().(claudeviewer.PlanSummary); ok {
					return s, tea.Batch(cmd, s.loadPlanDetail(plan.FileName, plan.SyncSource))
				}
				return s, cmd
			}
			return s, cmd
		}
		s.lastKey = key
		s.lastKeyTime = now
		return s, nil
	}

	return s, s.list.Update(msg)
}

// handleContentKey handles keys in content view mode.
func (s *PlansScreen) handleContentKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "tab":
		// In three-panel mode, complete the cycle: content → tag panel.
		// In split view, content → list.
		if s.layout == types.LayoutSplit {
			if s.displayMode == claudeviewer.DisplayModeTagPlanContent && s.tagPanel != nil {
				s.tagPanel.Focus()
				s.focus = types.FocusTagPanel
				s.list.Blur()
			} else {
				s.focus = types.FocusList
				s.list.Focus()
			}
			return s, nil
		}
		return s, nil

	case "shift+tab":
		if s.displayMode == claudeviewer.DisplayModeTagPlanContent {
			s.focus = types.FocusList
			s.list.Focus()
			return s, nil
		}
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
				return messages.RequestVersionsScreenMsg{PlanName: s.current.FileName, SyncSource: s.current.SyncSource}
			}
		}
		return s, nil

	case "t":
		// Transmit to connector.
		if s.current != nil {
			return s, func() tea.Msg {
				return messages.SendToConnectorMsg{PlanFileName: s.current.FileName, SyncSource: s.current.SyncSource}
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
		s.lastKey = ""
		return s, nil

	case "ctrl+s":
		// Save.
		if s.current != nil {
			return s, s.savePlan()
		}
		return s, nil

	case "t", "b":
		// Check for double-key press (tt = top, bb = bottom).
		now := time.Now()
		if s.lastKey == key && now.Sub(s.lastKeyTime) < 500*time.Millisecond {
			// Double-key detected, remove the first typed character and trigger navigation.
			s.lastKey = ""
			// Simulate backspace to remove the first character.
			backspaceMsg := tea.KeyMsg{
				Type: tea.KeyBackspace,
			}
			s.editor.Update(backspaceMsg)
			// Now jump to top or bottom.
			if key == "t" {
				s.editor.MoveCursorToFirstRow()
			} else {
				s.editor.MoveCursorToLastRow()
			}
			return s, nil
		}
		// First key press or timeout expired, record and pass to editor.
		s.lastKey = key
		s.lastKeyTime = now
		return s, s.editor.Update(msg)

	case "d":
		// Check for double-key press (dd = delete line).
		now := time.Now()
		if s.lastKey == key && now.Sub(s.lastKeyTime) < 500*time.Millisecond {
			// Double-key detected, remove the first typed character.
			s.lastKey = ""
			// Simulate backspace to remove the first character.
			backspaceMsg := tea.KeyMsg{
				Type: tea.KeyBackspace,
			}
			s.editor.Update(backspaceMsg)

			// Delete the current line using the editor's method.
			s.editor.DeleteCurrentLine()

			return s, nil
		}
		// First key press or timeout expired, record and pass to editor.
		s.lastKey = key
		s.lastKeyTime = now
		return s, s.editor.Update(msg)

	case "o":
		// Check for double-key press (oo = new line below).
		now := time.Now()
		if s.lastKey == key && now.Sub(s.lastKeyTime) < 500*time.Millisecond {
			// Double-key detected, remove the first typed character.
			s.lastKey = ""
			// Simulate backspace to remove the first character.
			backspaceMsg := tea.KeyMsg{
				Type: tea.KeyBackspace,
			}
			s.editor.Update(backspaceMsg)

			// Insert new line below using the editor's method.
			s.editor.InsertNewLineBelow()

			return s, nil
		}
		// First key press or timeout expired, record and pass to editor.
		s.lastKey = key
		s.lastKeyTime = now
		return s, s.editor.Update(msg)

	default:
		// Any other key clears the double-key tracking.
		s.lastKey = ""
		return s, s.editor.Update(msg)
	}
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
			return messages.SearchPlansMsg{Query: query}
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
			return messages.FilterByTagsMsg{
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
		if s.displayMode == claudeviewer.DisplayModeTagPlanContent && s.tagPanel != nil {
			mainContent = s.renderThreePanelView()
		} else {
			mainContent = s.renderSplitView()
		}
	}

	// Overlay TLDR popup if showing.
	if s.showingTLDR {
		popupWidth := s.width * 2 / 3
		popupHeight := s.height / 2

		title := styles.TitleStyle.Render(fmt.Sprintf("TLDR: %s", s.tldrTitle))

		panel := s.borderStyle.
			Width(popupWidth).
			Height(popupHeight - 2).
			Render(s.tldrViewport.View())

		popup := lipgloss.JoinVertical(lipgloss.Left, title, panel)

		overlay := lipgloss.Place(
			s.width,
			s.height,
			lipgloss.Center,
			lipgloss.Center,
			popup,
		)

		return s.overlayContent(mainContent, overlay)
	}

	// Overlay tag modal if showing.
	if s.showingModal {
		modal := s.tagModal.View()

		overlay := lipgloss.Place(
			s.width,
			s.height,
			lipgloss.Center,
			lipgloss.Center,
			modal,
		)

		mainContent = s.overlayContent(mainContent, overlay)

		// Overlay confirm dialog on top of the tag modal when active.
		if s.confirmDialog.IsActive() {
			dialogOverlay := lipgloss.Place(
				s.width,
				s.height,
				lipgloss.Center,
				lipgloss.Center,
				s.confirmDialog.View(),
			)
			return s.overlayContent(mainContent, dialogOverlay)
		}

		return mainContent
	}

	// Overlay comment modal if showing.
	if s.showingCommentModal {
		overlay := lipgloss.Place(
			s.width,
			s.height,
			lipgloss.Center,
			lipgloss.Center,
			s.commentModal.View(),
		)
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
	plansListContent := s.list.View()
	if s.searchBar.IsActive() {
		plansListContent = lipgloss.JoinVertical(lipgloss.Left, plansListContent, s.searchBar.View())
	}
	if s.tagFilter.IsActive() {
		plansListContent = lipgloss.JoinVertical(lipgloss.Left, plansListContent, s.tagFilter.View())
	}

	activeBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.AccentColor)

	plansListContentBorder := s.borderStyle
	plansContentBorder := s.borderStyle

	switch s.focus {
	case types.FocusList:
		plansListContentBorder = activeBorder
	case types.FocusContent:
		plansContentBorder = activeBorder
	default:
	}

	plansListView := plansListContentBorder.
		Width(panelWidth).
		Height(contentHeight).
		Render(plansListContent)

	planContentView := plansContentBorder.
		Width(panelWidth).
		Height(contentHeight).
		Render(s.viewer.View())

	divider := s.renderDivider(contentHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, plansListView, divider, planContentView)
}

// renderThreePanelView renders the three-panel layout: tags | list | content.
func (s *PlansScreen) renderThreePanelView() string {
	tagsW, plansW, viewW := s.panelWidths()
	contentHeight := s.height - 4
	innerH := contentHeight - 4

	listInnerH := innerH
	if s.searchBar.IsActive() {
		listInnerH -= 3
	}
	s.tagPanel.SetSize(tagsW-4, innerH)
	s.list.SetSize(plansW-4, listInnerH)
	s.viewer.SetSize(viewW-4, innerH)
	s.searchBar.SetWidth(plansW - 4)

	activeBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.AccentColor)

	tagBorder := s.borderStyle
	listBorder := s.borderStyle
	contentBorder := s.borderStyle
	switch s.focus {
	case types.FocusTagPanel:
		tagBorder = activeBorder
	case types.FocusList:
		listBorder = activeBorder
	case types.FocusContent:
		contentBorder = activeBorder
	default:
	}

	tagPanelView := tagBorder.
		Width(tagsW).
		Height(contentHeight).
		Render(s.tagPanel.View())

	var listContent string
	if s.list.ItemCount() == 0 {
		listContent = styles.InactiveStyle.Render("No items.")
	} else {
		listContent = s.list.View()
	}
	if s.searchBar.IsActive() {
		listContent = lipgloss.JoinVertical(lipgloss.Left, listContent, s.searchBar.View())
	}
	listView := listBorder.
		Width(plansW).
		Height(contentHeight).
		Render(listContent)

	rightView := contentBorder.
		Width(viewW).
		Height(contentHeight).
		Render(s.viewer.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, tagPanelView, listView, rightView)
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
	var sb strings.Builder
	for range height {
		sb.WriteString("│\n")
	}
	return strings.TrimSuffix(sb.String(), "\n")
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

// RenderedMarkdownByDefault upddates the renderned markdown by default mesasage.
func (s *PlansScreen) RenderedMarkdownByDefault(enabled bool) {
	renderMode := components.RenderModeRaw
	if enabled {
		renderMode = components.RenderModeGlamour
	}

	if s.viewer.RenderMode() != renderMode {
		viewerWidth := s.getViewerWidth()
		s.viewer.SetRenderMode(renderMode)
		s.viewer.SetContent(content.NewPlanContent(s.current, s.isDarkModeEnabled, s.focus, viewerWidth))
	}
}

// getViewerWidth calculates the current viewer width based on layout.
func (s *PlansScreen) getViewerWidth() int {
	if s.layout == types.LayoutFullscreen {
		return s.width - 4
	}
	if s.displayMode == claudeviewer.DisplayModeTagPlanContent && s.tagPanel != nil {
		_, _, viewW := s.panelWidths()
		return viewW - 4
	}
	panelWidth := (s.width - 3) / 2
	return panelWidth - 4
}

// panelWidths returns the widths for the three-panel layout.
func (s *PlansScreen) panelWidths() (int, int, int) {
	return s.width / 5, (s.width * 3) / 10, s.width - s.width/5 - (s.width*3)/10 - 4
}

// rebuildTagPanelEntries updates the tag panel using authoritative DB counts.
// tagPlanCounts (from GetTagPlanCounts) is the source of truth; allTags provides
// the full tag list so unassigned tags still appear.
func (s *PlansScreen) rebuildTagPanelEntries() {
	if s.tagPanel == nil {
		return
	}
	counts := s.tagPlanCounts
	if counts == nil {
		// Fall back to counting from plan summaries until DB counts arrive.
		counts = make(map[string]int)
		for _, plan := range s.allPlans {
			for _, tag := range plan.Tags {
				if tag.Name != "" {
					counts[tag.Name]++
				}
			}
		}
	}
	seen := make(map[string]bool)
	var entries []components.TagPanelEntry
	for _, tag := range s.allTags {
		if tag.Name == "" {
			continue
		}
		seen[tag.Name] = true
		entries = append(entries, components.TagPanelEntry{Name: tag.Name, Count: counts[tag.Name]})
	}
	// Include any tags found only in plan summaries (safety net).
	for name := range counts {
		if name != "" && !seen[name] {
			entries = append(entries, components.TagPanelEntry{Name: name, Count: counts[name]})
		}
	}
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	s.tagPanel.SetEntries(entries, len(s.allPlans), s.untaggedCount)
}

// SetDisplayMode switches the screen between display modes.
func (s *PlansScreen) SetDisplayMode(mode string) {
	s.displayMode = mode
	if mode == claudeviewer.DisplayModeTagPlanContent {
		if s.tagPanel == nil {
			tagsW, _, _ := s.panelWidths()
			contentHeight := s.height - 4
			s.tagPanel = components.NewTagPanel(tagsW-4, contentHeight-4)
		}
		s.rebuildTagPanelEntries()
		s.tagPanel.Focus()
		s.focus = types.FocusTagPanel
	} else {
		s.tagPanel = nil
		if s.focus == types.FocusTagPanel {
			s.focus = types.FocusList
		}
		// Restore full unfiltered list.
		if len(s.allPlans) > 0 {
			s.plans = s.allPlans
			s.updateListItems()
		}
	}
}

// ShortHelp returns key binding help.
func (s *PlansScreen) ShortHelp() string {
	if s.confirmDialog.IsActive() {
		return "←/→: select  y: yes  n/esc: cancel  enter: confirm"
	}
	switch s.focus {
	case types.FocusTagPanel:
		if s.tagPanel != nil && s.tagPanel.IsCreating() {
			return "enter: create tag | esc: cancel"
		}
		return fmt.Sprintf("down/up: navigate tags | tab: plans | /: search | n: new tag | q: quit | Plans: %d", len(s.plans))
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
		tagNav := ""
		if s.displayMode == claudeviewer.DisplayModeTagPlanContent {
			tagNav = "shift+tab: tags | "
		}
		return fmt.Sprintf("down/up: navigate | m: manage tags | tab: content | %sv: fullscreen | e: edit | s: sync | S: settings | r: rsync | d: dump | C: connectors | X: summarize | %s | Plans: %d", tagNav, searchHelp, len(s.plans))
	case types.FocusContent:
		mode := "RAW"
		if s.viewer.RenderMode() == components.RenderModeGlamour {
			mode = "RENDERED"
		}
		if s.layout == types.LayoutSplit {
			return fmt.Sprintf("down/up: scroll | g/G: top/bottom | r: render (%s) | tab: list | esc: back", mode)
		}
		return fmt.Sprintf("down/up: scroll | g/G: top/bottom | r: render (%s) | e: edit | v: versions | t: transmit | esc: back", mode)
	case types.FocusEditor:
		modified := ""
		if s.editor.IsModified() {
			modified = " [MODIFIED]"
		}
		return fmt.Sprintf("ctrl+s: save | tt/bb: top/bottom | dd: delete line | oo: new line | esc: cancel%s", modified)
	case types.FocusSearch:
		return "enter: search | esc: cancel"
	case types.FocusTagFilter:
		mode := "OR"
		if s.tagFilter.MatchAll() {
			mode = "AND"
		}
		return fmt.Sprintf("enter: filter | ctrl+t: toggle mode (%s) | esc: cancel", mode)
	default:
		return ""
	}
}

// IsInputMode returns true when capturing text input.
func (s *PlansScreen) IsInputMode() bool {
	if s.focus == types.FocusTagPanel && s.tagPanel != nil && s.tagPanel.IsCreating() {
		return true
	}
	if s.confirmDialog.IsActive() {
		return true
	}
	return s.focus == types.FocusEditor || s.focus == types.FocusSearch || s.focus == types.FocusTagFilter || s.showingModal || s.showingCommentModal
}

// EditorMode returns true when the screen is in editor focus.
func (s *PlansScreen) EditorMode() bool {
	return s.focus == types.FocusEditor
}

// overlayContent overlays the modal on top of the main content.
func (s *PlansScreen) overlayContent(base, overlay string) string {
	baseLines := strings.Split(base, "\n")
	overlayLines := strings.Split(overlay, "\n")

	// Ensure both have the same number of lines
	maxLines := max(len(baseLines), len(overlayLines))

	result := make([]string, maxLines)
	for i := range maxLines {
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
		if plan.SyncLabel != "" {
			desc += fmt.Sprintf(" | %s", plan.SyncLabel)
		}
		if len(plan.Tags) > 0 {
			tagNames := make([]string, len(plan.Tags))
			for j, tag := range plan.Tags {
				tagNames[j] = tag.Name
			}
			desc += fmt.Sprintf(" | [%s]", strings.Join(tagNames, ", "))
		}
		if plan.CommentCount > 0 {
			desc += fmt.Sprintf(" | 💬%d", plan.CommentCount)
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

func (s *PlansScreen) loadPlanDetail(fileName, syncSource string) tea.Cmd {
	return func() tea.Msg {
		return messages.LoadPlanDetailMsg{FileName: fileName, SyncSource: syncSource}
	}
}

func (s *PlansScreen) savePlan() tea.Cmd {
	return func() tea.Msg {
		return messages.SavePlanMsg{
			FileName:   s.current.FileName,
			SyncSource: s.current.SyncSource,
			Content:    s.editor.Content(),
			Modified:   s.current.ModifiedAt,
		}
	}
}

func (s *PlansScreen) syncPlans() tea.Cmd {
	return func() tea.Msg {
		return messages.SyncPlansMsg{}
	}
}

func (s *PlansScreen) rsyncPlans() tea.Cmd {
	return func() tea.Msg {
		return messages.RSyncPlansMsg{}
	}
}

func (s *PlansScreen) dumpPlans() tea.Cmd {
	return func() tea.Msg {
		return messages.DumpPlansMsg{}
	}
}
