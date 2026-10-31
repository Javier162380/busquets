package components

import (
	"testing"

	"github.com/Javier162380/busquets/cmd/tui/messages"
	"github.com/Javier162380/busquets/services/busquets"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

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

	t.Run("enter emits the checked set in lifecycle order", func(t *testing.T) {
		m := NewMilestoneModal()
		m.Open(7, 3, []busquets.VersionMilestone{busquets.MilestonePlanImplementationStart})

		cmd := press(m, down, space, enter) // check HumanReview
		require.False(t, m.IsActive())
		require.Equal(t, messages.SetVersionMilestonesMsg{
			PlanID:        7,
			VersionNumber: 3,
			Milestones:    []busquets.VersionMilestone{busquets.MilestoneHumanReview, busquets.MilestonePlanImplementationStart},
		}, cmd())
	})

	t.Run("toggling a checked milestone unchecks it", func(t *testing.T) {
		m := NewMilestoneModal()
		m.Open(7, 3, []busquets.VersionMilestone{busquets.MilestoneLLMGenerated})

		cmd := press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}}, enter)
		require.Equal(t, messages.SetVersionMilestonesMsg{PlanID: 7, VersionNumber: 3}, cmd())
	})

	t.Run("esc closes without emitting", func(t *testing.T) {
		m := NewMilestoneModal()
		m.Open(7, 3, nil)

		require.Nil(t, press(m, space, esc))
		require.False(t, m.IsActive())
	})

	t.Run("the cursor clamps at both ends", func(t *testing.T) {
		m := NewMilestoneModal()
		m.Open(7, 3, nil)

		press(m, up)
		require.Equal(t, 0, m.cursor)
		press(m, down, down, down, down, down, down, down)
		require.Equal(t, len(busquets.VersionMilestones)-1, m.cursor)
	})

	t.Run("j and k do not move the cursor", func(t *testing.T) {
		m := NewMilestoneModal()
		m.Open(7, 3, nil)

		press(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
		require.Equal(t, 0, m.cursor)
	})

	t.Run("reopening resets the previous selection", func(t *testing.T) {
		m := NewMilestoneModal()
		m.Open(7, 3, []busquets.VersionMilestone{busquets.MilestoneLLMGenerated})
		press(m, esc)
		m.Open(7, 4, nil)

		require.Equal(t, messages.SetVersionMilestonesMsg{PlanID: 7, VersionNumber: 4}, press(m, enter)())
	})

	t.Run("an inactive modal ignores keys", func(t *testing.T) {
		require.Nil(t, NewMilestoneModal().Update(enter))
	})
}
