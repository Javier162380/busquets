
package screens

import (
	"testing"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/types"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	"github.com/stretchr/testify/require"
)

func TestVersionsGetViewerWidth(t *testing.T) {
	t.Run("fullscreen returns width minus padding", func(t *testing.T) {
		s := NewVersionsScreenWithData("plan.md", nil, 100, 40, false, false)
		s.layout = types.LayoutFullscreen
		require.Equal(t, 96, s.getViewerWidth())
	})

	t.Run("split layout returns half width minus padding", func(t *testing.T) {
		s := NewVersionsScreenWithData("plan.md", nil, 100, 40, false, false)
		require.Equal(t, 44, s.getViewerWidth())
	})
}

func TestVersionsUpdateListItems(t *testing.T) {
	t.Run("empty versions produces empty list", func(t *testing.T) {
		s := NewVersionsScreenWithData("plan.md", nil, 80, 24, false, false)
		require.Equal(t, 0, s.list.ItemCount())
	})

	t.Run("item count matches versions count", func(t *testing.T) {
		versions := []claudeviewer.PlanVersionDetail{
			{PlanVersion: claudeviewer.PlanVersion{VersionNumber: 1, CreatedAt: time.Now()}},
			{PlanVersion: claudeviewer.PlanVersion{VersionNumber: 2, CreatedAt: time.Now()}},
		}
		s := NewVersionsScreenWithData("plan.md", versions, 80, 24, false, false)
		require.Equal(t, 2, s.list.ItemCount())
	})

	t.Run("items display version number in title", func(t *testing.T) {
		versions := []claudeviewer.PlanVersionDetail{
			{PlanVersion: claudeviewer.PlanVersion{VersionNumber: 3, CreatedAt: time.Now()}},
		}
		s := NewVersionsScreenWithData("plan.md", versions, 80, 24, false, false)
		item := s.list.SelectedItem()
		require.NotNil(t, item)
		require.Contains(t, item.Title(), "3")
	})
}

func TestVersionsIsInputMode(t *testing.T) {
	t.Run("false when search bar is inactive", func(t *testing.T) {
		s := NewVersionsScreenWithData("plan.md", nil, 80, 24, false, false)
		require.False(t, s.IsInputMode())
	})
}
