package screens

import (
	"github.com/Javier162380/busquets/cmd/tui/messages"
	"github.com/Javier162380/busquets/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
)

const helpContent = `Busquets - Keyboard Shortcuts

GENERAL:
  ?              Toggle this help screen
  q, Ctrl+C      Quit application
  S              Open settings
  C              Open connectors

PLANS LIST (left panel):
  j/k, ↑/↓       Navigate plans list
  Tab            Switch to content panel (right)
  Shift+Tab      Switch to the tag / label panel (three-panel mode)
  /              Search plans
  T              Filter by tags
  Ctrl+L         Clear active search / tag filters
  c              Copy plan content to clipboard
  m              Manage tags for the selected plan
  n              View / add comments
  X              Generate (or regenerate) a TLDR summary
  v              Open fullscreen view
  e              Edit the plan
  R              Rename the plan file
  D              Delete the plan
  s              Sync plans from the source directory
  r              Rsync indexed plans back to the source directory
  d              Dump all plans from the database to the source directory

PLAN CONTENT (right panel / fullscreen):
  j/k, ↑/↓       Scroll up/down
  g / G          Jump to top / bottom
  r              Toggle markdown rendering (raw vs rendered)
  l              Toggle line numbers
  c              Copy plan content to clipboard
  Ctrl+L         Jump to a specific line
  /              Search within the plan content (fullscreen only)
  N / P          Jump to next / previous search match (fullscreen only)
  Ctrl+U         Clear the search highlight (fullscreen only)
  e              Edit the plan
  v              View version history
  t              Transmit to a connector (e.g. Telegram)
  Tab            Switch panel
  Esc            Back to the plans list / split view

EDITING:
  Ctrl+S         Save and sync changes
  Ctrl+L         Jump to a specific line
  tt / bb        Jump to first / last line
  dd             Delete the current line
  oo             Insert a new line below
  Esc            Cancel without saving

SEARCH:
  Enter          Execute search
  Esc            Cancel search input

CONTENT SEARCH (/ in fullscreen plan content or version content):
  Enter          Jump to the first match
  Esc            Cancel and clear the highlight

TAG FILTER:
  Enter          Apply the tag filter
  Ctrl+T         Toggle match mode (AND / OR)
  Esc            Cancel

TAG PANEL (three-panel mode):
  ↑/↓            Navigate tags (filters the plan list)
  Tab            Switch to the plans list
  /              Search plans
  n              Create a new tag
  Enter          Confirm new tag (while creating)
  Esc            Cancel new tag (while creating)

LABEL PANEL (three-panel mode):
  ↑/↓            Navigate sync labels (filters the plan list)
                 Each label shows its source path beneath it
  Tab            Switch to the plans list
  /              Search plans

MANAGE TAGS (m):
  Space          Toggle the selected tag on the plan
  Tab            Switch between tag list and input
  Enter          Add the typed tag
  d              Delete the selected tag (with confirmation)
  Esc            Save and close

COMMENTS (n):
  ↑/↓            Navigate comments
  Tab            Switch between comment list and input
  Ctrl+S         Save the comment
  d              Delete the selected comment (with confirmation)
  Esc            Cancel / close

RENAME (R):
  (type)         Enter the new file name
  Enter          Continue to confirmation
  Esc            Cancel

SUMMARY POPUP (X):
  s              Save the summary as a comment
  q, Esc         Close

CONFIRMATION DIALOG:
  ←/→            Select Yes / No
  Enter          Confirm the selection
  Esc            Cancel

VERSION HISTORY (left panel):
  j/k, ↑/↓       Navigate versions
  g / G          Jump to top / bottom
  Tab            Switch to content panel
  /              Search versions
  Ctrl+L         Clear the version search
  v              View the version fullscreen
  R              Restore the selected version
  r              Toggle markdown rendering (raw vs rendered)
  l              Toggle line numbers
  c              Copy the version content to clipboard
  Esc            Back to the plan

VERSION VIEW (fullscreen):
  j/k, ↑/↓       Scroll up/down
  g / G          Jump to top / bottom
  Tab            Switch to the versions list
  R              Restore this version
  r              Toggle markdown rendering (raw vs rendered)
  l              Toggle line numbers
  c              Copy the version content to clipboard
  Ctrl+L         Jump to a specific line
  /              Search within the version content
  N / P          Jump to next / previous search match
  Ctrl+U         Clear the search highlight
  Esc            Back to the versions list

CONNECTORS (left panel):
  j/k, ↑/↓       Navigate connectors
  t              Set as the transmit connector
  X              Set as the summarizer connector
  d              Clear the connector's role
  V              Validate the connector's settings
  Tab            Switch to the settings panel
  Esc            Back to the plans list

CONNECTOR SETTINGS (right panel):
  j/k, ↑/↓       Navigate settings
  e              Edit the selected setting
  s              Show / hide secret values
  V              Validate the connector's settings
  Tab            Switch to the connectors list
  Esc            Back
  Enter          Save (while editing)

SETTINGS:
  j/k, ↑/↓       Navigate settings
  Enter/Space    Toggle boolean / edit number
  0-9            Type digits (while editing)
  Backspace      Delete a digit (while editing)
  Enter          Save (while editing)
  Esc            Cancel edit / go back`

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
