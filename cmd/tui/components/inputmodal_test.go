package components

import (
	"errors"
	"strconv"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestInputModal(t *testing.T) {
	digitsOnly := func(r rune) bool { return r >= '0' && r <= '9' }

	t.Run("IsActive is false before Open", func(t *testing.T) {
		m := NewInputModal[int]()
		require.False(t, m.IsActive())
	})

	t.Run("Open activates the modal with an empty input", func(t *testing.T) {
		m := NewInputModal[int]()
		m.SetSize(40, 8)
		m.Open("Go to line", "line number", digitsOnly, strconv.Atoi, func(int) {})
		require.True(t, m.IsActive())
		require.Equal(t, "", m.input.Value())
	})

	t.Run("esc cancels without calling onSubmit", func(t *testing.T) {
		m := NewInputModal[int]()
		m.SetSize(40, 8)
		called := false
		m.Open("Go to line", "line number", digitsOnly, strconv.Atoi, func(int) { called = true })

		cmd := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		require.Nil(t, cmd)
		require.False(t, m.IsActive())
		require.False(t, called)
	})

	t.Run("filter rejects non-matching runes", func(t *testing.T) {
		m := NewInputModal[int]()
		m.SetSize(40, 8)
		m.Open("Go to line", "line number", digitsOnly, strconv.Atoi, func(int) {})

		for _, r := range "a1b2" {
			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		require.Equal(t, "12", m.input.Value())
	})

	t.Run("enter on a parseable value calls onSubmit and closes", func(t *testing.T) {
		m := NewInputModal[int]()
		m.SetSize(40, 8)
		var got int
		called := false
		m.Open("Go to line", "line number", digitsOnly, strconv.Atoi, func(n int) {
			got = n
			called = true
		})

		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("42")})
		cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

		require.Nil(t, cmd)
		require.False(t, m.IsActive())
		require.True(t, called)
		require.Equal(t, 42, got)
	})

	t.Run("enter on an empty value closes without calling onSubmit", func(t *testing.T) {
		m := NewInputModal[int]()
		m.SetSize(40, 8)
		called := false
		m.Open("Go to line", "line number", digitsOnly, strconv.Atoi, func(int) { called = true })

		cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		require.Nil(t, cmd)
		require.False(t, m.IsActive())
		require.False(t, called)
	})

	t.Run("enter on a value that fails parse closes without calling onSubmit", func(t *testing.T) {
		m := NewInputModal[string]()
		m.SetSize(40, 8)
		called := false
		failingParse := func(string) (string, error) { return "", errors.New("nope") }
		m.Open("Title", "value", nil, failingParse, func(string) { called = true })

		m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
		cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})

		require.Nil(t, cmd)
		require.False(t, m.IsActive())
		require.False(t, called)
	})
}
