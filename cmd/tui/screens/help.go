package screens

import (
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

const helpContent = `Claude Plan Viewer - Keyboard Shortcuts

PLANS LIST (Two-Panel mode):
  j/k, ↑/↓       Navigate plans list
  Tab            Switch to content panel (right)
  /              Open search bar
  c              Clear search (when search is active)
  v              Enter fullscreen view mode
  e              Enter edit mode
  s              Sync plans from source directory
  d              Dump all plans from database to source directory

PLAN CONTENT (Two-Panel mode, right panel):
  j/k, ↑/↓       Scroll up/down
  g              Jump to top
  G              Jump to bottom
  r              Toggle markdown rendering (raw vs HTML)
  Tab            Switch to plans list (left)
  Esc            Back to plans list

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
  t              Transmit to connector (e.g., Telegram)
  Esc            Back to split view

VERSION HISTORY (Two-Panel mode):
  j/k, ↑/↓       Navigate versions list
  /              Open search bar
  v              View version fullscreen
  r              Restore this version
  Esc            Back to plan view

VERSION VIEW (Fullscreen mode):
  j/k, ↑/↓       Scroll up/down
  g              Jump to top
  G              Jump to bottom
  r              Restore this version
  Esc            Back to versions list

CONNECTORS (Two-Panel mode):
  j/k, ↑/↓       Navigate list / settings
  Tab            Switch between panels
  Enter          Enable connector (left panel)
  e              Edit setting (right panel)
  d              Disable connector
  Esc            Back to plans / Cancel edit

EDITING:
  Ctrl+S         Save and sync changes
  Esc            Cancel without saving

SETTINGS:
  j/k, ↑/↓       Navigate settings
  Enter/Space    Toggle boolean / Edit number
  0-9            Type digits (in edit mode)
  Backspace      Delete digit (in edit mode)
  Enter          Save (in edit mode)
  Esc            Cancel edit / Go back

GENERAL:
  S              Open settings
  ?              Toggle this help screen
  q, Ctrl+C      Quit application`

// HelpScreen displays keyboard shortcuts and help information.
type HelpScreen struct {
	width    int
	height   int
	viewport viewport.Model
}

// NewHelpScreen creates a new help screen.
func NewHelpScreen(width, height int) *HelpScreen {
	s := &HelpScreen{width: width, height: height}
	s.viewport = viewport.New(s.vpWidth(), s.vpHeight())
	s.viewport.SetContent(helpContent)
	return s
}

func (s *HelpScreen) vpWidth() int {
	// box: Width(width-4) content area, Padding(2,4) = 8 LR, Border = 2 LR → content = width-14
	w := s.width - 14
	if w < 20 {
		return 20
	}
	return w
}

func (s *HelpScreen) vpHeight() int {
	// "\n\n" (2) + border top (1) + padding top (2) + padding bot (2) + border bot (1) = 8
	h := s.height - 8
	if h < 5 {
		return 5
	}
	return h
}

// Init initializes the screen.
func (s *HelpScreen) Init() tea.Cmd {
	return nil
}

// Update handles messages.
func (s *HelpScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "?", "esc", "q":
			return s, func() tea.Msg {
				return messages.CloseHelpMsg{}
			}
		}
	}
	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// View renders the help screen.
func (s *HelpScreen) View() string {
	return "\n\n" + styles.HelpBoxStyle(s.width).Render(s.viewport.View())
}

// SetSize updates screen dimensions.
func (s *HelpScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.viewport.Width = s.vpWidth()
	s.viewport.Height = s.vpHeight()
}

// ShortHelp returns key binding help.
func (s *HelpScreen) ShortHelp() string {
	return "Press '?' or 'Esc' to close help"
}

// IsInputMode returns true when capturing text input.
func (s *HelpScreen) IsInputMode() bool {
	return false
}

// EditorMode ...
func (s *HelpScreen) EditorMode() bool {
	return false
}
