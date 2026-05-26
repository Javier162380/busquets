package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestConfirmModal(t *testing.T) {
	t.Run("IsActive is false before Open", func(t *testing.T) {
		m := NewConfirmModal()
		require.False(t, m.IsActive())
	})

	t.Run("IsActive is true after Open", func(t *testing.T) {
		m := NewConfirmModal()
		m.Open("Are you sure?", func() tea.Msg { return nil })
		require.True(t, m.IsActive())
	})

	t.Run("IsActive is false after Close", func(t *testing.T) {
		m := NewConfirmModal()
		m.Open("Are you sure?", func() tea.Msg { return nil })
		m.Close()
		require.False(t, m.IsActive())
	})
}
