package screens

import (
	"fmt"
	"sort"
	"strconv"
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
	labelPanel    *components.LabelPanel
	commentModal  *components.CommentModal
	renameModal   *components.RenameModal
	inputModal    *components.InputModal[int]

	// State.
	layout types.Layout
	// baseLayout is the layout to restore when leaving fullscreen: Split in
	// plan_content, ThreePanel whenever a side panel is mounted.
	baseLayout    types.Layout
	focus         types.Focus
	plans         []claudeviewer.PlanSummary            // filtered view shown in list
	allPlans      []claudeviewer.PlanSummary            // full unfiltered source of truth
	allTags       []claudeviewer.Tag                    // all tags in the system (including unassigned)
	tagPlanCounts map[string]int                        // authoritative plan count per tag from DB
	tagPlanMap    map[string][]claudeviewer.PlanSummary // map tags to a planSummary
	untaggedCount int                                   // number of plans with no tags assigned
	current       *claudeviewer.PlanDetail
	searchQuery   string   // Current active search query (empty = show all).
	tagFilters    []string // Active tag filters.
	activeModal   types.ModalState
	tldrTitle     string // Title of the plan being summarized.
	tldrSummary   string // Raw summary text, used when saving as a comment.
	tldrViewport  viewport.Model
	lastKey       string    // Last key pressed in editor (for double-key detection).
	lastKeyTime   time.Time // Time of last key press in editor.

	// Dimensions.
	width  int
	height int

	// Styles.
	borderStyle lipgloss.Style

	// Theme
	isDarkModeEnabled bool
	markdownTheme     string
}

// NewPlansScreen creates a new plans screen.
func NewPlansScreen(width, height int, isDarkModeEnabled, renderMarkdownByDefault, focus bool, displayMode, markdownRenderedTheme string) *PlansScreen {
	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	p := PlansScreen{
		list:          components.NewList(nil, panelWidth, contentHeight, focus),
		viewer:        components.NewViewer(panelWidth, contentHeight, markdownRenderedTheme),
		editor:        components.NewEditor(width-4, contentHeight),
		searchBar:     components.NewSearchBar(panelWidth),
		tagModal:      components.NewTagModal(),
		confirmDialog: components.NewConfirmModal(),
		commentModal:  components.NewCommentModal(),
		renameModal:   components.NewRenameModal(),
		inputModal:    components.NewInputModal[int](),
		tagFilter:     components.NewTagFilter(panelWidth),
		layout:        types.LayoutSplit,
		baseLayout:    types.LayoutSplit,
		focus:         types.FocusList,
		width:         width,
		height:        height,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
		isDarkModeEnabled: isDarkModeEnabled,
		markdownTheme:     markdownRenderedTheme,
	}

	if renderMarkdownByDefault {
		p.viewer.SetRenderMode(components.RenderModeGlamour)
	}

	switch displayMode {
	case claudeviewer.DisplayModeTagPlanContent:
		p.tagPanel = components.NewTagPanel(0, contentHeight-4)
		p.tagPanel.Focus()
		p.focus = types.FocusTagPanel
		p.layout = types.LayoutThreePanel
		p.baseLayout = types.LayoutThreePanel
	case claudeviewer.DisplayModeLabelPlanContent:
		p.labelPanel = components.NewLabelPanel(0, contentHeight-4)
		p.labelPanel.Focus()
		p.focus = types.FocusLabelPanel
		p.layout = types.LayoutThreePanel
		p.baseLayout = types.LayoutThreePanel
	}

	// panelWidths depends on which panel is mounted, so size after the switch.
	p.sizeSidePanel()

	return &p
}

// Init initializes the screen.
func (s *PlansScreen) Init() tea.Cmd {
	return nil // Plans loaded via messages.PlansLoadedMsg from App.
}

// Update handles messages.
func (s *PlansScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	// Confirm dialog intercepts keys in all states — it can layer on top of any modal.
	if s.confirmDialog.IsActive() {
		if _, ok := msg.(tea.KeyMsg); ok {
			return s, s.confirmDialog.Update(msg)
		}
	}

	switch s.activeModal {
	case types.ModalNone:
		// no modal active, handle normally below
	case types.ModalTLDR:
		return s.handleTLDRUpdate(msg)
	case types.ModalTagManager:
		return s.handleTagModalUpdate(msg)
	case types.ModalComment:
		return s.handleCommentModalUpdate(msg)
	case types.ModalRenameFile:
		return s.handleRenameModalUpdate(msg)
	case types.ModalTextInput:
		return s.handleInputModalUpdate(msg)
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case messages.PlansLoadedMsg:
		if !msg.IsFiltered {
			s.allPlans = msg.Plans
		}
		s.plans = msg.Plans
		s.rebuildTagPanelEntries()
		s.rebuildLabelPanelEntries()
		s.updateListItems()
		s.list.Select(0)
		var cmds []tea.Cmd
		if len(s.plans) > 0 {
			cmds = append(cmds, s.loadPlanDetail(s.plans[0].FileName, s.plans[0].SyncSource))
		}

		if !msg.IsFiltered {
			switch {
			case s.tagPanel != nil:
				// The selected tag filter is reapplied once the tags finish loading
				// (see AllTagsForPanelLoadedMsg) — it needs the tag/plan map.
				cmds = append(cmds, func() tea.Msg { return messages.LoadAllTagsForPanelMsg{} })
			case s.labelPanel != nil:
				// Label filtering is purely in-memory, so reapply the selection right
				// away — otherwise an unfiltered reload (e.g. after a mutating command)
				// would silently drop the active filter.
				cmds = append(cmds, s.applyLabelFilter(s.labelPanel.SelectedLabel()))
			}
		}

		return s, tea.Batch(cmds...)

	case messages.AllTagsForPanelLoadedMsg:
		// Tag-panel data only. Other display modes never request it and must not
		// be mutated by it.
		if s.tagPanel == nil {
			return s, nil
		}
		s.allTags = msg.Tags
		s.tagPlanCounts = msg.Counts
		s.untaggedCount = msg.UntaggedCount
		s.tagPlanMap = msg.TagPlanMap
		s.allPlans = msg.AllPlans
		s.rebuildTagPanelEntries()
		return s, s.applyTagFilter(s.tagPanel.SelectedTag())

	case components.CreateTagRequestedMsg:
		return s, func() tea.Msg {
			return messages.CreateTagMsg{Name: msg.Name}
		}

	case messages.PlanDetailLoadedMsg:
		s.current = msg.Detail
		viewerWidth := s.getViewerWidth()
		s.viewer.SetContent(content.NewPlanContent(msg.Detail, viewerWidth))
		s.editor.SetContent(msg.Detail.Content)
		return s, nil

	case messages.SaveResultMsg:
		if msg.Error == nil && msg.Result.Success && !msg.Result.HasConflict && msg.Plan != nil {
			s.current = msg.Plan
			viewerWidth := s.getViewerWidth()
			s.viewer.SetContent(content.NewPlanContent(msg.Plan, viewerWidth))
			s.editor.MarkSaved(msg.Plan.Content)
		}
		return s, nil

	case messages.TLDRGeneratedMsg:
		popupWidth := s.width * 2 / 3
		popupHeight := s.height / 2
		s.tldrTitle = msg.PlanTitle
		s.tldrSummary = msg.Summary
		s.tldrViewport = viewport.New(popupWidth-4, popupHeight-4)
		rendered := content.RenderMarkdown(msg.Summary, s.markdownTheme, popupWidth-4)
		s.tldrViewport.SetContent(rendered)
		s.activeModal = types.ModalTLDR
		return s, nil

	case messages.OpenTagModalMsg:
		// Load all tags and current plan tags, then open modal.
		return s, func() tea.Msg {
			return messages.LoadTagsForModalMsg{FileName: msg.FileName, SyncSource: msg.SyncSource}
		}

	case components.TagsLoadedMsg:
		// Open modal with loaded tags.
		s.tagModal.Open(msg.FileName, msg.SyncSource, msg.PlanTags, msg.AllTags)
		s.activeModal = types.ModalTagManager
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
	case types.FocusEditor:
		cmd = s.editor.Update(msg)
	case types.FocusTagPanel:
		if s.tagPanel != nil {
			cmd = s.tagPanel.Update(msg)
		}
	default:
		// FocusLabelPanel included: LabelPanel takes no input.
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
	case types.FocusLabelPanel:
		return s.handleLabelPanelKey(key)
	default:
	}

	return s, nil
}

// sizeSidePanel resizes whichever side panel is mounted to the current layout.
func (s *PlansScreen) sizeSidePanel() {
	sideW, _, _ := s.panelWidths()
	innerH := s.height - 8

	switch {
	case s.tagPanel != nil:
		s.tagPanel.SetSize(sideW-4, innerH)
	case s.labelPanel != nil:
		s.labelPanel.SetSize(sideW-4, innerH)
	}
}

// focusSidePanel moves focus to whichever side panel is mounted, reporting
// whether one was. Callers are responsible for blurring the panel they leave.
func (s *PlansScreen) focusSidePanel() bool {
	switch {
	case s.tagPanel != nil:
		s.tagPanel.Focus()
		s.focus = types.FocusTagPanel
		return true
	case s.labelPanel != nil:
		s.labelPanel.Focus()
		s.focus = types.FocusLabelPanel
		return true
	default:
		return false
	}
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
		return s, s.applyTagFilter(s.tagPanel.MoveDown())

	case "up":
		return s, s.applyTagFilter(s.tagPanel.MoveUp())

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

// handleLabelPanelKey handles keys while the label panel has focus. Labels are
// read-only, so there is no creation path here.
func (s *PlansScreen) handleLabelPanelKey(key string) (Screen, tea.Cmd) {
	if s.labelPanel == nil {
		return s, nil
	}

	switch key {
	case "down":
		return s, s.applyLabelFilter(s.labelPanel.MoveDown())

	case "up":
		return s, s.applyLabelFilter(s.labelPanel.MoveUp())

	case "tab":
		s.labelPanel.Blur()
		s.focus = types.FocusList
		s.list.Focus()
		return s, nil

	case "/":
		s.labelPanel.Blur()
		s.focus = types.FocusSearch
		return s, s.searchBar.Focus()
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

// applyLabelFilter filters the plan list by sync label, entirely in-memory —
// unlike tags, every plan's SyncLabel is already present on s.allPlans, so
// there's no DB round-trip or "map not yet loaded" fallback to handle.
func (s *PlansScreen) applyLabelFilter(label string) tea.Cmd {
	if label == "" {
		s.plans = s.allPlans
	} else {
		filtered := make([]claudeviewer.PlanSummary, 0, len(s.allPlans))
		for _, p := range s.allPlans {
			if p.SyncLabel == label {
				filtered = append(filtered, p)
			}
		}
		s.plans = filtered
	}
	s.updateListItems()
	s.list.Select(0)
	if len(s.plans) > 0 {
		return s.loadPlanDetail(s.plans[0].FileName, s.plans[0].SyncSource)
	}
	return nil
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
			s.activeModal = types.ModalComment
			s.commentModal.SetSize(s.width*3/4, s.height*3/4)
			s.commentModal.SetDarkMode(s.isDarkModeEnabled)
			s.commentModal.SetRenderMarkdown(s.viewer.RenderMode() == components.RenderModeGlamour)
			return s, func() tea.Msg {
				return messages.OpenCommentModalMsg{FileName: s.current.FileName, SyncSource: s.current.SyncSource}
			}
		}
		return s, nil

	case "ctrl+l":
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

	case "c":
		if s.current != nil {
			return s, func() tea.Msg {
				return messages.CopyToClipboardMsg{Text: s.current.Content, Label: s.current.FileName}
			}
		}
		return s, nil

	case "v":
		if s.current != nil {
			s.layout = types.LayoutFullscreen
			s.focus = types.FocusContent
			// Regenerate content with fullscreen width
			viewerWidth := s.getViewerWidth()
			s.viewer.SetContent(content.NewPlanContent(s.current, viewerWidth))
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

	case "D":
		// Guard: s.current is the loaded *PlanDetail and is nil when the list is empty
		// or before the first PlanDetailLoadedMsg. Without it, D would nil-panic.
		if s.current != nil {
			fileName, syncSource := s.current.FileName, s.current.SyncSource
			title := s.current.Title
			s.confirmDialog.Open(
				fmt.Sprintf("Are you sure you want to delete the plan %q? This cannot be undone.", title),
				func() tea.Msg {
					return messages.DeletePlanMsg{FileName: fileName, SyncSource: syncSource}
				},
			)
		}
		return s, nil

	case "R":
		if s.current != nil {
			s.activeModal = types.ModalRenameFile
			s.renameModal.SetSize(min(60, s.width-4), min(12, s.height-2))
			s.renameModal.Open(s.current.FileName, s.current.SyncSource)
		}
		return s, nil

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
			if s.activeModal == types.ModalTLDR {
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
		if s.focusSidePanel() {
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
		// In three-panel mode, complete the cycle: content → side panel.
		// In split view, content → list.
		if s.layout != types.LayoutFullscreen {
			if s.focusSidePanel() {
				s.list.Blur()
			} else {
				s.focus = types.FocusList
				s.list.Focus()
			}
		}
		return s, nil

	case "shift+tab":
		if s.layout == types.LayoutThreePanel {
			s.focus = types.FocusList
			s.list.Focus()
			return s, nil
		}
	case "esc":
		// Back to list (split/three-panel view) or back out of fullscreen.
		if s.layout == types.LayoutFullscreen {
			s.layout = s.baseLayout
			s.focus = types.FocusList
			s.viewer.GotoTop()
			// Regenerate content with split view width
			if s.current != nil {
				viewerWidth := s.getViewerWidth()
				s.viewer.SetContent(content.NewPlanContent(s.current, viewerWidth))
			}
		} else {
			s.focus = types.FocusList
		}
		return s, nil

	case "e":
		// Enter edit mode, keeping the cursor near wherever the viewer was scrolled to.
		if s.current != nil {
			s.focus = types.FocusEditor
			return s, s.editor.FocusAtLine(s.viewer.CurrentContentLine())
		}
		return s, nil

	case "r":
		// Toggle render mode.
		s.viewer.ToggleRenderMode()
		return s, nil

	case "l":
		// Toggle line numbers.
		s.viewer.ToggleLineNumbers()
		return s, nil

	case "c":
		// Copy the plan's raw markdown to the clipboard.
		if s.current != nil {
			return s, func() tea.Msg {
				return messages.CopyToClipboardMsg{Text: s.current.Content, Label: s.current.FileName}
			}
		}
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

	case "n":
		if s.current != nil {
			s.activeModal = types.ModalComment
			s.commentModal.SetSize(s.width*3/4, s.height*3/4)
			s.commentModal.SetDarkMode(s.isDarkModeEnabled)
			s.commentModal.SetRenderMarkdown(s.viewer.RenderMode() == components.RenderModeGlamour)
			return s, func() tea.Msg {
				return messages.OpenCommentModalMsg{FileName: s.current.FileName, SyncSource: s.current.SyncSource}
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
		// Cancel editing, back to content view. Capture the cursor's line
		// before Reset() rewinds it to the top, so the viewer can scroll to
		// keep showing the same spot.
		line := s.editor.CurrentLine()
		s.focus = types.FocusContent
		s.editor.Blur()
		s.editor.Reset()
		s.viewer.ScrollToContentLine(line)
		s.lastKey = ""
		return s, nil

	case "ctrl+s":
		// Save.
		if s.current != nil {
			return s, s.savePlan()
		}
		return s, nil

	case "ctrl+l":
		// Jump to a specific line.
		maxLine := s.editor.LineCount()
		s.activeModal = types.ModalTextInput
		s.inputModal.SetSize(min(40, s.width-4), min(8, s.height-2))
		s.inputModal.Open(
			fmt.Sprintf("Go to line (1-%d)", maxLine),
			"line number",
			func(r rune) bool { return r >= '0' && r <= '9' },
			strconv.Atoi,
			func(n int) {
				if n > 0 {
					s.editor.JumpToLine(n)
				}
			},
		)
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

// handleTLDRUpdate routes messages while the TLDR popup is active.
func (s *PlansScreen) handleTLDRUpdate(msg tea.Msg) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "esc", "q":
			s.activeModal = types.ModalNone
			return s, nil
		case "s":
			if s.current != nil {
				fileName, syncSource, summary := s.current.FileName, s.current.SyncSource, s.tldrSummary
				return s, func() tea.Msg {
					return messages.AddCommentMsg{
						FileName:   fileName,
						SyncSource: syncSource,
						Content:    summary,
					}
				}
			}
			return s, nil
		}
	}
	var cmd tea.Cmd
	s.tldrViewport, cmd = s.tldrViewport.Update(msg)
	return s, cmd
}

// handleTagModalUpdate routes messages while the tag modal is active.
func (s *PlansScreen) handleTagModalUpdate(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s, s.tagModal.Update(msg)
	case components.SavePlanTagsMsg:
		s.activeModal = types.ModalNone
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

// handleCommentModalUpdate routes messages while the comment modal is active.
func (s *PlansScreen) handleCommentModalUpdate(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		cmd := s.commentModal.Update(msg)
		if !s.commentModal.IsActive() {
			s.activeModal = types.ModalNone
		}
		return s, cmd
	case messages.CommentsLoadedMsg:
		if s.commentModal.IsActive() {
			s.commentModal.SetComments(msg.Comments)
		} else {
			s.commentModal.Open(msg.FileName, msg.SyncSource, s.markdownTheme, msg.Comments)
		}
		return s, nil
	case messages.AddCommentMsg, messages.DeleteCommentMsg:
		return s, func() tea.Msg { return msg }
	}
	return s, nil
}

// handleRenameModalUpdate routes messages while the rename modal is active.
func (s *PlansScreen) handleRenameModalUpdate(msg tea.Msg) (Screen, tea.Cmd) {
	cmd := s.renameModal.Update(msg)
	if !s.renameModal.IsActive() {
		s.activeModal = types.ModalNone
	}
	return s, cmd
}

// handleInputModalUpdate routes messages while the generic input modal is active.
func (s *PlansScreen) handleInputModalUpdate(msg tea.Msg) (Screen, tea.Cmd) {
	cmd := s.inputModal.Update(msg)
	if !s.inputModal.IsActive() {
		s.activeModal = types.ModalNone
	}
	return s, cmd
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
	case types.LayoutThreePanel:
		mainContent = s.renderThreePanelView()
	default:
		mainContent = s.renderSplitView()
	}

	switch s.activeModal {
	case types.ModalNone:
		// no overlay, fall through to confirmDialog check below
	case types.ModalTLDR:
		popupWidth := s.width * 2 / 3
		popupHeight := s.height / 2

		title := styles.TitleStyle.Render(fmt.Sprintf("TLDR: %s", s.tldrTitle))

		panel := s.borderStyle.
			Width(popupWidth).
			Height(popupHeight - 2).
			Render(s.tldrViewport.View())

		help := styles.MutedStyle.Render("s: save as comment  q/esc: close")
		popup := lipgloss.JoinVertical(lipgloss.Left, title, panel, help)

		overlay := lipgloss.Place(
			s.width,
			s.height,
			lipgloss.Center,
			lipgloss.Center,
			popup,
		)

		return s.overlayContent(mainContent, overlay)

	case types.ModalTagManager:
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

	case types.ModalComment:
		overlay := lipgloss.Place(
			s.width,
			s.height,
			lipgloss.Center,
			lipgloss.Center,
			s.commentModal.View(),
		)
		return s.overlayContent(mainContent, overlay)

	case types.ModalRenameFile:
		overlay := lipgloss.Place(
			s.width,
			s.height,
			lipgloss.Center,
			lipgloss.Center,
			s.renameModal.View(),
		)
		return s.overlayContent(mainContent, overlay)

	case types.ModalTextInput:
		overlay := lipgloss.Place(
			s.width,
			s.height,
			lipgloss.Center,
			lipgloss.Center,
			s.inputModal.View(),
		)
		return s.overlayContent(mainContent, overlay)
	}

	if s.confirmDialog.IsActive() {
		overlay := lipgloss.Place(
			s.width,
			s.height,
			lipgloss.Center,
			lipgloss.Center,
			s.confirmDialog.View(),
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

// renderThreePanelView renders the three-panel layout: tags or labels | list | content.
func (s *PlansScreen) renderThreePanelView() string {
	sideW, plansW, viewW := s.panelWidths()
	contentHeight := s.height - 4
	innerH := contentHeight - 4

	listInnerH := innerH
	if s.searchBar.IsActive() {
		listInnerH -= 3
	}
	if s.tagFilter.IsActive() {
		listInnerH -= 3
	}

	s.sizeSidePanel()

	var sidePanelContent string
	sideFocused := false
	switch {
	case s.tagPanel != nil:
		sidePanelContent = s.tagPanel.View()
		sideFocused = s.focus == types.FocusTagPanel
	case s.labelPanel != nil:
		sidePanelContent = s.labelPanel.View()
		sideFocused = s.focus == types.FocusLabelPanel
	}

	s.list.SetSize(plansW-4, listInnerH)
	s.viewer.SetSize(viewW-4, innerH)
	s.searchBar.SetWidth(plansW - 4)
	s.tagFilter.SetWidth(plansW - 4)

	activeBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.AccentColor)

	sideBorder := s.borderStyle
	listBorder := s.borderStyle
	contentBorder := s.borderStyle
	switch {
	case sideFocused:
		sideBorder = activeBorder
	case s.focus == types.FocusList:
		listBorder = activeBorder
	case s.focus == types.FocusContent:
		contentBorder = activeBorder
	}

	sidePanelView := sideBorder.
		Width(sideW).
		Height(contentHeight).
		Render(sidePanelContent)

	var listContent string
	if s.list.ItemCount() == 0 {
		listContent = styles.InactiveStyle.Render("No items.")
	} else {
		listContent = s.list.View()
	}
	if s.searchBar.IsActive() {
		listContent = lipgloss.JoinVertical(lipgloss.Left, listContent, s.searchBar.View())
	}
	if s.tagFilter.IsActive() {
		listContent = lipgloss.JoinVertical(lipgloss.Left, listContent, s.tagFilter.View())
	}
	listView := listBorder.
		Width(plansW).
		Height(contentHeight).
		Render(listContent)

	rightView := contentBorder.
		Width(viewW).
		Height(contentHeight).
		Render(s.viewer.View())

	return lipgloss.JoinHorizontal(lipgloss.Top, sidePanelView, listView, rightView)
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
	s.commentModal.SetDarkMode(enabled)
	if s.current != nil {
		viewerWidth := s.getViewerWidth()
		s.viewer.SetContent(content.NewPlanContent(s.current, viewerWidth))
	}
}

// RenderedMarkdownByDefault updates the renderned markdown by default mesasage.
func (s *PlansScreen) RenderedMarkdownByDefault(enabled bool) {
	renderMode := components.RenderModeRaw
	if enabled {
		renderMode = components.RenderModeGlamour
	}

	s.commentModal.SetRenderMarkdown(enabled)

	if s.viewer.RenderMode() != renderMode {
		s.viewer.SetRenderMode(renderMode)
		if s.current != nil {
			s.viewer.SetContent(content.NewPlanContent(s.current, s.getViewerWidth()))
		}
	}
}

// RenderedMarkdownTheme updates the markdown theme.
func (s *PlansScreen) RenderedMarkdownTheme(theme string) {
	s.markdownTheme = theme
	s.viewer.SetMarkdownTheme(theme)
	s.commentModal.SetMarkdownTheme(theme)
}

// getViewerWidth calculates the current viewer width based on layout.
func (s *PlansScreen) getViewerWidth() int {
	switch s.layout {
	case types.LayoutFullscreen:
		return s.width - 4
	case types.LayoutThreePanel:
		_, _, viewW := s.panelWidths()
		return viewW - 4
	default:
		panelWidth := (s.width - 3) / 2
		return panelWidth - 4
	}
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

// rebuildLabelPanelEntries updates the label panel with sync-label counts computed
// from s.allPlans. Unlike tags, every plan has exactly one SyncLabel (never
// empty — see labelForSource), so counting from allPlans is authoritative;
// there's no DB-backed "universe of labels" to reconcile against.
func (s *PlansScreen) rebuildLabelPanelEntries() {
	if s.labelPanel == nil {
		return
	}
	counts := make(map[string]int)
	paths := make(map[string]string)
	for _, plan := range s.allPlans {
		counts[plan.SyncLabel]++
		// A label maps to exactly one source directory, so the first plan
		// carrying it settles the path.
		if _, seen := paths[plan.SyncLabel]; !seen {
			paths[plan.SyncLabel] = plan.SyncSource
		}
	}
	labels := make([]string, 0, len(counts))
	for label := range counts {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	entries := make([]components.LabelPanelEntry, len(labels))
	for i, label := range labels {
		entries[i] = components.LabelPanelEntry{
			Name:      label,
			Path:      paths[label],
			PlanCount: counts[label],
		}
	}
	s.labelPanel.SetEntries(entries, len(s.allPlans))
}

// SetDisplayMode switches the screen between display modes, mounting exactly one
// side panel (or neither) and setting the layout it implies.
func (s *PlansScreen) SetDisplayMode(mode string) {
	switch mode {
	case claudeviewer.DisplayModeTagPlanContent:
		s.labelPanel = nil
		if s.tagPanel == nil {
			s.tagPanel = components.NewTagPanel(0, 0)
		}
		s.rebuildTagPanelEntries()
		s.tagPanel.Focus()
		s.focus = types.FocusTagPanel
		s.setBaseLayout(types.LayoutThreePanel)

	case claudeviewer.DisplayModeLabelPlanContent:
		s.tagPanel = nil
		if s.labelPanel == nil {
			s.labelPanel = components.NewLabelPanel(0, 0)
		}
		s.rebuildLabelPanelEntries()
		s.labelPanel.Focus()
		s.focus = types.FocusLabelPanel
		s.setBaseLayout(types.LayoutThreePanel)

	default:
		s.tagPanel = nil
		s.labelPanel = nil
		if s.focus == types.FocusTagPanel || s.focus == types.FocusLabelPanel {
			s.focus = types.FocusList
		}
		s.setBaseLayout(types.LayoutSplit)
		// Restore full unfiltered list.
		if len(s.allPlans) > 0 {
			s.plans = s.allPlans
			s.updateListItems()
		}
	}

	s.sizeSidePanel()
}

// setBaseLayout records the layout to return to when leaving fullscreen, and
// switches to it immediately unless the screen is currently fullscreen.
func (s *PlansScreen) setBaseLayout(layout types.Layout) {
	s.baseLayout = layout
	if s.layout != types.LayoutFullscreen {
		s.layout = layout
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
	case types.FocusLabelPanel:
		return fmt.Sprintf("down/up: navigate labels | tab: plans | /: search | q: quit | Plans: %d", len(s.plans))
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
			searchHelp = fmt.Sprintf("/: search | T: tags | ctrl+l: clear [%s]", activeFilters)
		}
		tagNav := ""
		switch {
		case s.tagPanel != nil:
			tagNav = "shift+tab: tags | "
		case s.labelPanel != nil:
			tagNav = "shift+tab: labels | "
		}
		return fmt.Sprintf("down/up: navigate | m: manage tags | tab: content | %sv: fullscreen | e: edit | s: sync | S: settings | n: comments | r: rsync | d: dump | D: delete | R: rename | c: copy | C: connectors | X: summarize | %s | Plans: %d", tagNav, searchHelp, len(s.plans))
	case types.FocusContent:
		mode := "RAW"
		if s.viewer.RenderMode() == components.RenderModeGlamour {
			mode = "RENDERED"
		}
		lines := "OFF"
		if s.viewer.ShowLineNumbers() {
			lines = "ON"
		}
		if s.layout != types.LayoutFullscreen {
			return fmt.Sprintf("down/up: scroll | g/G: top/bottom | r: render (%s) | l: lines (%s) | c: copy | tab: list | esc: back", mode, lines)
		}
		return fmt.Sprintf("down/up: scroll | g/G: top/bottom | r: render (%s) | l: lines (%s) | c: copy | e: edit | v: versions | t: transmit | esc: back", mode, lines)
	case types.FocusEditor:
		modified := ""
		if s.editor.IsModified() {
			modified = " [MODIFIED]"
		}
		return fmt.Sprintf("ctrl+s: save | ctrl+l: go to line | tt/bb: top/bottom | dd: delete line | oo: new line | esc: cancel%s", modified)
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
	return s.focus == types.FocusEditor || s.focus == types.FocusSearch || s.focus == types.FocusTagFilter || s.activeModal != types.ModalNone
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
