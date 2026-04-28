package screens

import (
	"fmt"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/types"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SessionChatScreen handles interactive chat with a Claude session.
type SessionChatScreen struct {
	// Components
	chatView     *components.ChatMessagesViewer
	messageInput *components.MessageInput
	fileChanges  *components.FileChangesBar

	// State
	sessionUUID string
	session     *claudeviewer.SessionDetailInfo
	focus       types.Focus
	sending     bool // Whether a message is currently being sent

	// Dimensions
	width  int
	height int

	// Styles
	borderStyle lipgloss.Style

	// Theme
	isDarkModeEnabled bool
}

// NewSessionChatScreen creates a new session chat screen.
func NewSessionChatScreen(sessionUUID string, width, height int, isDarkModeEnabled bool) *SessionChatScreen {
	contentHeight := height - 8 // Leave room for input, file changes, borders

	return &SessionChatScreen{
		sessionUUID:  sessionUUID,
		chatView:     components.NewChatMessagesViewer(width-4, contentHeight),
		messageInput: components.NewMessageInput(width - 4),
		fileChanges:  components.NewFileChangesBar(width - 4),
		focus:        types.FocusMessageViewer,
		width:        width,
		height:       height,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
		isDarkModeEnabled: isDarkModeEnabled,
	}
}

// Init initializes the screen.
func (s *SessionChatScreen) Init() tea.Cmd {
	// Load session detail and start watching
	return tea.Batch(
		func() tea.Msg {
			return LoadSessionDetailMsg{SessionUUID: s.sessionUUID}
		},
		func() tea.Msg {
			return StartSessionWatchMsg{SessionUUID: s.sessionUUID}
		},
	)
}

// Update handles messages.
func (s *SessionChatScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case SessionDetailLoadedMsg:
		s.session = msg.Detail
		s.chatView.SetMessages(msg.Detail.Messages)
		s.fileChanges.SetChanges(msg.Detail.FileChanges)
		return s, nil

	case MessageSentMsg:
		s.sending = false
		if msg.Error == nil {
			// Clear input
			s.messageInput.Clear()
			// Message will be picked up by the watcher
		}
		return s, nil

	case SessionMessagesUpdatedMsg:
		// New messages from the session watcher
		if msg.SessionUUID == s.sessionUUID {
			s.chatView.AppendMessages(msg.NewMessages)
		}
		return s, nil

	case SessionFileChangesUpdatedMsg:
		if msg.SessionUUID == s.sessionUUID {
			s.fileChanges.SetChanges(msg.Changes)
		}
		return s, nil
	}

	// Delegate to focused component
	switch s.focus {
	case types.FocusMessageInput:
		return s, s.messageInput.Update(msg)
	case types.FocusMessageViewer:
		return s, s.chatView.Update(msg)
	default:
		return s, nil
	}
}

// View renders the screen.
func (s *SessionChatScreen) View() string {
	title := "Session Chat"
	if s.session != nil {
		title = fmt.Sprintf("Session: %s", s.session.Session.ProjectName)
		if s.session.Session.PlanTitle != nil {
			title += fmt.Sprintf(" (Plan: %s)", *s.session.Session.PlanTitle)
		}
	}

	titleStyle := styles.MetaStyle
	header := titleStyle.Width(s.width - 4).Render(title)

	// Chat messages viewport
	messagesView := s.chatView.View()

	// File changes bar
	fileChangesView := s.fileChanges.View()

	// Message input
	inputView := s.messageInput.View()

	// Status line
	statusLine := ""
	if s.sending {
		statusLine = styles.AccentStyle.Render("Sending message...")
	} else {
		statusLine = styles.AccentStyle.Render("Press 'i' to start typing, 'esc' to go back")
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		header,
		messagesView,
		"",
		fileChangesView,
		"",
		inputView,
		statusLine,
	)

	return styles.FocusedBorderStyle.
		Width(s.width - 2).
		Height(s.height - 2).
		Render(content)
}

// SetSize updates the screen dimensions.
func (s *SessionChatScreen) SetSize(width, height int) {
	s.width = width
	s.height = height

	contentHeight := height - 8
	s.chatView.SetSize(width-4, contentHeight)
	s.messageInput.SetSize(width - 4)
	s.fileChanges.SetSize(width - 4)
}

// ShortHelp returns help text for the status bar.
func (s *SessionChatScreen) ShortHelp() string {
	switch s.focus {
	case types.FocusMessageViewer:
		return "j/k: scroll • i: type message • f: view files • esc: back"
	case types.FocusMessageInput:
		return "enter: send • esc: cancel"
	default:
		return "esc: back"
	}
}

// IsInputMode returns whether the screen is in input mode.
func (s *SessionChatScreen) IsInputMode() bool {
	return s.focus == types.FocusMessageInput
}

// handleKey processes keyboard input.
func (s *SessionChatScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch s.focus {
	case types.FocusMessageViewer:
		return s.handleViewerKey(msg)
	case types.FocusMessageInput:
		return s.handleInputKey(msg)
	default:
		return s, nil
	}
}

// handleViewerKey processes keys when viewing messages.
func (s *SessionChatScreen) handleViewerKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Stop watching and return to sessions screen
		return s, tea.Batch(
			func() tea.Msg {
				return StopSessionWatchMsg{SessionUUID: s.sessionUUID}
			},
			func() tea.Msg {
				return PopScreenMsg{}
			},
		)

	case "i":
		// Enter input mode
		s.focus = types.FocusMessageInput
		return s, s.messageInput.Focus()

	case "f":
		// TODO: Show file changes detail view
		return s, nil

	case "p":
		// View associated plan if exists
		if s.session != nil && s.session.AssociatedPlan != nil {
			return s, func() tea.Msg {
				return LoadPlanDetailMsg{FileName: s.session.AssociatedPlan.FileName}
			}
		}
		return s, nil

	default:
		// Delegate to chat viewer for scrolling
		return s, s.chatView.Update(msg)
	}
}

// handleInputKey processes keys when typing a message.
func (s *SessionChatScreen) handleInputKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Cancel input and return to viewer
		s.focus = types.FocusMessageViewer
		s.messageInput.Clear()
		s.messageInput.Blur()
		return s, nil

	case "enter":
		// Send message
		message := s.messageInput.Value()
		if message != "" && !s.sending {
			s.sending = true
			s.focus = types.FocusMessageViewer
			s.messageInput.Blur()
			return s, func() tea.Msg {
				return SendSessionMessageMsg{
					SessionUUID: s.sessionUUID,
					Content:     message,
				}
			}
		}
		return s, nil

	default:
		// Delegate to message input
		return s, s.messageInput.Update(msg)
	}
}

type SendSessionMessageMsg struct {
	SessionUUID string
	Content     string
}

type MessageSentMsg struct {
	Success bool
	Error   error
}

type SessionMessagesUpdatedMsg struct {
	SessionUUID string
	NewMessages []claudeviewer.SessionMessageInfo
}

type SessionFileChangesUpdatedMsg struct {
	SessionUUID string
	Changes     []claudeviewer.SessionFileChangeInfo
}

type StartSessionWatchMsg struct {
	SessionUUID string
}

type StopSessionWatchMsg struct {
	SessionUUID string
}
