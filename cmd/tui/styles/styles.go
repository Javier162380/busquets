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
	SubtleColor     = lipgloss.Color("238") // Recessive gray for secondary lines under a row.
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

	// SubtleStyle is for a secondary line rendered beneath the row it belongs to,
	// e.g. a source path under a label. It sits one step below the row's own text
	// in both themes so it never out-shouts the row.
	SubtleStyle = lipgloss.NewStyle().
			Foreground(SubtleColor).
			Italic(true)

	TitleStyle = lipgloss.NewStyle().
			Foreground(AccentColor).
			Bold(true)

	MetaStyle = lipgloss.NewStyle().
			Foreground(ForegroundColor).
			Italic(true)

	// LineNumberStyle is the gutter style for line numbers in the viewer,
	// matching the muted look of the editor's textarea gutter.
	LineNumberStyle = lipgloss.NewStyle().
			Foreground(InactiveColor)
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
