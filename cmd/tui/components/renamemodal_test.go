package components

import (
	"testing"

	"github.com/Javier162380/busquets/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestRenameModal(t *testing.T) {
	t.Run("IsActive is false before Open", func(t *testing.T) {
		m := NewRenameModal()
		require.False(t, m.IsActive())
	})

	t.Run("Open pre-fills the current file name", func(t *testing.T) {
		m := NewRenameModal()
		m.SetSize(60, 12)
		m.Open("old.md", "/src")
		require.True(t, m.IsActive())
		require.Equal(t, "old.md", m.input.Value())
	})

	t.Run("esc cancels without emitting", func(t *testing.T) {
		m := NewRenameModal()
		m.SetSize(60, 12)
		m.Open("old.md", "/src")
		cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		require.Nil(t, cmd)
		require.False(t, m.IsActive())
	})

	t.Run("enter with unchanged name does not open the confirm", func(t *testing.T) {
		m := NewRenameModal()
		m.SetSize(60, 12)
		m.Open("old.md", "/src")
		cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.Nil(t, cmd)
		require.False(t, m.confirm.IsActive())
		require.True(t, m.IsActive())
	})

	t.Run("enter then confirm emits RenamePlanFileMsg with the new name", func(t *testing.T) {
		m := NewRenameModal()
		m.SetSize(60, 12)
		m.Open("old.md", "/src")

		// Type a suffix so the name differs from the original.
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-new")})
		require.Equal(t, "old.md-new", m.input.Value())

		// enter opens the confirmation dialog.
		require.Nil(t, m.Update(tea.KeyMsg{Type: tea.KeyEnter}))
		require.True(t, m.confirm.IsActive())

		// left selects "Yes", enter confirms and returns the emit command.
		m.Update(tea.KeyMsg{Type: tea.KeyLeft})
		cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.NotNil(t, cmd)
		require.False(t, m.IsActive())

		msg, ok := cmd().(messages.RenamePlanFileMsg)
		require.True(t, ok)
		require.Equal(t, "old.md", msg.FileName)
		require.Equal(t, "/src", msg.SyncSource)
		require.Equal(t, "old.md-new", msg.NewFileName)
	})

	t.Run("confirm cancelled (No) emits nothing and closes", func(t *testing.T) {
		m := NewRenameModal()
		m.SetSize(60, 12)
		m.Open("old.md", "/src")
		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("-new")})
		m.Update(tea.KeyMsg{Type: tea.KeyEnter}) // open confirm (defaults to No)
		cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.Nil(t, cmd)
		require.False(t, m.IsActive())
	})
}
