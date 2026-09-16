package screens

import (
	"fmt"
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
)

// MemoryScreen shows a plan's memory beside the timeline it describes.
//
// The timeline is computed, so it renders even when no memory has been written;
// that is deliberately different from VersionsScreen, which refuses to open
// without data.
type MemoryScreen struct {
	// Components.
	list          *components.List
	viewer        *components.Viewer
	confirmDialog *components.ConfirmModal

	// State.
	layout     types.Layout
	focus      types.Focus
	planName   string
	syncSource string
	memory     *busquets.PlanMemory
	staleness  busquets.MemoryStaleness
	events     []busquets.MemoryEvent
	current    *busquets.MemoryEvent
	generating bool

	// Diff state, for the event selected with enter.
	diffViewport viewport.Model
	diffPlain    string

	// Dimensions.
	width  int
	height int

	borderStyle           lipgloss.Style
	isDarkModeEnabled     bool
	markdownRenderedTheme string
	screenOrientation     string
}

// NewMemoryScreen creates a memory screen for a plan.
func NewMemoryScreen(planName, syncSource, markdownRenderedTheme string, width, height int, isDarkModeEnabled, renderMarkdownByDefault bool, screenOrientation string) *MemoryScreen {
	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	s := &MemoryScreen{
		list:          components.NewList(nil, panelWidth, contentHeight, true),
		viewer:        components.NewViewer(panelWidth, contentHeight, markdownRenderedTheme),
		confirmDialog: components.NewConfirmModal(),
		layout:        types.LayoutSplit,
		focus:         types.FocusTimeline,
		planName:      planName,
		syncSource:    syncSource,
		width:         width,
		height:        height,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
		isDarkModeEnabled:     isDarkModeEnabled,
		markdownRenderedTheme: markdownRenderedTheme,
		screenOrientation:     screenOrientation,
	}

	if renderMarkdownByDefault {
		s.viewer.SetRenderMode(components.RenderModeGlamour)
	}

	return s
}

// NewMemoryScreenWithData creates a memory screen with its memory already loaded.
func NewMemoryScreenWithData(planName, syncSource, markdownRenderedTheme string, memory *busquets.PlanMemory, staleness busquets.MemoryStaleness, width, height int, isDarkModeEnabled, renderMarkdownByDefault bool, screenOrientation string) *MemoryScreen {
	s := NewMemoryScreen(planName, syncSource, markdownRenderedTheme, width, height, isDarkModeEnabled, renderMarkdownByDefault, screenOrientation)
	s.setMemory(memory, staleness)
	return s
}

// Init initialises the screen; data arrives via MemoryLoadedMsg when absent.
func (s *MemoryScreen) Init() tea.Cmd {
	if s.memory != nil {
		return nil
	}
	return func() tea.Msg {
		return messages.RequestMemoryScreenMsg{PlanName: s.planName, SyncSource: s.syncSource}
	}
}

// setMemory stores a loaded memory and refreshes the list and viewer.
func (s *MemoryScreen) setMemory(memory *busquets.PlanMemory, staleness busquets.MemoryStaleness) {
	firstLoad := s.memory == nil

	s.memory = memory
	s.staleness = staleness
	if memory != nil {
		s.events = memory.Events
	}
	s.updateListItems()

	if len(s.events) > 0 {
		// The timeline reads oldest-first, but the newest event is what someone
		// opening the screen wants; a reload keeps wherever they had scrolled to.
		selected := s.list.SelectedIndex()
		if firstLoad || selected < 0 || selected >= len(s.events) {
			selected = len(s.events) - 1
		}
		s.list.Select(selected)
		s.current = &s.events[selected]
	}
	s.refreshViewer()
}

// refreshViewer redraws the memory document pane.
func (s *MemoryScreen) refreshViewer() {
	if s.memory == nil {
		return
	}
	s.viewer.SetContent(content.NewMemoryContent(s.memory, s.getViewerWidth()))
}

// Update handles messages.
func (s *MemoryScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if s.confirmDialog.IsActive() {
		cmd := s.confirmDialog.Update(msg)
		return s, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case messages.MemoryLoadedMsg:
		if msg.Err == nil {
			s.setMemory(msg.Memory, msg.Staleness)
		}
		return s, nil

	case messages.MemoryGeneratedMsg:
		s.generating = false
		if msg.Err == nil {
			s.setMemory(msg.Memory, msg.Staleness)
		}
		return s, nil

	case messages.MemoryDiffLoadedMsg:
		if msg.Err != nil {
			return s, nil
		}
		s.showDiff(msg.Diff)
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

	var cmd tea.Cmd
	switch s.focus {
	case types.FocusTimeline:
		cmd = s.list.Update(msg)
	case types.FocusContent:
		cmd = s.viewer.Update(msg)
	default:
		return s, cmd
	}
	return s, cmd
}

// handleKey routes key input by focus.
func (s *MemoryScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()

	switch s.focus {
	case types.FocusTimeline:
		return s.handleTimelineKey(key, msg)
	case types.FocusContent:
		return s.handleContentKey(key, msg)
	case types.FocusDiff:
		return s.handleDiffKey(key, msg)
	default:
		return s, nil
	}
}

// handleTimelineKey handles keys while the timeline list has focus.
func (s *MemoryScreen) handleTimelineKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "esc":
		return s, func() tea.Msg { return messages.PopScreenMsg{} }

	case "tab":
		s.focus = types.FocusContent
		return s, nil

	case "enter":
		return s, s.requestDiff()

	case "r":
		return s, s.generate(busquets.MemoryModeIncremental)

	case "R":
		s.confirmDialog.Open(
			fmt.Sprintf("Rewrite the memory for %q from scratch? The current one is replaced.", s.planName),
			func() tea.Msg {
				return messages.GenerateMemoryMsg{
					PlanName: s.planName, SyncSource: s.syncSource, Mode: busquets.MemoryModeRebuild,
				}
			},
		)
		return s, nil

	case "c":
		return s, s.copyMemory()

	case "d":
		if s.memory == nil || !s.memory.Exists() {
			return s, func() tea.Msg {
				return messages.ErrorMsg{Error: fmt.Errorf("there is no memory to delete")}
			}
		}
		s.confirmDialog.Open(
			fmt.Sprintf("Delete the memory for %q? The plan and its versions are untouched.", s.planName),
			func() tea.Msg {
				return messages.DeleteMemoryMsg{PlanName: s.planName, SyncSource: s.syncSource}
			},
		)
		return s, nil

	case "v":
		s.layout = types.LayoutFullscreen
		s.focus = types.FocusContent
		s.refreshViewer()
		return s, nil

	case "j", "down", "k", "up":
		cmd := s.list.Update(msg)
		if item := s.list.SelectedItem(); item != nil {
			if event, ok := item.Data().(busquets.MemoryEvent); ok {
				s.current = &event
			}
		}
		return s, cmd
	}

	return s, s.list.Update(msg)
}

// handleContentKey handles keys while the memory document has focus.
func (s *MemoryScreen) handleContentKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "esc":
		if s.layout == types.LayoutFullscreen {
			s.layout = types.LayoutSplit
			s.focus = types.FocusTimeline
			s.viewer.GotoTop()
			s.refreshViewer()
			return s, nil
		}
		return s, func() tea.Msg { return messages.PopScreenMsg{} }

	case "tab":
		if s.layout == types.LayoutSplit {
			s.focus = types.FocusTimeline
		}
		return s, nil

	case "r":
		s.viewer.ToggleRenderMode()
		return s, nil

	case "l":
		s.viewer.ToggleLineNumbers()
		return s, nil

	case "c":
		return s, s.copyMemory()

	case "g":
		s.viewer.GotoTop()
		return s, nil

	case "G":
		s.viewer.GotoBottom()
		return s, nil
	}

	return s, s.viewer.Update(msg)
}

// handleDiffKey handles keys while a timeline event's diff is showing.
func (s *MemoryScreen) handleDiffKey(key string, msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch key {
	case "esc":
		s.layout = types.LayoutSplit
		s.focus = types.FocusTimeline
		return s, nil
	case "g":
		s.diffViewport.GotoTop()
		return s, nil
	case "G":
		s.diffViewport.GotoBottom()
		return s, nil
	case "c":
		return s, func() tea.Msg {
			return messages.CopyToClipboardMsg{Text: s.diffPlain, Label: "Memory event diff"}
		}
	}

	var cmd tea.Cmd
	s.diffViewport, cmd = s.diffViewport.Update(msg)
	return s, cmd
}

// requestDiff asks for the diff behind the selected event.
//
// Only a version that has a predecessor has one: the first snapshot is a
// baseline and a comment is not a change to the document.
func (s *MemoryScreen) requestDiff() tea.Cmd {
	if s.current == nil {
		return nil
	}

	if s.current.Kind == busquets.MemoryEventComment {
		return func() tea.Msg {
			return messages.ErrorMsg{Error: fmt.Errorf("a comment has no diff — read it in the memory document")}
		}
	}
	if s.current.VersionNumber == nil || *s.current.VersionNumber == 0 {
		return func() tea.Msg {
			return messages.ErrorMsg{Error: fmt.Errorf("the first version is a baseline, so it has no diff")}
		}
	}

	to := *s.current.VersionNumber
	return func() tea.Msg {
		return messages.RequestMemoryDiffMsg{
			PlanName: s.planName, SyncSource: s.syncSource, FromVersion: to - 1, ToVersion: to,
		}
	}
}

// showDiff switches into the fullscreen diff view.
func (s *MemoryScreen) showDiff(diff string) {
	s.diffPlain = diff
	s.diffViewport = viewport.New(s.width-4, s.height-8)
	s.diffViewport.SetContent(colorizeDiff(diff))
	s.focus = types.FocusDiff
}

// generate requests a generation run, refusing to start a second one.
func (s *MemoryScreen) generate(mode busquets.MemoryMode) tea.Cmd {
	if s.generating {
		return func() tea.Msg {
			return messages.ErrorMsg{Error: fmt.Errorf("a memory is already being written")}
		}
	}
	if mode == busquets.MemoryModeIncremental && s.memory != nil && s.memory.Exists() && !s.staleness.IsStale() {
		return func() tea.Msg {
			return messages.ErrorMsg{Error: fmt.Errorf("this memory is already up to date (R rewrites it)")}
		}
	}

	s.generating = true
	return func() tea.Msg {
		return messages.GenerateMemoryMsg{PlanName: s.planName, SyncSource: s.syncSource, Mode: mode}
	}
}

// copyMemory copies the memory markdown to the clipboard.
func (s *MemoryScreen) copyMemory() tea.Cmd {
	if s.memory == nil || !s.memory.Exists() {
		return func() tea.Msg {
			return messages.ErrorMsg{Error: fmt.Errorf("there is no memory to copy")}
		}
	}
	return func() tea.Msg {
		return messages.CopyToClipboardMsg{Text: s.memory.Content, Label: "Memory: " + s.planName}
	}
}

// SetGenerating reflects whether a run is in flight, so a failure re-enables the key.
func (s *MemoryScreen) SetGenerating(generating bool) {
	s.generating = generating
}

// updateListItems rebuilds the timeline rows from the computed events.
func (s *MemoryScreen) updateListItems() {
	items := make([]components.ListItem, len(s.events))
	for i, event := range s.events {
		items[i] = components.NewListItem(
			memoryEventTitle(event),
			fmt.Sprintf("%s | %s", event.OccurredAt.Format("2006-01-02 15:04"), memoryEventDetail(event)),
			event,
		)
	}
	s.list.SetItems(items)
}

// memoryEventTitle labels a timeline row from computed facts.
func memoryEventTitle(event busquets.MemoryEvent) string {
	if event.Kind == busquets.MemoryEventComment {
		return "Comment"
	}
	if event.VersionNumber == nil {
		return "Version"
	}
	return fmt.Sprintf("v%d", *event.VersionNumber)
}

// memoryEventDetail summarises what an event did.
func memoryEventDetail(event busquets.MemoryEvent) string {
	switch {
	case event.Kind == busquets.MemoryEventComment:
		return firstLine(event.Body)
	case event.Kind == busquets.MemoryEventRestore && event.RestoredFrom != nil:
		return fmt.Sprintf("restored from v%d", *event.RestoredFrom)
	case event.LinesAdded == 0 && event.LinesRemoved == 0:
		return fmt.Sprintf("initial | %d words", event.WordCount)
	default:
		return fmt.Sprintf("+%d −%d", event.LinesAdded, event.LinesRemoved)
	}
}

// firstLine returns text's first line, trimmed for a single-row list entry.
func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	if len(line) > 60 {
		return line[:57] + "..."
	}
	return line
}

// View renders the screen.
func (s *MemoryScreen) View() string {
	if s.focus == types.FocusDiff {
		return s.renderDiffView()
	}

	var mainContent string
	if s.layout == types.LayoutFullscreen {
		mainContent = s.renderFullscreenViewer()
	} else {
		mainContent = s.renderSplitView()
	}

	if s.confirmDialog.IsActive() {
		return overlayContent(mainContent, s.width, s.height, lipgloss.Center, lipgloss.Center, s.confirmDialog.View())
	}
	return mainContent
}

// renderSplitView renders the timeline beside the memory document.
func (s *MemoryScreen) renderSplitView() string {
	vertical := s.screenOrientation == busquets.ScreenOrientationVertical

	var panelWidth, listPanelHeight, contentPanelHeight int
	if vertical {
		panelWidth = s.width - 2
		available := s.height - types.VerticalHeightOverhead
		listPanelHeight = available * types.VerticalListRatioNum / types.VerticalListRatioDenom
		contentPanelHeight = available - listPanelHeight
	} else {
		panelWidth = (s.width - types.HorizontalPanelWidthOverhead) / 2
		listPanelHeight = s.height - types.HorizontalHeightOverhead
		contentPanelHeight = s.height - types.HorizontalHeightOverhead
	}

	s.list.SetSize(panelWidth-4, listPanelHeight-4)
	s.viewer.SetSize(panelWidth-4, contentPanelHeight-4)

	leftPanel := s.borderStyle.Width(panelWidth).Height(listPanelHeight).Render(s.list.View())
	rightPanel := s.borderStyle.Width(panelWidth).Height(contentPanelHeight).Render(s.viewer.View())

	if vertical {
		return lipgloss.JoinVertical(lipgloss.Left, leftPanel, strings.Repeat("─", panelWidth), rightPanel)
	}

	var divider strings.Builder
	for range listPanelHeight {
		divider.WriteString("│\n")
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, strings.TrimSuffix(divider.String(), "\n"), rightPanel)
}

// renderFullscreenViewer renders the memory document fullscreen.
func (s *MemoryScreen) renderFullscreenViewer() string {
	contentHeight := s.height - 4
	s.viewer.SetSize(s.width-4, contentHeight-4)
	return s.borderStyle.Width(s.width).Height(contentHeight).Render(s.viewer.View())
}

// renderDiffView renders a timeline event's diff fullscreen.
func (s *MemoryScreen) renderDiffView() string {
	contentHeight := s.height - 4
	s.diffViewport.Width = s.width - 4
	s.diffViewport.Height = contentHeight - 4
	return s.borderStyle.Width(s.width).Height(contentHeight).Render(s.diffViewport.View())
}

// SetSize updates screen dimensions.
func (s *MemoryScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// SetScreenOrientation updates the panel arrangement.
func (s *MemoryScreen) SetScreenOrientation(orientation string) {
	s.screenOrientation = orientation
}

// UpdateDarkMode updates the dark mode setting and redraws.
func (s *MemoryScreen) UpdateDarkMode(enabled bool) {
	s.isDarkModeEnabled = enabled
	s.refreshViewer()
}

// RenderedMarkdownByDefault switches the viewer's render mode.
func (s *MemoryScreen) RenderedMarkdownByDefault(enabled bool) {
	renderMode := components.RenderModeRaw
	if enabled {
		renderMode = components.RenderModeGlamour
	}
	if s.viewer.RenderMode() != renderMode {
		s.viewer.SetRenderMode(renderMode)
		s.refreshViewer()
	}
}

// RenderedMarkdownTheme changes the markdown theme.
func (s *MemoryScreen) RenderedMarkdownTheme(theme string) {
	s.markdownRenderedTheme = theme
	s.viewer.SetMarkdownTheme(theme)
}

// getViewerWidth calculates the viewer width for the current layout.
func (s *MemoryScreen) getViewerWidth() int {
	if s.layout == types.LayoutFullscreen {
		return s.width - 4
	}
	if s.screenOrientation == busquets.ScreenOrientationVertical {
		return s.width - 2 - 4
	}
	return (s.width-3)/2 - 4
}

// ShortHelp returns key binding help.
func (s *MemoryScreen) ShortHelp() string {
	if s.confirmDialog.IsActive() {
		return "←/→: select  y: yes  n/esc: cancel  enter: confirm"
	}
	if s.focus == types.FocusDiff {
		return "j/k, ↑/↓: scroll | g/G: top/bottom | c: copy diff | esc: back"
	}

	mode := "RAW"
	if s.viewer.RenderMode() == components.RenderModeGlamour {
		mode = "RENDERED"
	}

	status := "no memory yet — r: write it"
	switch {
	case s.generating:
		status = "writing..."
	case s.memory != nil && s.memory.Exists() && s.staleness.IsStale():
		status = fmt.Sprintf("%d new events — r: update", s.staleness.NewVersions+s.staleness.NewComments)
	case s.memory != nil && s.memory.Exists():
		status = "up to date"
	}

	switch s.focus {
	case types.FocusTimeline:
		return fmt.Sprintf("down/up: navigate | enter: diff | tab: memory | v: fullscreen | r: update | R: rewrite | c: copy | d: delete | esc: back | %s | Events: %d",
			status, len(s.events))
	case types.FocusContent:
		return fmt.Sprintf("down/up: scroll | g/G: top/bottom | tab: timeline | r: render (%s) | l: lines | c: copy | esc: back | %s",
			mode, status)
	default:
		return ""
	}
}

// IsInputMode returns true when capturing text input.
func (s *MemoryScreen) IsInputMode() bool {
	return s.confirmDialog.IsActive()
}

// EditorMode reports whether the screen is editing; a memory is never edited here.
func (s *MemoryScreen) EditorMode() bool {
	return false
}
