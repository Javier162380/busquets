package tui

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/screens"
	"github.com/Javier162380/claude-plan-viewer/internal/storage"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository/sqlite"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func newTestRepository(ctx context.Context, dbPath string) (*sqlite.Repository, error) {
	db, err := storage.NewSQLiteClientWithMigrations(ctx, dbPath, nil)
	if err != nil {
		return nil, err
	}
	return sqlite.NewRepository(db.DB()), nil
}

func setupTestService(t *testing.T) (*claudeviewer.Service, func()) {
	t.Helper()
	tempDir, err := os.MkdirTemp(os.TempDir(), "claude-viewer-tui-test-*")
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
	return svc, func() { os.RemoveAll(tempDir) }
}

func newTestApp(t *testing.T) *App {
	t.Helper()
	svc, cleanup := setupTestService(t)
	t.Cleanup(cleanup)
	app := New(context.Background(), svc)
	_ = app.Init()
	model, _ := app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	return model.(*App)
}

func TestHandleSyncHandlers(t *testing.T) {
	t.Run("SyncPlansMsg sets loading state", func(t *testing.T) {
		app := newTestApp(t)
		model, cmd := app.Update(messages.SyncPlansMsg{})
		app = model.(*App)
		require.True(t, app.statusBar.IsLoading())
		require.NotNil(t, cmd)
	})

	t.Run("SyncResultMsg success sets success message with count", func(t *testing.T) {
		app := newTestApp(t)
		model, cmd := app.Update(messages.SyncResultMsg{Count: 3})
		app = model.(*App)
		require.Contains(t, app.statusBar.SuccessMessage(), "3")
		require.NotNil(t, cmd)
	})

	t.Run("SyncResultMsg error sets error message", func(t *testing.T) {
		app := newTestApp(t)
		model, _ := app.Update(messages.SyncResultMsg{Error: errors.New("sync failed")})
		app = model.(*App)
		require.Contains(t, app.statusBar.ErrorMessage(), "sync failed")
	})

	t.Run("RSyncResultMsg success sets success message", func(t *testing.T) {
		app := newTestApp(t)
		model, _ := app.Update(messages.RSyncResultMsg{Count: 2})
		app = model.(*App)
		require.Contains(t, app.statusBar.SuccessMessage(), "2")
	})

	t.Run("DumpResultMsg success sets success message with count", func(t *testing.T) {
		app := newTestApp(t)
		model, _ := app.Update(messages.DumpResultMsg{Count: 5})
		app = model.(*App)
		require.Contains(t, app.statusBar.SuccessMessage(), "5")
	})
}

func TestHandleSaveResult(t *testing.T) {
	t.Run("success sets success message", func(t *testing.T) {
		app := newTestApp(t)
		model, _ := app.Update(messages.SaveResultMsg{Result: &claudeviewer.UpdatePlanResult{Success: true}})
		app = model.(*App)
		require.NotEmpty(t, app.statusBar.SuccessMessage())
	})

	t.Run("error sets error message", func(t *testing.T) {
		app := newTestApp(t)
		model, _ := app.Update(messages.SaveResultMsg{Error: errors.New("write error")})
		app = model.(*App)
		require.NotEmpty(t, app.statusBar.ErrorMessage())
	})
}

func TestHandleTagHandlers(t *testing.T) {
	t.Run("CreateTagResultMsg success sets success message and returns reload cmd", func(t *testing.T) {
		app := newTestApp(t)
		model, cmd := app.Update(messages.CreateTagResultMsg{})
		app = model.(*App)
		require.Equal(t, "Tag created", app.statusBar.SuccessMessage())
		require.NotNil(t, cmd)
	})

	t.Run("CreateTagResultMsg error sets error message", func(t *testing.T) {
		app := newTestApp(t)
		model, _ := app.Update(messages.CreateTagResultMsg{Error: errors.New("duplicate tag")})
		app = model.(*App)
		require.NotEmpty(t, app.statusBar.ErrorMessage())
	})
}

func TestHandleSettingsHandlers(t *testing.T) {
	t.Run("ThemeChangedMsg updates isDarkModeEnabled", func(t *testing.T) {
		app := newTestApp(t)
		require.True(t, app.isDarkModeEnabled)
		model, _ := app.Update(messages.ThemeChangedMsg{DarkMode: false})
		app = model.(*App)
		require.False(t, app.isDarkModeEnabled)
	})

	t.Run("DisplayModeChangedMsg updates displayMode", func(t *testing.T) {
		app := newTestApp(t)
		require.Equal(t, claudeviewer.DisplayModePlanContent, app.displayMode)
		model, _ := app.Update(messages.DisplayModeChangedMsg{Mode: claudeviewer.DisplayModeTagPlanContent})
		app = model.(*App)
		require.Equal(t, claudeviewer.DisplayModeTagPlanContent, app.displayMode)
	})

	t.Run("PlansSortKeyChangedMsg returns non-nil reload cmd", func(t *testing.T) {
		app := newTestApp(t)
		_, cmd := app.Update(messages.PlansSortKeyChangedMsg{})
		require.NotNil(t, cmd)
	})
}

func TestNavigation(t *testing.T) {
	t.Run("PopScreenMsg with multi-screen stack reduces stack by one", func(t *testing.T) {
		app := newTestApp(t)
		model, _ := app.Update(messages.OpenSettingsMsg{})
		app = model.(*App)
		require.Equal(t, 2, len(app.stack))

		model, _ = app.Update(messages.PopScreenMsg{})
		app = model.(*App)
		require.Equal(t, 1, len(app.stack))
	})

	t.Run("PopScreenMsg with single screen stack does not shrink", func(t *testing.T) {
		app := newTestApp(t)
		require.Equal(t, 1, len(app.stack))
		model, _ := app.Update(messages.PopScreenMsg{})
		app = model.(*App)
		require.Equal(t, 1, len(app.stack))
	})

	t.Run("PlansLoadedMsg routes to PlansScreen even when settings screen is on top", func(t *testing.T) {
		app := newTestApp(t)
		model, _ := app.Update(messages.OpenSettingsMsg{})
		app = model.(*App)
		require.Equal(t, 2, len(app.stack))
		_, topIsPlansScreen := app.stack[len(app.stack)-1].(*screens.PlansScreen)
		require.False(t, topIsPlansScreen)

		model, _ = app.Update(messages.PlansLoadedMsg{Plans: []claudeviewer.PlanSummary{}})
		app = model.(*App)
		require.Equal(t, 2, len(app.stack))
	})
}

func TestInit(t *testing.T) {
	t.Run("returns non-nil batch command", func(t *testing.T) {
		svc, cleanup := setupTestService(t)
		defer cleanup()
		app := New(context.Background(), svc)
		cmd := app.Init()
		require.NotNil(t, cmd)
	})
}
