package busquets

import (
	"testing"

	"github.com/Javier162380/busquets/internal/config"

	"github.com/stretchr/testify/require"
)

func TestRemapSyncSource(t *testing.T) {
	dirs := []config.SyncDir{
		{Path: "/Users/llorenj1/.claude/plans", Label: "claudeRoot"},
		{Path: "/Users/llorenj1/fuga/trends-data-platform/docs/superpowers/plans", Label: "trendsDataPlatformPlans"},
		{Path: "/Users/llorenj1/fuga/trends-data-platform/docs/superpowers/specs", Label: "trendsDataPlatformSpecs"},
	}

	t.Run("keeps an exact configured path", func(t *testing.T) {
		require.Equal(t, dirs[0].Path, remapSyncSource(dirs[0].Path, dirs))
	})

	t.Run("maps a same-layout path from another home directory", func(t *testing.T) {
		require.Equal(t, dirs[0].Path, remapSyncSource("/Users/javier/.claude/plans", dirs))
		require.Equal(t, dirs[1].Path, remapSyncSource("/Users/javier/fuga/trends-data-platform/docs/superpowers/plans", dirs))
		require.Equal(t, dirs[2].Path, remapSyncSource("/Users/javier/fuga/trends-data-platform/docs/superpowers/specs", dirs))
	})

	t.Run("does not map a path that only shares the last component", func(t *testing.T) {
		require.Equal(t, "/tmp/other/plans", remapSyncSource("/tmp/other/plans", dirs))
	})

	t.Run("returns the stored path when no dirs are configured", func(t *testing.T) {
		require.Equal(t, "/Users/javier/.claude/plans", remapSyncSource("/Users/javier/.claude/plans", nil))
	})
}
