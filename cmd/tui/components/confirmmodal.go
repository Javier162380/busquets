package components

import (
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConfirmModal is a generic yes/no confirmation overlay.
// Activate it with Open, passing the message to emit when the user confirms.
// The default selection is "No" to prevent accidental destructive actions.
type ConfirmModal struct {
	isActive   bool
	message    string
	pendingMsg tea.Msg
	confirmed  bool // true = Yes selected
	width      int
}

// NewConfirmModal creates a new confirm modal.
func NewConfirmModal() *ConfirmModal {
	return &ConfirmModal{width: 44}
}

// Open activates the modal with the given prompt and the message to emit on confirmation.
func (d *ConfirmModal) Open(message string, onConfirm tea.Msg) {
	d.isActive = true
	d.message = message
	d.pendingMsg = onConfirm
	d.confirmed = false // default to No
}

// Close deactivates the modal without emitting anything.
func (d *ConfirmModal) Close() {
	d.isActive = false
	d.pendingMsg = nil
}

// IsActive returns whether the modal is currently shown.
func (d *ConfirmModal) IsActive() bool {
	return d.isActive
}

// Update handles keyboard input for the modal and returns a command.
func (d *ConfirmModal) Update(msg tea.Msg) tea.Cmd {
	if !d.isActive {
		return nil
	}

	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch keyMsg.String() {
	case "left", "h", "tab":
		d.confirmed = true
	case "right", "l":
		d.confirmed = false
	case "y":
		pending := d.pendingMsg
		d.Close()
		return func() tea.Msg { return pending }
	case "n", "esc":
		d.Close()
		return nil
	case "enter":
		if d.confirmed {
			pending := d.pendingMsg
			d.Close()
			return func() tea.Msg { return pending }
		}
		d.Close()
		return nil
	}

	return nil
}

// View renders the confirmation modal.
func (d *ConfirmModal) View() string {
	if !d.isActive {
		return ""
	}

	dialogStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.ErrorColor).
		Padding(1, 2).
		Width(d.width)

	titleStyle := lipgloss.NewStyle().
		Foreground(styles.ErrorColor).
		Bold(true).
		MarginBottom(1)

	// Wrap message text to fit inside the dialog.
	messageStyle := lipgloss.NewStyle().
		Foreground(styles.ForegroundColor).
		Width(d.width - 4)

	selectedBtnStyle := lipgloss.NewStyle().
		Background(styles.AccentColor).
		Foreground(lipgloss.Color("0")).
		Bold(true).
		Padding(0, 2)

	unselectedBtnStyle := lipgloss.NewStyle().
		Foreground(styles.MutedColor).
		Padding(0, 2)

	var sb strings.Builder

	sb.WriteString(titleStyle.Render("Confirm Action"))
	sb.WriteString("\n")
	sb.WriteString(messageStyle.Render(d.message))
	sb.WriteString("\n\n")

	yesStyle := unselectedBtnStyle
	noStyle := unselectedBtnStyle
	if d.confirmed {
		yesStyle = selectedBtnStyle
	} else {
		noStyle = selectedBtnStyle
	}

	buttons := lipgloss.JoinHorizontal(
		lipgloss.Top,
		yesStyle.Render("Yes"),
		"  ",
		noStyle.Render("No"),
	)
	sb.WriteString(buttons)
	sb.WriteString("\n\n")

	helpStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
	sb.WriteString(helpStyle.Render("←/→: select  y/n: choose  enter: confirm  esc: cancel"))

	return dialogStyle.Render(sb.String())
}
