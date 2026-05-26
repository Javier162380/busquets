package commands

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	"github.com/Javier162380/claude-plan-viewer/internal/storage"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository/sqlite"
	"github.com/stretchr/testify/require"
)

func newTestRepository(ctx context.Context, dbPath string) (*sqlite.Repository, error) {
	db, err := storage.NewSQLiteClientWithMigrations(ctx, dbPath, nil)
	if err != nil {
		return nil, err
	}
	return sqlite.NewRepository(db.DB()), nil
}

func setupTestService(t *testing.T) (*claudeviewer.Service, string, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp(os.TempDir(), "claude-viewer-commands-test-*")
	require.NoError(t, err)
	viewerDir := filepath.Join(tempDir, "viewer")
	sourcePlansDir := filepath.Join(tempDir, "source")
	dbPath := filepath.Join(tempDir, "test.db")
	require.NoError(t, os.MkdirAll(viewerDir, 0o755))
	require.NoError(t, os.MkdirAll(sourcePlansDir, 0o755))
	ctx := context.Background()
	db, err := newTestRepository(ctx, dbPath)
	require.NoError(t, err)
	svc, err := claudeviewer.New(db, viewerDir, sourcePlansDir, true)
	require.NoError(t, err)
	return svc, sourcePlansDir, func() { os.RemoveAll(tempDir) }
}

func createTestPlanFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(content), 0o600))
}

func TestLoadPlansCmd(t *testing.T) {
	ctx := context.Background()

	t.Run("empty database returns empty plans loaded message", func(t *testing.T) {
		svc, _, cleanup := setupTestService(t)
		defer cleanup()
		msg := LoadPlansCmd(ctx, svc)()
		loaded, ok := msg.(messages.PlansLoadedMsg)
		require.True(t, ok)
		require.Empty(t, loaded.Plans)
	})

	t.Run("synced plans are returned in message", func(t *testing.T) {
		svc, sourcePlansDir, cleanup := setupTestService(t)
		defer cleanup()
		createTestPlanFile(t, sourcePlansDir, "test.md", "# Test Plan\nContent")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		msg := LoadPlansCmd(ctx, svc)()
		loaded, ok := msg.(messages.PlansLoadedMsg)
		require.True(t, ok)
		require.Len(t, loaded.Plans, 1)
	})

	t.Run("cancelled context returns error message", func(t *testing.T) {
		svc, _, cleanup := setupTestService(t)
		defer cleanup()
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()
		msg := LoadPlansCmd(cancelCtx, svc)()
		_, ok := msg.(messages.ErrorMsg)
		require.True(t, ok)
	})
}

func TestLoadPlanDetailCmd(t *testing.T) {
	ctx := context.Background()

	t.Run("existing plan returns detail message", func(t *testing.T) {
		svc, sourcePlansDir, cleanup := setupTestService(t)
		defer cleanup()
		createTestPlanFile(t, sourcePlansDir, "detail.md", "# Detail Plan\nContent here")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		msg := LoadPlanDetailCmd(ctx, svc, "detail.md")()
		detail, ok := msg.(messages.PlanDetailLoadedMsg)
		require.True(t, ok)
		require.NotNil(t, detail.Detail)
		require.Equal(t, "detail.md", detail.Detail.FileName)
	})

	t.Run("unknown filename returns error message", func(t *testing.T) {
		svc, _, cleanup := setupTestService(t)
		defer cleanup()
		msg := LoadPlanDetailCmd(ctx, svc, "nonexistent.md")()
		_, ok := msg.(messages.ErrorMsg)
		require.True(t, ok)
	})
}

func TestSyncPlansCmd(t *testing.T) {
	ctx := context.Background()

	t.Run("new md file returns sync result with count 1", func(t *testing.T) {
		svc, sourcePlansDir, cleanup := setupTestService(t)
		defer cleanup()
		createTestPlanFile(t, sourcePlansDir, "new.md", "# New Plan\nContent")
		msg := SyncPlansCmd(ctx, svc)()
		result, ok := msg.(messages.SyncResultMsg)
		require.True(t, ok)
		require.NoError(t, result.Error)
		require.Equal(t, 1, result.Count)
	})

	t.Run("already synced file returns count 0", func(t *testing.T) {
		svc, sourcePlansDir, cleanup := setupTestService(t)
		defer cleanup()
		createTestPlanFile(t, sourcePlansDir, "existing.md", "# Existing\nContent")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		msg := SyncPlansCmd(ctx, svc)()
		result, ok := msg.(messages.SyncResultMsg)
		require.True(t, ok)
		require.Equal(t, 0, result.Count)
	})
}

func TestSearchPlansCmd(t *testing.T) {
	ctx := context.Background()

	t.Run("matching query returns plans loaded message with results", func(t *testing.T) {
		svc, sourcePlansDir, cleanup := setupTestService(t)
		defer cleanup()
		createTestPlanFile(t, sourcePlansDir, "searchable.md", "# Searchable Plan\nContent")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		msg := SearchPlansCmd(ctx, svc, "Searchable")()
		loaded, ok := msg.(messages.PlansLoadedMsg)
		require.True(t, ok)
		require.NotEmpty(t, loaded.Plans)
	})

	t.Run("non-matching query returns empty plans", func(t *testing.T) {
		svc, sourcePlansDir, cleanup := setupTestService(t)
		defer cleanup()
		createTestPlanFile(t, sourcePlansDir, "another.md", "# Another Plan\nContent")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		msg := SearchPlansCmd(ctx, svc, "zzznomatch")()
		loaded, ok := msg.(messages.PlansLoadedMsg)
		require.True(t, ok)
		require.Empty(t, loaded.Plans)
	})
}

func TestCreateTagCmd(t *testing.T) {
	ctx := context.Background()

	t.Run("valid name returns result with no error", func(t *testing.T) {
		svc, _, cleanup := setupTestService(t)
		defer cleanup()
		msg := CreateTagCmd(ctx, svc, "my-tag")()
		result, ok := msg.(messages.CreateTagResultMsg)
		require.True(t, ok)
		require.NoError(t, result.Error)
	})
}

func TestSetSettingCmd(t *testing.T) {
	ctx := context.Background()

	t.Run("boolean setting saves and returns success result", func(t *testing.T) {
		svc, _, cleanup := setupTestService(t)
		defer cleanup()
		boolTrue := true
		msg := SetSettingCmd(ctx, svc, claudeviewer.SettingDarkModeEnabled, claudeviewer.SettingValues{BooleanValue: &boolTrue})()
		result, ok := msg.(messages.SettingUpdateResultMsg)
		require.True(t, ok)
		require.True(t, result.Success)
		require.NoError(t, result.Error)
	})
}

func TestLoadAllTagsForPanelCmd(t *testing.T) {
	ctx := context.Background()

	t.Run("returns all tags for panel message", func(t *testing.T) {
		svc, _, cleanup := setupTestService(t)
		defer cleanup()
		msg := LoadAllTagsForPanelCmd(ctx, svc)()
		_, ok := msg.(messages.AllTagsForPanelLoadedMsg)
		require.True(t, ok)
	})
}

func TestClearStatusCmd(t *testing.T) {
	t.Run("returns clear status message after delay", func(t *testing.T) {
		msg := ClearStatusCmd(1 * time.Millisecond)()
		_, ok := msg.(messages.ClearStatusMsg)
		require.True(t, ok)
	})
}
