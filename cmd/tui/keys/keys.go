// Package keys provides centralized key bindings for the TUI.
package keys

import "github.com/charmbracelet/bubbles/key"

// Key constants for consistent key handling.
const (
	KeyUp     = "k"
	KeyDown   = "j"
	KeyTop    = "g"
	KeyBottom = "G"
	KeyView   = "v"
	KeyEdit   = "e"
	KeySave   = "ctrl+s"
	KeySync   = "s"
	KeyRender = "r"
	KeyHelp   = "?"
	KeyBack   = "esc"
	KeyQuit   = "q"
)

// KeyMap defines all key bindings for the application.
type KeyMap struct {
	// Navigation.
	Up     key.Binding
	Down   key.Binding
	Top    key.Binding
	Bottom key.Binding

	// Actions.
	View     key.Binding
	Edit     key.Binding
	Save     key.Binding
	Sync     key.Binding
	Render   key.Binding
	Versions key.Binding

	// General.
	Help key.Binding
	Back key.Binding
	Quit key.Binding
}

// DefaultKeyMap returns the default key bindings.
func DefaultKeyMap() KeyMap {
	return KeyMap{
		Up: key.NewBinding(
			key.WithKeys(KeyUp, "up"),
			key.WithHelp("j/k", "navigate"),
		),
		Down: key.NewBinding(
			key.WithKeys(KeyDown, "down"),
			key.WithHelp("j/k", "navigate"),
		),
		Top: key.NewBinding(
			key.WithKeys(KeyTop),
			key.WithHelp("g", "top"),
		),
		Bottom: key.NewBinding(
			key.WithKeys(KeyBottom),
			key.WithHelp("G", "bottom"),
		),
		View: key.NewBinding(
			key.WithKeys(KeyView),
			key.WithHelp("v", "view"),
		),
		Edit: key.NewBinding(
			key.WithKeys(KeyEdit),
			key.WithHelp("e", "edit"),
		),
		Save: key.NewBinding(
			key.WithKeys(KeySave),
			key.WithHelp("ctrl+s", "save"),
		),
		Sync: key.NewBinding(
			key.WithKeys(KeySync),
			key.WithHelp("s", "sync"),
		),
		Render: key.NewBinding(
			key.WithKeys(KeyRender),
			key.WithHelp("r", "toggle render"),
		),
		Versions: key.NewBinding(
			key.WithKeys(KeyView),
			key.WithHelp("v", "versions"),
		),
		Help: key.NewBinding(
			key.WithKeys(KeyHelp),
			key.WithHelp("?", "help"),
		),
		Back: key.NewBinding(
			key.WithKeys(KeyBack),
			key.WithHelp("esc", "back"),
		),
		Quit: key.NewBinding(
			key.WithKeys(KeyQuit, "ctrl+c"),
			key.WithHelp("q", "quit"),
		),
	}
}

// Binding represents a single key binding for display.
type Binding struct {
	Key  string
	Help string
}

// ListBindings returns key bindings for list navigation.
func ListBindings() []Binding {
	return []Binding{
		{Key: "j/k", Help: "navigate"},
		{Key: "v", Help: "view"},
		{Key: "e", Help: "edit"},
		{Key: "s", Help: "sync"},
		{Key: "?", Help: "help"},
		{Key: "q", Help: "quit"},
	}
}

// ViewerBindings returns key bindings for the viewer.
func ViewerBindings() []Binding {
	return []Binding{
		{Key: "j/k", Help: "scroll"},
		{Key: "g/G", Help: "top/bottom"},
		{Key: "r", Help: "toggle render"},
		{Key: "e", Help: "edit"},
		{Key: "v", Help: "versions"},
		{Key: "esc", Help: "back"},
	}
}

// VersionListBindings returns key bindings for version list.
func VersionListBindings() []Binding {
	return []Binding{
		{Key: "j/k", Help: "navigate"},
		{Key: "v", Help: "view"},
		{Key: "esc", Help: "back"},
	}
}

// VersionViewerBindings returns key bindings for version viewer.
func VersionViewerBindings() []Binding {
	return []Binding{
		{Key: "j/k", Help: "scroll"},
		{Key: "g/G", Help: "top/bottom"},
		{Key: "esc", Help: "back"},
	}
}

// EditorBindings returns key bindings for the editor.
func EditorBindings() []Binding {
	return []Binding{
		{Key: "ctrl+s", Help: "save"},
		{Key: "esc", Help: "cancel"},
	}
}

// FormatBindings formats key bindings as a string for the status bar.
func FormatBindings(bindings []Binding) string {
	result := ""
	for i, b := range bindings {
		if i > 0 {
			result += " | "
		}
		result += b.Key + ": " + b.Help
	}
	return result
}
