package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestHelpToggle(t *testing.T) {
	t.Run("? from list mode opens help", func(t *testing.T) {
		app := newTestApp(t)
		require.False(t, app.showHelp)
		model, _ := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("?")})
		app = model.(*App)
		require.True(t, app.showHelp, "help should open when ? pressed in list mode")
	})

	t.Run("EditorMode false in list mode", func(t *testing.T) {
		app := newTestApp(t)
		require.False(t, app.stack[0].EditorMode())
	})
}
