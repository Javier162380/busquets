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

	MutedStyle = lipgloss.NewStyle().
			Foreground(MutedColor)

	TitleStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	MetaStyle = lipgloss.NewStyle().
			Foreground(ForegroundColor).
			Italic(true)
)

// StatusStyle returns a style for the status bar.
func StatusStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width).
		Padding(0, 2).
		Background(HeaderBgColor).
		Foreground(MutedColor)
}

// HelpBoxStyle returns a style for the help overlay.
func HelpBoxStyle(width int) lipgloss.Style {
	return lipgloss.NewStyle().
		Width(width-4).
		Padding(2, 4).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(HelpColor)
}

// Border styles for panels.
var (
	FocusedBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(AccentColor)

	BlurredBorderStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(BorderColor)

	FocusedTitleStyle = lipgloss.NewStyle().
				Foreground(AccentColor).
				Bold(true)

	DimStyle = lipgloss.NewStyle().
			Foreground(MutedColor)
)

// VerticalDivider returns a vertical divider of specified height.
func VerticalDivider(height int) string {
	divider := ""
	for i := 0; i < height; i++ {
		divider += "│\n"
	}
	return lipgloss.NewStyle().
		Foreground(BorderColor).
		Render(divider)
}
