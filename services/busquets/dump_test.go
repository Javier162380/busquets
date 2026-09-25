package busquets

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Javier162380/busquets/internal/config"

	"github.com/stretchr/testify/require"
)

func TestDumpPlans(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testDumpPlans(t, b)
		})
	}
}

func testDumpPlans(t *testing.T, b backendSetup) {
	t.Helper()
	ctx := context.Background()

	t.Run("empty database dumps nothing", func(t *testing.T) {
		svc, _, _, cleanup := b.setupFn(t)
		defer cleanup()

		count, err := svc.DumpPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("restores a plan deleted from its source dir", func(t *testing.T) {
		svc, sourceDir, _, cleanup := b.setupFn(t)
		defer cleanup()

		createTestPlanFile(t, sourceDir, "gone.md", sampleMarkdown)
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		require.NoError(t, os.Remove(filepath.Join(sourceDir, "gone.md")))

		count, err := svc.DumpPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		content, err := os.ReadFile(filepath.Join(sourceDir, "gone.md"))
		require.NoError(t, err)
		require.Equal(t, sampleMarkdown, string(content))
	})

	t.Run("recreates the source dir when it has been lost entirely", func(t *testing.T) {
		svc, sourceDir, _, cleanup := b.setupFn(t)
		defer cleanup()

		createTestPlanFile(t, sourceDir, "lost.md", sampleMarkdown)
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		require.NoError(t, os.RemoveAll(sourceDir))

		count, err := svc.DumpPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		content, err := os.ReadFile(filepath.Join(sourceDir, "lost.md"))
		require.NoError(t, err)
		require.Equal(t, sampleMarkdown, string(content))
	})

	t.Run("does not count or overwrite plans already on disk", func(t *testing.T) {
		svc, sourceDir, _, cleanup := b.setupFn(t)
		defer cleanup()

		createTestPlanFile(t, sourceDir, "present.md", sampleMarkdown)
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		edited := "# Present\n\nEdited on disk after sync.\n"
		createTestPlanFile(t, sourceDir, "present.md", edited)

		count, err := svc.DumpPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)

		content, err := os.ReadFile(filepath.Join(sourceDir, "present.md"))
		require.NoError(t, err)
		require.Equal(t, edited, string(content))
	})

	t.Run("writes each plan back to its own source dir", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "from-a.md", "# From A\n\nContent.")
		createTestPlanFile(t, dirs.sourceDir2, "from-b.md", "# From B\n\nContent.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		require.NoError(t, os.Remove(filepath.Join(dirs.sourceDir1, "from-a.md")))
		require.NoError(t, os.Remove(filepath.Join(dirs.sourceDir2, "from-b.md")))

		count, err := svc.DumpPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		require.FileExists(t, filepath.Join(dirs.sourceDir1, "from-a.md"))
		require.FileExists(t, filepath.Join(dirs.sourceDir2, "from-b.md"))
		require.NoFileExists(t, filepath.Join(dirs.sourceDir1, "from-b.md"))
		require.NoFileExists(t, filepath.Join(dirs.sourceDir2, "from-a.md"))
	})

	t.Run("rewrites a sync source recorded on another machine", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "moved.md", sampleMarkdown)
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		require.NoError(t, os.Remove(filepath.Join(dirs.sourceDir1, "moved.md")))

		// Simulate a DB copied from a machine whose home directory differs:
		// the stored sync_source shares only the trailing path components.
		stale := filepath.Join("/somewhere/else", filepath.Base(filepath.Dir(dirs.sourceDir1)), filepath.Base(dirs.sourceDir1))
		require.Equal(t, dirs.sourceDir1, svc.configuredSourcePath(stale))

		count, err := svc.DumpPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)
		require.FileExists(t, filepath.Join(dirs.sourceDir1, "moved.md"))
	})

	t.Run("skips a sync source that matches no configured plans dir", func(t *testing.T) {
		tempDir := t.TempDir()
		sourceDir := filepath.Join(tempDir, "source")
		require.NoError(t, os.MkdirAll(sourceDir, 0o755))

		repo, repoCleanup := b.repoFn(t, tempDir)
		if repoCleanup != nil {
			defer repoCleanup()
		}
		svc, _ := newServiceFromRepo(t, tempDir, repo, []config.SyncDir{{Path: sourceDir, Label: "test"}})

		createTestPlanFile(t, sourceDir, "orphan.md", sampleMarkdown)
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		require.NoError(t, os.Remove(filepath.Join(sourceDir, "orphan.md")))

		// Drop the configured dir the plan came from; nothing now matches it.
		unrelated := filepath.Join(tempDir, "unrelated")
		svc.sourcePlansDirs = []config.SyncDir{{Path: unrelated, Label: "unrelated"}}

		count, err := svc.DumpPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
		require.NoDirExists(t, unrelated)
		require.NoFileExists(t, filepath.Join(sourceDir, "orphan.md"))
	})
}
