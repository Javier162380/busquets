package screens

import (
	"fmt"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TLDRScreen displays a LLM-generated summary of a plan.
type TLDRScreen struct {
	viewport    viewport.Model
	planTitle   string
	width       int
	height      int
	borderStyle lipgloss.Style
}

// NewTLDRScreen creates a new TLDR screen with the given summary text.
func NewTLDRScreen(planTitle, summary string, width, height int) *TLDRScreen {
	contentHeight := height - 6 // leave room for border + title + help

	vp := viewport.New(width-4, contentHeight)
	vp.SetContent(summary)

	return &TLDRScreen{
		viewport:  vp,
		planTitle: planTitle,
		width:     width,
		height:    height,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
	}
}

// Init satisfies the tea.Model interface; data is already loaded.
func (s *TLDRScreen) Init() tea.Cmd { return nil }

// Update handles key events for the TLDR screen.
func (s *TLDRScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "q":
			return s, func() tea.Msg { return messages.PopScreenMsg{} }
		case "g":
			s.viewport.GotoTop()
			return s, nil
		case "G":
			s.viewport.GotoBottom()
			return s, nil
		}
	}

	var cmd tea.Cmd
	s.viewport, cmd = s.viewport.Update(msg)
	return s, cmd
}

// View renders the TLDR screen.
func (s *TLDRScreen) View() string {
	contentHeight := s.height - 6

	title := styles.TitleStyle.Render(fmt.Sprintf("TLDR: %s", s.planTitle))

	panel := s.borderStyle.
		Width(s.width).
		Height(contentHeight).
		Render(s.viewport.View())

	return lipgloss.JoinVertical(lipgloss.Left, title, panel)
}

// ShortHelp returns key binding help text.
func (s *TLDRScreen) ShortHelp() string {
	return "↑/↓: scroll • g/G: top/bottom • esc: back"
}

// IsInputMode always returns false; TLDR screen has no text input.
func (s *TLDRScreen) IsInputMode() bool { return false }

// SetSize updates screen dimensions.
func (s *TLDRScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
	contentHeight := height - 6
	s.viewport.Width = width - 4
	s.viewport.Height = contentHeight
}
