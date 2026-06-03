package components

import (
	"testing"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	"github.com/stretchr/testify/require"
)

func TestTagModal(t *testing.T) {
	t.Run("IsActive is false before Open", func(t *testing.T) {
		m := NewTagModal()
		require.False(t, m.IsActive())
	})

	t.Run("IsActive is true after Open", func(t *testing.T) {
		m := NewTagModal()
		m.Open("plan.md", "/path/to/plans", nil, nil)
		require.True(t, m.IsActive())
	})

	t.Run("Close sets IsActive to false", func(t *testing.T) {
		m := NewTagModal()
		m.Open("plan.md", "/path/to/plans", nil, nil)
		m.Close()
		require.False(t, m.IsActive())
	})

	t.Run("Close returns empty slice when no tags toggled", func(t *testing.T) {
		m := NewTagModal()
		m.Open("plan.md", "/path/to/plans", nil, nil)
		result := m.Close()
		require.Empty(t, result)
	})

	t.Run("ToggleTag selects an unselected tag", func(t *testing.T) {
		m := NewTagModal()
		m.Open("plan.md", "/path/to/plans", nil, nil)
		m.ToggleTag("go")
		result := m.Close()
		require.Contains(t, result, "go")
	})

	t.Run("ToggleTag deselects an already selected tag", func(t *testing.T) {
		m := NewTagModal()
		planTags := []dto.Tag{{Name: "go"}}
		m.Open("plan.md", "/path/to/plans", planTags, planTags)
		m.ToggleTag("go")
		result := m.Close()
		require.NotContains(t, result, "go")
	})

	t.Run("AddNewTag appends to selected tags", func(t *testing.T) {
		m := NewTagModal()
		m.Open("plan.md", "/path/to/plans", nil, nil)
		m.AddNewTag("new-feature")
		result := m.Close()
		require.Contains(t, result, "new-feature")
	})

	t.Run("AddNewTag ignores empty string", func(t *testing.T) {
		m := NewTagModal()
		m.Open("plan.md", "/path/to/plans", nil, nil)
		m.AddNewTag("")
		result := m.Close()
		require.Empty(t, result)
	})
}
