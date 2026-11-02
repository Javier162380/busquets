package components

import (
	"strings"
	"testing"

	"github.com/Javier162380/busquets/services/busquets"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

// submitted records every onSubmit call so tests can assert both the set and how often it fired.
type submitted struct {
	calls [][]busquets.VersionMilestone
}

func (r *submitted) onSubmit(milestones []busquets.VersionMilestone) tea.Cmd {
	r.calls = append(r.calls, milestones)
	return nil
}

func TestMilestoneModal(t *testing.T) {
	press := func(m *MilestoneModal, keys ...tea.KeyMsg) tea.Cmd {
		var cmd tea.Cmd
		for _, key := range keys {
			cmd = m.Update(key)
		}
		return cmd
	}
	down := tea.KeyMsg{Type: tea.KeyDown}
	up := tea.KeyMsg{Type: tea.KeyUp}
	space := tea.KeyMsg{Type: tea.KeySpace}
	enter := tea.KeyMsg{Type: tea.KeyEnter}
	esc := tea.KeyMsg{Type: tea.KeyEsc}

	t.Run("enter hands the checked set to onSubmit in lifecycle order", func(t *testing.T) {
		var got submitted
		m := NewMilestoneModal()
		m.Open("Version 3 milestones", []busquets.VersionMilestone{busquets.MilestonePlanImplementationStart}, got.onSubmit)

		press(m, down, space, enter) // check HumanReview
		require.False(t, m.IsActive())
		require.Equal(t, [][]busquets.VersionMilestone{
			{busquets.MilestoneHumanReview, busquets.MilestonePlanImplementationStart},
		}, got.calls)
	})

	t.Run("enter returns the callback's command", func(t *testing.T) {
		m := NewMilestoneModal()
		type savedMsg struct{}
		m.Open("t", nil, func([]busquets.VersionMilestone) tea.Cmd {
			return func() tea.Msg { return savedMsg{} }
		})

		require.Equal(t, savedMsg{}, press(m, enter)())
	})

	t.Run("toggling a checked milestone unchecks it", func(t *testing.T) {
		var got submitted
		m := NewMilestoneModal()
		m.Open("t", []busquets.VersionMilestone{busquets.MilestoneLLMGenerated}, got.onSubmit)

		press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, enter)
		require.Equal(t, [][]busquets.VersionMilestone{nil}, got.calls)
	})

	t.Run("esc closes without calling onSubmit", func(t *testing.T) {
		var got submitted
		m := NewMilestoneModal()
		m.Open("t", nil, got.onSubmit)

		require.Nil(t, press(m, space, esc))
		require.False(t, m.IsActive())
		require.Nil(t, got.calls)
	})

	t.Run("the title is the caller's", func(t *testing.T) {
		m := NewMilestoneModal()
		m.SetSize(70, 20)
		m.Open("Add milestones to the new version? (optional)", nil, (&submitted{}).onSubmit)

		titleRow := strings.Split(m.View(), "\n")[2] // below the top border and padding row
		require.Equal(t, "│  Add milestones to the new version? (optional)                     │", titleRow)
	})

	t.Run("the cursor clamps at both ends", func(t *testing.T) {
		m := NewMilestoneModal()
		m.Open("t", nil, (&submitted{}).onSubmit)

		press(m, up)
		require.Equal(t, 0, m.cursor)
		press(m, down, down, down, down, down, down, down)
		require.Equal(t, len(busquets.VersionMilestones)-1, m.cursor)
	})

	t.Run("j and k do not move the cursor", func(t *testing.T) {
		m := NewMilestoneModal()
		m.Open("t", nil, (&submitted{}).onSubmit)

		press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		require.Equal(t, 0, m.cursor)
	})

	t.Run("reopening resets the previous selection and callback", func(t *testing.T) {
		var first, second submitted
		m := NewMilestoneModal()
		m.Open("t", []busquets.VersionMilestone{busquets.MilestoneLLMGenerated}, first.onSubmit)
		press(m, esc)
		m.Open("t", nil, second.onSubmit)

		press(m, enter)
		require.Nil(t, first.calls)
		require.Equal(t, [][]busquets.VersionMilestone{nil}, second.calls)
	})

	t.Run("an inactive modal ignores keys", func(t *testing.T) {
		require.Nil(t, NewMilestoneModal().Update(enter))
	})
}
