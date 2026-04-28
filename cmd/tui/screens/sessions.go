package screens

import (
	"fmt"
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/types"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SessionsScreen handles session browsing and navigation.
type SessionsScreen struct {
	// Components
	list      *components.List
	chatView  *components.ChatMessagesViewer
	searchBar *components.SearchBar

	// State
	layout   types.Layout
	focus    types.Focus
	sessions []claudeviewer.SessionSummaryInfo
	current  *claudeviewer.SessionDetailInfo

	// Dimensions
	width  int
	height int

	// Styles
	borderStyle lipgloss.Style

	// Theme
	isDarkModeEnabled bool
}

// NewSessionsScreen creates a new sessions screen.
func NewSessionsScreen(width, height int, isDarkModeEnabled bool) *SessionsScreen {
	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	return &SessionsScreen{
		list:      components.NewList(nil, panelWidth, contentHeight),
		chatView:  components.NewChatMessagesViewer(panelWidth, contentHeight),
		searchBar: components.NewSearchBar(panelWidth),
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
func (s *SessionsScreen) Init() tea.Cmd {
	return nil // Sessions loaded via SessionsLoadedMsg from App
}

// Update handles messages.
func (s *SessionsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case SessionsLoadedMsg:
		s.sessions = msg.Sessions
		s.updateListItems()
		if len(s.sessions) > 0 {
			return s, s.loadSessionDetail(s.sessions[0].SessionUUID)
		}
		return s, nil

	case SessionDetailLoadedMsg:
		s.current = msg.Detail
		s.chatView.SetMessages(msg.Detail.Messages)
		return s, nil

	case SessionCreatedMsg:
		if msg.Error == nil {
			// Open the chat screen for the new session
			return s, func() tea.Msg {
				return OpenSessionChatMsg{SessionUUID: msg.Session.SessionUUID}
			}
		}
		return s, nil

	case DiscoverResultMsg:
		if msg.Error == nil {
			// Reload sessions after discovery
			return s, func() tea.Msg { return LoadSessionsMsg{} }
		}
		return s, nil
	}

	return s, nil
}

// View renders the screen.
func (s *SessionsScreen) View() string {
	if s.layout == types.LayoutFullscreen {
		return s.renderFullscreen()
	}
	return s.renderSplit()
}

// SetSize updates the screen dimensions.
func (s *SessionsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height

	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	s.list.SetSize(panelWidth, contentHeight)
	s.chatView.SetSize(panelWidth, contentHeight)
	s.searchBar.SetSize(panelWidth)
}

// ShortHelp returns help text for the status bar.
func (s *SessionsScreen) ShortHelp() string {
	switch s.focus {
	case types.FocusList:
		return "j/k: navigate • enter: open chat • n: new session • d: discover • esc: back"
	case types.FocusContent:
		return "j/k: scroll • tab: switch panel • esc: back"
	default:
		return "esc: back"
	}
}

// IsInputMode returns whether the screen is in input mode.
func (s *SessionsScreen) IsInputMode() bool {
	return s.focus == types.FocusSearch
}

// handleKey processes keyboard input.
func (s *SessionsScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch s.focus {
	case types.FocusList:
		return s.handleListKey(msg)
	case types.FocusContent:
		return s.handleContentKey(msg)
	case types.FocusSearch:
		return s.handleSearchKey(msg)
	default:
		return s, nil
	}
}

// handleListKey processes keys when list is focused.
func (s *SessionsScreen) handleListKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return PopScreenMsg{} }

	case "enter":
		// Open session chat
		if s.current != nil {
			return s, func() tea.Msg {
				return OpenSessionChatMsg{SessionUUID: s.current.Session.SessionUUID}
			}
		}

	case "n":
		// Create new standalone session
		return s, func() tea.Msg {
			return CreateStandaloneSessionMsg{}
		}

	case "d":
		// Discover sessions
		return s, func() tea.Msg {
			return DiscoverSessionsMsg{}
		}

	case "tab":
		s.focus = types.FocusContent
		return s, nil

	case "/":
		s.focus = types.FocusSearch
		return s, nil

	default:
		// Delegate to list
		cmd := s.list.Update(msg)
		// Check if selection changed
		if s.list.SelectedItem() != nil {
			selectedIdx := s.list.Index()
			if selectedIdx >= 0 && selectedIdx < len(s.sessions) {
				selected := s.sessions[selectedIdx]
				if s.current == nil || s.current.Session.SessionUUID != selected.SessionUUID {
					return s, s.loadSessionDetail(selected.SessionUUID)
				}
			}
		}
		return s, cmd
	}

	return s, nil
}

// handleContentKey processes keys when content is focused.
func (s *SessionsScreen) handleContentKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return PopScreenMsg{} }

	case "tab":
		s.focus = types.FocusList
		return s, nil

	default:
		// Delegate to chat view
		return s, s.chatView.Update(msg)
	}
}

// handleSearchKey processes keys during search.
func (s *SessionsScreen) handleSearchKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		s.focus = types.FocusList
		s.searchBar.Clear()
		return s, nil

	case "enter":
		s.focus = types.FocusList
		// TODO: Implement search functionality
		return s, nil

	default:
		return s, s.searchBar.Update(msg)
	}
}

// renderSplit renders the split-panel layout.
func (s *SessionsScreen) renderSplit() string {
	leftPanel := s.renderLeftPanel()
	rightPanel := s.renderRightPanel()

	divider := styles.VerticalDivider(s.height - 2)

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPanel,
		divider,
		rightPanel,
	)
}

// renderFullscreen renders fullscreen layout (not used yet).
func (s *SessionsScreen) renderFullscreen() string {
	return s.renderRightPanel()
}

// renderLeftPanel renders the session list panel.
func (s *SessionsScreen) renderLeftPanel() string {
	panelWidth := (s.width - 3) / 2

	var title string
	if len(s.sessions) == 0 {
		title = "Sessions (0)"
	} else {
		title = fmt.Sprintf("Sessions (%d)", len(s.sessions))
	}

	titleStyle := styles.TitleStyle
	if s.focus == types.FocusList {
		titleStyle = styles.FocusedTitleStyle
	}

	var content string
	if s.focus == types.FocusSearch {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Width(panelWidth).Render(title),
			s.searchBar.View(),
			s.list.View(),
		)
	} else {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Width(panelWidth).Render(title),
			s.list.View(),
		)
	}

	borderStyle := styles.BlurredBorderStyle
	if s.focus == types.FocusList {
		borderStyle = styles.FocusedBorderStyle
	}

	return borderStyle.
		Width(panelWidth).
		Height(s.height - 2).
		Render(content)
}

// renderRightPanel renders the session preview panel.
func (s *SessionsScreen) renderRightPanel() string {
	panelWidth := (s.width - 3) / 2

	title := "Session Preview"
	if s.current != nil {
		title = fmt.Sprintf("Session: %s", s.current.Session.ProjectName)
	}

	titleStyle := styles.TitleStyle
	if s.focus == types.FocusContent {
		titleStyle = styles.FocusedTitleStyle
	}

	var content string
	if s.current == nil {
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Width(panelWidth).Render(title),
			styles.DimStyle.Render("Select a session to view details"),
		)
	} else {
		// Show session info and recent messages
		info := s.renderSessionInfo()
		content = lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Width(panelWidth).Render(title),
			info,
			s.chatView.View(),
		)
	}

	borderStyle := styles.BlurredBorderStyle
	if s.focus == types.FocusContent {
		borderStyle = styles.FocusedBorderStyle
	}

	return borderStyle.
		Width(panelWidth).
		Height(s.height - 2).
		Render(content)
}

// renderSessionInfo renders session metadata.
func (s *SessionsScreen) renderSessionInfo() string {
	if s.current == nil {
		return ""
	}

	var parts []string

	parts = append(parts, styles.DimStyle.Render(fmt.Sprintf("UUID: %s", s.current.Session.SessionUUID)))
	parts = append(parts, styles.DimStyle.Render(fmt.Sprintf("Status: %s", s.current.Session.Status)))

	if s.current.Session.PlanTitle != nil {
		parts = append(parts, styles.DimStyle.Render(fmt.Sprintf("Plan: %s", *s.current.Session.PlanTitle)))
	}

	parts = append(parts, styles.DimStyle.Render(fmt.Sprintf("Messages: %d", len(s.current.Messages))))

	if len(s.current.FileChanges) > 0 {
		parts = append(parts, styles.DimStyle.Render(fmt.Sprintf("Files changed: %d", len(s.current.FileChanges))))
	}

	return strings.Join(parts, " • ") + "\n"
}

// updateListItems updates the list with current sessions.
func (s *SessionsScreen) updateListItems() {
	items := make([]components.ListItem, len(s.sessions))
	for i, session := range s.sessions {
		listItem := sessionListItem{session}
		items[i] = components.NewListItem(listItem.Title(), listItem.Description(), session)
	}
	s.list.SetItems(items)
}

// loadSessionDetail creates a command to load session details.
func (s *SessionsScreen) loadSessionDetail(sessionUUID string) tea.Cmd {
	return func() tea.Msg {
		return LoadSessionDetailMsg{SessionUUID: sessionUUID}
	}
}

// sessionListItem implements list.Item for sessions.
type sessionListItem struct {
	session claudeviewer.SessionSummaryInfo
}

func (i sessionListItem) Title() string {
	title := i.session.ProjectName
	if i.session.Slug != nil {
		title += fmt.Sprintf(" (%s)", *i.session.Slug)
	}
	return title
}

func (i sessionListItem) Description() string {
	desc := fmt.Sprintf("Messages: %d", i.session.MessageCount)
	if i.session.LastMessageAt != nil {
		desc += " • Last: " + *i.session.LastMessageAt
	}
	if i.session.PlanTitle != nil {
		desc += " • Plan: " + *i.session.PlanTitle
	}
	return desc
}

func (i sessionListItem) FilterValue() string {
	return i.session.ProjectName
}

type SessionsLoadedMsg struct {
	Sessions []claudeviewer.SessionSummaryInfo
}

type LoadSessionsMsg struct{}

type SessionDetailLoadedMsg struct {
	Detail *claudeviewer.SessionDetailInfo
}

type LoadSessionDetailMsg struct {
	SessionUUID string
}

type CreateStandaloneSessionMsg struct{}

type CreateSessionFromPlanMsg struct {
	PlanFileName string
}

type SessionCreatedMsg struct {
	Session *claudeviewer.SessionInfo
	Error   error
}

type OpenSessionChatMsg struct {
	SessionUUID string
}

type DiscoverSessionsMsg struct{}

type DiscoverResultMsg struct {
	Count int
	Error error
}

type OpenSessionsMsg struct{}
