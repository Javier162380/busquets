package screens

import (
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	tea "github.com/charmbracelet/bubbletea"
)

// HelpScreen displays keyboard shortcuts and help information.
type HelpScreen struct {
	width  int
	height int
}

// NewHelpScreen creates a new help screen.
func NewHelpScreen(width, height int) *HelpScreen {
	return &HelpScreen{
		width:  width,
		height: height,
	}
}

// Init initializes the screen.
func (s *HelpScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (s *HelpScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if msg, ok := msg.(tea.KeyMsg); ok {
		if msg.String() == "?" || msg.String() == "esc" || msg.String() == "q" {
			return s, func() tea.Msg {
				return CloseHelpMsg{}
			}
		}
	}
	return s, nil
}

// View renders the help screen.
func (s *HelpScreen) View() string {
	helpText := `
Claude Plan Viewer - Keyboard Shortcuts

PLANS LIST (Two-Panel mode):
  j/k, ↑/↓       Navigate plans list
  /              Open search bar
  c              Clear search (when search is active)
  v              Enter fullscreen view mode
  e              Enter edit mode
  s              Sync plans from source directory

SEARCH:
  Enter          Execute search
  Esc            Cancel search input

PLAN VIEW (Fullscreen mode):
  j/k, ↑/↓       Scroll up/down
  g              Jump to top
  G              Jump to bottom
  r              Toggle markdown rendering (raw vs HTML)
  e              Enter edit mode
  v              View version history
  Esc            Back to plans list

VERSION HISTORY (Two-Panel mode):
  j/k, ↑/↓       Navigate versions list
  v              View version fullscreen
  r              Restore this version
  Esc            Back to plan view

VERSION VIEW (Fullscreen mode):
  j/k, ↑/↓       Scroll up/down
  g              Jump to top
  G              Jump to bottom
  r              Restore this version
  Esc            Back to versions list

EDITING:
  Ctrl+S         Save and sync changes
  Esc            Cancel without saving

GENERAL:
  ?              Toggle this help screen
  q, Ctrl+C      Quit application

Press '?' or 'Esc' to close this help screen
`

	return "\n\n" + styles.HelpBoxStyle(s.width).Render(helpText)
}

// SetSize updates screen dimensions.
func (s *HelpScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// ShortHelp returns key binding help.
func (s *HelpScreen) ShortHelp() string {
	return "Press '?' or 'Esc' to close help"
}

// CloseHelpMsg requests closing the help screen.
type CloseHelpMsg struct{}
