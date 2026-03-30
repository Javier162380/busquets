// Package styles provides centralized styling for the TUI.
package styles

import "github.com/charmbracelet/lipgloss"

// Color palette.
var (
	// Primary accent color.
	AccentColor = lipgloss.Color("212") // Magenta.

	// UI colors.
	HeaderBgColor   = lipgloss.Color("235") // Dark gray.
	BorderColor     = lipgloss.Color("238") // Darker gray.
	MutedColor      = lipgloss.Color("244") // Gray.
	InactiveColor   = lipgloss.Color("240") // Dim gray.
	ForegroundColor = lipgloss.Color("250") // Light gray.

	// Message colors.
	ErrorColor   = lipgloss.Color("160") // Dark red.
	SuccessColor = lipgloss.Color("42")  // Green.
	LoadingColor = lipgloss.Color("81")  // Cyan.
	HelpColor    = lipgloss.Color("206") // Pink.
)

// Text styles.
var (
	AccentStyle = lipgloss.NewStyle().
			Foreground(AccentColor)

	ActiveStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	InactiveStyle = lipgloss.NewStyle().
			Foreground(InactiveColor)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(ErrorColor).
			Bold(true)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(SuccessColor).
			Bold(true)

	LoadingStyle = lipgloss.NewStyle().
			Foreground(LoadingColor).
			Bold(true)

	HelpStyle = lipgloss.NewStyle().
			Foreground(ForegroundColor).
			Italic(true)

	MutedStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	TitleStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	MetaStyle = lipgloss.NewStyle().
			Foreground(ForegroundColor).
			Italic(true)
)

// HeaderStyle returns a style for the header bar.
func HeaderStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 2).
		Background(HeaderBgColor).
		Foreground(ForegroundColor)
}

// StatusStyle returns a style for the status bar.
func StatusStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 2).
		Background(HeaderBgColor).
		Foreground(MutedColor)
}

// PanelStyle returns a style for a bordered panel.
func PanelStyle(width, height int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Height(height).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(BorderColor)
}

// HelpBoxStyle returns a style for the help overlay.
func HelpBoxStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width-4).
		Padding(2, 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(HelpColor)
}

// VerticalDivider creates a vertical divider string of the given height.
func VerticalDivider(height int) string {
	divider := ""
	for i := 0; i < height; i++ {
		divider += "│"
		if i < height-1 {
			divider += "\n"
		}
	}
	return divider
}
