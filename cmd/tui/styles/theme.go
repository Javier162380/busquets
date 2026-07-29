package styles

import "github.com/charmbracelet/lipgloss"

// Theme holds the current color scheme.
type Theme struct {
	AccentColor     lipgloss.Color
	HeaderBgColor   lipgloss.Color
	BorderColor     lipgloss.Color
	MutedColor      lipgloss.Color
	InactiveColor   lipgloss.Color
	SubtleColor     lipgloss.Color
	ForegroundColor lipgloss.Color
	BackgroundColor lipgloss.Color
	ErrorColor      lipgloss.Color
	SuccessColor    lipgloss.Color
	LoadingColor    lipgloss.Color
	HelpColor       lipgloss.Color
}

// DarkTheme is the default dark color scheme.
var DarkTheme = Theme{
	AccentColor:     lipgloss.Color("205"),
	HeaderBgColor:   lipgloss.Color("235"),
	BorderColor:     lipgloss.Color("238"),
	MutedColor:      lipgloss.Color("244"),
	InactiveColor:   lipgloss.Color("240"),
	SubtleColor:     lipgloss.Color("238"), // dimmer than InactiveColor: recedes toward the dark background
	ForegroundColor: lipgloss.Color("250"),
	BackgroundColor: lipgloss.Color("234"),
	ErrorColor:      lipgloss.Color("160"),
	SuccessColor:    lipgloss.Color("42"),
	LoadingColor:    lipgloss.Color("81"),
	HelpColor:       lipgloss.Color("205"),
}

// LightTheme is the light color scheme.
var LightTheme = Theme{
	AccentColor:     lipgloss.Color("162"),
	HeaderBgColor:   lipgloss.Color("254"),
	BorderColor:     lipgloss.Color("240"),
	MutedColor:      lipgloss.Color("238"),
	InactiveColor:   lipgloss.Color("242"),
	SubtleColor:     lipgloss.Color("247"), // lighter than InactiveColor: recedes toward the light background
	ForegroundColor: lipgloss.Color("232"),
	BackgroundColor: lipgloss.Color("255"),
	ErrorColor:      lipgloss.Color("160"),
	SuccessColor:    lipgloss.Color("28"),
	LoadingColor:    lipgloss.Color("33"),
	HelpColor:       lipgloss.Color("162"),
}

// CurrentTheme is the active theme.
var CurrentTheme = DarkTheme

// SetDarkMode switches between dark and light themes.
func SetDarkMode(enabled bool) {
	if enabled {
		CurrentTheme = DarkTheme
	} else {
		CurrentTheme = LightTheme
	}
	updateStyleVariables()
}

func updateStyleVariables() {
	AccentColor = CurrentTheme.AccentColor
	HeaderBgColor = CurrentTheme.HeaderBgColor
	BorderColor = CurrentTheme.BorderColor
	MutedColor = CurrentTheme.MutedColor
	InactiveColor = CurrentTheme.InactiveColor
	SubtleColor = CurrentTheme.SubtleColor
	ForegroundColor = CurrentTheme.ForegroundColor
	ErrorColor = CurrentTheme.ErrorColor
	SuccessColor = CurrentTheme.SuccessColor
	LoadingColor = CurrentTheme.LoadingColor
	HelpColor = CurrentTheme.HelpColor

	// Regenerate text styles.
	AccentStyle = lipgloss.NewStyle().Foreground(AccentColor)
	ActiveStyle = lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
	InactiveStyle = lipgloss.NewStyle().Foreground(InactiveColor)
	ErrorStyle = lipgloss.NewStyle().Foreground(ErrorColor).Bold(true)
	SuccessStyle = lipgloss.NewStyle().Foreground(SuccessColor).Bold(true)
	LoadingStyle = lipgloss.NewStyle().Foreground(LoadingColor).Bold(true)
	MutedStyle = lipgloss.NewStyle().Foreground(MutedColor)
	SubtleStyle = lipgloss.NewStyle().Foreground(SubtleColor).Italic(true)
	TitleStyle = lipgloss.NewStyle().Foreground(AccentColor).Bold(true)
	MetaStyle = lipgloss.NewStyle().Foreground(ForegroundColor).Italic(true)
}
