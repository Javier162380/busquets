package claudeviewer

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	planviewer "github.com/Javier162380/claude-plan-viewer"
	clipboard_test "github.com/Javier162380/claude-plan-viewer/internal/clipboard/test"
	"github.com/Javier162380/claude-plan-viewer/internal/config"
	"github.com/Javier162380/claude-plan-viewer/internal/connectors"
	connectors_test "github.com/Javier162380/claude-plan-viewer/internal/connectors/test"
	"github.com/Javier162380/claude-plan-viewer/internal/storage"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
	postgresrepo "github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository/postgres"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository/sqlite"

	"github.com/golang/mock/gomock"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// newTestRepository creates a new SQLite repository with migrations for testing.
func newTestRepository(ctx context.Context, dbPath string) (*sqlite.Repository, error) {
	db, err := storage.NewSQLiteClientWithMigrations(ctx, dbPath, nil)
	if err != nil {
		return nil, err
	}
	return sqlite.NewRepository(db.DB()), nil
}

type mockNowProvider struct {
	currentTime time.Time
}

func (m *mockNowProvider) Now() time.Time {
	return m.currentTime
}

func newMockNowProvider(t time.Time) *mockNowProvider {
	return &mockNowProvider{currentTime: t}
}

var (
	sampleMarkdown = `# Test Plan

This is a test plan with some content for testing purposes.
It has multiple lines and should be useful for testing the word count functionality.
The quick brown fox jumps over the lazy dog. This sentence contains every letter of the alphabet.

## Section 1

Some more content here.

## Section 2

And even more content.`

	sampleMarkdownUpdated = `# Updated Plan

This plan has been updated with new content.
Now it has different text and a different word count.

## New Section

With some updated information.`

	sampleMarkdownWithCode = `# Plan with Code

Here is some code:

` + "```go" + `
func main() {
	println("Hello, World!")
}
` + "```" + `

And a list:
- Item 1
- Item 2
- Item 3`

	sampleMarkdownTable = `# Plan with Table

| Header 1 | Header 2 |
|----------|----------|
| Cell 1   | Cell 2   |
| Cell 3   | Cell 4   |

Some ~~strikethrough~~ text.`
)

// newServiceFromRepo creates a Service from an existing repository.
// It creates the viewer directory under tempDir and sets a deterministic clock.
// Returns the service and the resolved viewerDir path.
func newServiceFromRepo(t *testing.T, tempDir string, repo dto.Repository, syncDirs []config.SyncDir) (*Service, string) {
	t.Helper()
	viewerDir := filepath.Join(tempDir, "viewer")
	require.NoError(t, os.MkdirAll(viewerDir, 0o755))

	svc, err := New(repo, viewerDir, syncDirs, true)
	require.NoError(t, err)

	svc.nowProvider = newMockNowProvider(time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC))
	return svc, viewerDir
}

// newPostgresRepo creates an isolated Postgres database and returns the repository
// plus a cleanup function that drops the database and closes the pool.
func newPostgresRepo(t *testing.T) (dto.Repository, func()) {
	t.Helper()
	ctx := context.Background()

	dbName := fmt.Sprintf("testdb_%d", time.Now().UnixNano())
	adminConnStr := pgDBConnStr(pgBaseConnStr, "postgres")

	adminPool, err := pgxpool.New(ctx, adminConnStr)
	require.NoError(t, err)
	_, err = adminPool.Exec(ctx, "CREATE DATABASE "+dbName)
	adminPool.Close()
	require.NoError(t, err)

	testConnStr := pgDBConnStr(pgBaseConnStr, dbName)
	err = storage.RunPostgresMigrations(ctx, storage.PostgresConfig{ConnectionString: testConnStr}, nil)
	require.NoError(t, err)

	pool, err := storage.NewPostgresClient(ctx, storage.PostgresConfig{
		ConnectionString: testConnStr,
		MaxOpenConns:     5,
		MaxIdleConns:     2,
	})
	require.NoError(t, err)

	cleanup := func() {
		pool.Close()
		dropPool, dropErr := pgxpool.New(ctx, adminConnStr)
		if dropErr == nil {
			_, _ = dropPool.Exec(ctx, "DROP DATABASE "+dbName+" WITH (FORCE)")
			dropPool.Close()
		}
	}
	return postgresrepo.NewRepository(pool.Pool()), cleanup
}

func setupTestServiceBackendSQLite(t *testing.T) (*Service, string, string, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp(os.TempDir(), "claude-viewer-test-*")
	require.NoError(t, err)

	sourcePlansDir := filepath.Join(tempDir, "source")
	require.NoError(t, os.MkdirAll(sourcePlansDir, 0o755))

	ctx := context.Background()
	db, err := newTestRepository(ctx, filepath.Join(tempDir, "test.db"))
	require.NoError(t, err)

	svc, viewerDir := newServiceFromRepo(t, tempDir, db, []config.SyncDir{{Path: sourcePlansDir, Label: "test"}})
	return svc, sourcePlansDir, viewerDir, func() { os.RemoveAll(tempDir) }
}

func createTestPlanFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
}

func setupTestServiceBackendPostgres(t *testing.T) (*Service, string, string, func()) {
	t.Helper()

	repo, dbCleanup := newPostgresRepo(t)

	tempDir, err := os.MkdirTemp(os.TempDir(), "claude-viewer-pg-test-*")
	require.NoError(t, err)

	sourcePlansDir := filepath.Join(tempDir, "source")
	require.NoError(t, os.MkdirAll(sourcePlansDir, 0o755))

	svc, viewerDir := newServiceFromRepo(t, tempDir, repo, []config.SyncDir{{Path: sourcePlansDir, Label: "test"}})
	cleanup := func() { dbCleanup(); os.RemoveAll(tempDir) }
	return svc, sourcePlansDir, viewerDir, cleanup
}

// pgDBConnStr replaces the database name in a postgres:// connection URL.
func pgDBConnStr(base, dbName string) string {
	u, err := url.Parse(base)
	if err != nil {
		return base
	}
	u.Path = "/" + dbName
	return u.String()
}

// ---- Backend registry ----

type serviceSetupFn func(t *testing.T) (*Service, string, string, func())

// repoFn creates a repository backed by the given backend.
// tempDir is available for backends (e.g. SQLite) that store state on disk
// alongside the test's other files. cleanup is nil when the caller owns teardown.
type repoFn func(t *testing.T, tempDir string) (dto.Repository, func())

type backendSetup struct {
	name    string
	setupFn serviceSetupFn
	repoFn  repoFn
}

var (
	registeredBackends []backendSetup
	pgBaseConnStr      string // set once in init() when INTEGRATION=1
	pgContainerCancel  func() // terminates the shared container
)

func TestMain(m *testing.M) {
	registeredBackends = append(registeredBackends, backendSetup{
		name:    "sqlite",
		setupFn: setupTestServiceBackendSQLite,
		repoFn: func(t *testing.T, tempDir string) (dto.Repository, func()) {
			ctx := context.Background()
			db, err := newTestRepository(ctx, filepath.Join(tempDir, "test.db"))
			require.NoError(t, err)
			return db, nil // tempDir cleanup is the caller's responsibility
		},
	})

	if os.Getenv("INTEGRATION") != "" {
		ctx := context.Background()
		pgContainer, err := tcpostgres.Run(ctx,
			"postgres:16-alpine",
			tcpostgres.WithDatabase("postgres"),
			tcpostgres.WithUsername("test"),
			tcpostgres.WithPassword("test"),
			testcontainers.WithWaitStrategy(
				wait.ForLog("database system is ready to accept connections").
					WithOccurrence(2).
					WithStartupTimeout(60*time.Second),
			),
		)
		if err != nil {
			log.Fatalf("failed to start postgres container: %v", err)
		}
		connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
		if err != nil {
			log.Fatalf("failed to get postgres connection string: %v", err)
		}
		pgBaseConnStr = connStr
		pgContainerCancel = func() { _ = pgContainer.Terminate(context.Background()) }

		registeredBackends = append(registeredBackends, backendSetup{
			name:    "postgres",
			setupFn: setupTestServiceBackendPostgres,
			repoFn: func(t *testing.T, _ string) (dto.Repository, func()) {
				return newPostgresRepo(t) // tempDir not needed; Postgres manages its own storage
			},
		})
	}

	code := m.Run()
	if pgContainerCancel != nil {
		pgContainerCancel()
	}
	os.Exit(code)
}

// ---- Tests ----

func TestUtilityFunctions(t *testing.T) {
	t.Run("CountWords returns 0 for empty string", func(t *testing.T) {
		count := CountWords("")
		require.Equal(t, 0, count)
	})

	t.Run("CountWords counts single word", func(t *testing.T) {
		count := CountWords("hello")
		require.Equal(t, 1, count)
	})

	t.Run("CountWords counts multiple words", func(t *testing.T) {
		count := CountWords("hello world test")
		require.Equal(t, 3, count)
	})

	t.Run("CountWords handles punctuation correctly", func(t *testing.T) {
		count := CountWords("Hello, world! How are you?")
		require.Equal(t, 5, count)
	})

	t.Run("CountWords handles UTF-8 characters", func(t *testing.T) {
		count := CountWords("Hello café naïve résumé")
		require.Equal(t, 4, count)
	})

	t.Run("CountWords handles newlines and multiple spaces", func(t *testing.T) {
		count := CountWords("hello\n\nworld\t\ttest   multiple")
		require.Equal(t, 4, count)
	})

	t.Run("CalculateReadingTime returns at least 1 minute", func(t *testing.T) {
		minutes := CalculateReadingTime(10)
		require.Equal(t, 1, minutes)
	})

	t.Run("CalculateReadingTime calculates correctly for 200 words", func(t *testing.T) {
		minutes := CalculateReadingTime(200)
		require.Equal(t, 1, minutes)
	})

	t.Run("CalculateReadingTime rounds up", func(t *testing.T) {
		minutes := CalculateReadingTime(250)
		require.Equal(t, 2, minutes)
	})

	t.Run("CalculateReadingTime for large documents", func(t *testing.T) {
		minutes := CalculateReadingTime(1000)
		require.Equal(t, 5, minutes)
	})

	t.Run("extractTitle returns first heading", func(t *testing.T) {
		content := "# My Title\n\nSome content"
		title := extractTitle(content)
		require.Equal(t, "My Title", title)
	})

	t.Run("extractTitle handles content without heading", func(t *testing.T) {
		content := "Just some content without a heading"
		title := extractTitle(content)
		require.Equal(t, "Untitled Plan", title)
	})

	t.Run("extractTitle handles heading with extra spaces", func(t *testing.T) {
		content := "#     Spaced Title    \n\nContent"
		title := extractTitle(content)
		require.Equal(t, "Spaced Title", title)
	})

	t.Run("extractTitle ignores non-h1 headings", func(t *testing.T) {
		content := "## Second Level\n\nContent"
		title := extractTitle(content)
		require.Equal(t, "Untitled Plan", title)
	})
}

func TestMarkdownRendering(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			service, _, _, cleanup := b.setupFn(t)
			defer cleanup()
			testMarkdownRendering(t, service)
		})
	}
}

func testMarkdownRendering(t *testing.T, service *Service) {
	t.Helper()

	t.Run("RenderMarkdown renders plain text", func(t *testing.T) {
		html, err := service.RenderMarkdown("Hello world")
		require.NoError(t, err)
		require.Contains(t, html, "Hello world")
	})

	t.Run("RenderMarkdown renders headings", func(t *testing.T) {
		html, err := service.RenderMarkdown("# Heading 1\n## Heading 2")
		require.NoError(t, err)
		require.Contains(t, html, "<h1>Heading 1</h1>")
		require.Contains(t, html, "<h2>Heading 2</h2>")
	})

	t.Run("RenderMarkdown renders lists", func(t *testing.T) {
		markdown := "- Item 1\n- Item 2\n- Item 3"
		html, err := service.RenderMarkdown(markdown)
		require.NoError(t, err)
		require.Contains(t, html, "<ul>")
		require.Contains(t, html, "<li>Item 1</li>")
		require.Contains(t, html, "<li>Item 2</li>")
	})

	t.Run("RenderMarkdown renders code blocks", func(t *testing.T) {
		html, err := service.RenderMarkdown(sampleMarkdownWithCode)
		require.NoError(t, err)
		require.Contains(t, html, "<code")
		require.Contains(t, html, "main()")
	})

	t.Run("RenderMarkdown handles GFM tables", func(t *testing.T) {
		html, err := service.RenderMarkdown(sampleMarkdownTable)
		require.NoError(t, err)
		require.Contains(t, html, "<table>")
		require.Contains(t, html, "<th>Header 1</th>")
		require.Contains(t, html, "<td>Cell 1</td>")
	})

	t.Run("RenderMarkdown handles GFM strikethrough", func(t *testing.T) {
		html, err := service.RenderMarkdown(sampleMarkdownTable)
		require.NoError(t, err)
		require.Contains(t, html, "<del>strikethrough</del>")
	})
}

func TestSyncOperations(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testSyncOperations(t, b.setupFn)
		})
	}
}

func testSyncOperations(t *testing.T, setup serviceSetupFn) {
	t.Helper()

	t.Run("SyncPlans with empty source directory", func(t *testing.T) {
		service, _, _, cleanup := setup(t)
		defer cleanup()

		ctx := context.Background()
		count, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("SyncPlans syncs new file", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)

		ctx := context.Background()
		count, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, "Test Plan", plan.Title)
		require.Greater(t, plan.WordCount, int64(0))
	})

	t.Run("SyncPlans updates modified file", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		count, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		time.Sleep(10 * time.Millisecond)
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdownUpdated)

		count, err = service.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, "Updated Plan", plan.Title)
	})

	t.Run("SyncPlans skips unchanged files", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)

		count, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		count, err = service.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("SyncPlans handles multiple files concurrently", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		for i := 1; i <= 10; i++ {
			filename := filepath.Join(sourcePlansDir, "plan-"+string(rune('0'+i))+".md")
			createTestPlanFile(t, sourcePlansDir, filepath.Base(filename), sampleMarkdown)
		}

		count, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 10, count)
	})

	t.Run("SyncPlans ignores non-markdown files", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test.txt", "not markdown")
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)

		count, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("SyncPlans calculates word count correctly", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		content := "# Title\n\n" + strings.Repeat("word ", 100)
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", content)

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, int64(101), plan.WordCount)
	})
}

func TestRSyncOperations(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testRSyncOperations(t, b.setupFn)
		})
	}
}

func testRSyncOperations(t *testing.T, setup serviceSetupFn) {
	t.Helper()

	t.Run("RSyncPlans with empty database returns zero", func(t *testing.T) {
		service, _, _, cleanup := setup(t)
		defer cleanup()

		ctx := context.Background()
		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("RSyncPlans copies indexed plan from viewerDir back to sourcePlansDir", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		// Sync indexes the plan and copies it to viewerDir.
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)

		// Simulate the plan being deleted from sourcePlansDir.
		require.NoError(t, os.Remove(filepath.Join(sourcePlansDir, "test-plan.md")))

		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		content, err := os.ReadFile(filepath.Join(sourcePlansDir, "test-plan.md"))
		require.NoError(t, err)
		require.Equal(t, sampleMarkdown, string(content))
		content, err = os.ReadFile(plan.FilePath)
		require.NoError(t, err)
		require.Equal(t, sampleMarkdown, string(content))
	})

	t.Run("RSyncPlans skips plans already present in sourcePlansDir", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// File still exists in sourcePlansDir — nothing to restore.
		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("RSyncPlans skips plan when viewerDir copy is also missing", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)

		// Remove from both locations — nothing can be restored.
		require.NoError(t, os.Remove(filepath.Join(sourcePlansDir, "test-plan.md")))
		require.NoError(t, os.Remove(plan.FilePath))

		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("RSyncPlans skips files on disk that are not in database", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		// File exists in sourcePlansDir but was never synced — DB has no record.
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)

		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("RSyncPlans restores multiple plans deleted from sourcePlansDir", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		for i := 1; i <= 5; i++ {
			createTestPlanFile(t, sourcePlansDir, "plan-"+strconv.Itoa(i)+".md", sampleMarkdown)
		}
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		for i := 1; i <= 5; i++ {
			require.NoError(t, os.Remove(filepath.Join(sourcePlansDir, "plan-"+strconv.Itoa(i)+".md")))
		}

		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 5, count)

		for i := 1; i <= 5; i++ {
			_, err := os.Stat(filepath.Join(sourcePlansDir, "plan-"+strconv.Itoa(i)+".md"))
			require.NoError(t, err)
		}
	})

	t.Run("RSyncPlans only restores missing plans not all plans", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		for i := 1; i <= 5; i++ {
			createTestPlanFile(t, sourcePlansDir, "plan-"+strconv.Itoa(i)+".md", sampleMarkdown)
		}
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		require.NoError(t, os.Remove(filepath.Join(sourcePlansDir, "plan-2.md")))
		require.NoError(t, os.Remove(filepath.Join(sourcePlansDir, "plan-4.md")))

		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)
	})
}

func TestSearchAndListing(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testSearchAndListing(t, b.setupFn)
		})
	}
}

func testSearchAndListing(t *testing.T, setup serviceSetupFn) {
	t.Helper()

	t.Run("ListAllPlansWithReadingTime returns empty list for empty database", func(t *testing.T) {
		service, _, _, cleanup := setup(t)
		defer cleanup()

		ctx := context.Background()
		plans, err := service.ListAllPlansWithReadingTime(ctx)
		require.NoError(t, err)
		require.Empty(t, plans)
	})

	t.Run("ListAllPlansWithReadingTime returns all plans", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "plan1.md", sampleMarkdown)
		createTestPlanFile(t, sourcePlansDir, "plan2.md", sampleMarkdownUpdated)

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plans, err := service.ListAllPlansWithReadingTime(ctx)
		require.NoError(t, err)
		require.Len(t, plans, 2)
		require.Greater(t, plans[0].ReadingTime, 0)
		require.Greater(t, plans[1].ReadingTime, 0)
	})

	t.Run("ListAllPlansWithReadingTime sorts by modified_at DESC", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "old-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)
		createTestPlanFile(t, sourcePlansDir, "new-plan.md", sampleMarkdownUpdated)
		_, err = service.SyncPlans(ctx)
		require.NoError(t, err)

		plans, err := service.ListAllPlansWithReadingTime(ctx)
		require.NoError(t, err)
		require.Len(t, plans, 2)
		require.Equal(t, "new-plan.md", plans[0].FileName)
		require.Equal(t, "old-plan.md", plans[1].FileName)
	})

	t.Run("ListAllPlansWithReadingTime sorts by modified_at ASC", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "old-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)
		createTestPlanFile(t, sourcePlansDir, "new-plan.md", sampleMarkdownUpdated)
		_, err = service.SyncPlans(ctx)
		require.NoError(t, err)

		sortKey, sortDir := SortKeyUpdatedAt, SortDirAsc
		require.NoError(t, service.SetSetting(ctx, SettingPlansSortKey, SettingValues{StringValue: &sortKey}))
		require.NoError(t, service.SetSetting(ctx, SettingPlansSortDir, SettingValues{StringValue: &sortDir}))

		plans, err := service.ListAllPlansWithReadingTime(ctx)
		require.NoError(t, err)
		require.Len(t, plans, 2)
		require.Equal(t, "old-plan.md", plans[0].FileName)
		require.Equal(t, "new-plan.md", plans[1].FileName)
	})

	t.Run("ListAllPlansWithReadingTime sorts by size DESC", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "large-plan.md", sampleMarkdown)
		createTestPlanFile(t, sourcePlansDir, "small-plan.md", sampleMarkdownUpdated)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		sortKey, sortDir := SortKeySize, SortDirDesc
		require.NoError(t, service.SetSetting(ctx, SettingPlansSortKey, SettingValues{StringValue: &sortKey}))
		require.NoError(t, service.SetSetting(ctx, SettingPlansSortDir, SettingValues{StringValue: &sortDir}))

		plans, err := service.ListAllPlansWithReadingTime(ctx)
		require.NoError(t, err)
		require.Len(t, plans, 2)
		require.Greater(t, plans[0].FileSize, plans[1].FileSize)
		require.Equal(t, "large-plan.md", plans[0].FileName)
	})

	t.Run("ListAllPlansWithReadingTime sorts by reading_time DESC", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "verbose-plan.md", sampleMarkdown)
		createTestPlanFile(t, sourcePlansDir, "terse-plan.md", sampleMarkdownUpdated)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		sortKey, sortDir := SortKeyReadingTime, SortDirDesc
		require.NoError(t, service.SetSetting(ctx, SettingPlansSortKey, SettingValues{StringValue: &sortKey}))
		require.NoError(t, service.SetSetting(ctx, SettingPlansSortDir, SettingValues{StringValue: &sortDir}))

		plans, err := service.ListAllPlansWithReadingTime(ctx)
		require.NoError(t, err)
		require.Len(t, plans, 2)
		require.Equal(t, "verbose-plan.md", plans[0].FileName)
		require.Equal(t, "terse-plan.md", plans[1].FileName)
	})

	t.Run("SearchPlansWithReadingTime with empty query returns all plans", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "plan1.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plans, err := service.SearchPlansWithReadingTime(ctx, "")
		require.NoError(t, err)
		require.Len(t, plans, 1)
	})

	t.Run("SearchPlansWithReadingTime matches title", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "plan1.md", "# Unique Title\n\nContent")
		createTestPlanFile(t, sourcePlansDir, "plan2.md", "# Another Title\n\nContent")
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plans, err := service.SearchPlansWithReadingTime(ctx, "Unique")
		require.NoError(t, err)
		require.Len(t, plans, 1)
		require.Equal(t, "Unique Title", plans[0].Title)
	})

	t.Run("SearchPlansWithReadingTime matches content", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "plan1.md", "# Title\n\nSpecialKeyword content")
		createTestPlanFile(t, sourcePlansDir, "plan2.md", "# Title\n\nDifferent content")
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plans, err := service.SearchPlansWithReadingTime(ctx, "SpecialKeyword")
		require.NoError(t, err)
		require.Len(t, plans, 1)
		require.Equal(t, "plan1.md", plans[0].FileName)
	})

	t.Run("SearchPlansWithReadingTime returns empty for no matches", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "plan1.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plans, err := service.SearchPlansWithReadingTime(ctx, "NonExistentKeyword")
		require.NoError(t, err)
		require.Empty(t, plans)
	})

	t.Run("SearchPlansWithReadingTime is case insensitive", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "plan1.md", "# CamelCase Title\n\nContent")
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plans, err := service.SearchPlansWithReadingTime(ctx, "CamelCase")
		require.NoError(t, err)
		require.Len(t, plans, 1)
	})
}

func TestPlanRetrieval(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testPlanRetrieval(t, b.setupFn)
		})
	}
}

func testPlanRetrieval(t *testing.T, setup serviceSetupFn) {
	t.Helper()

	t.Run("GetPlanByFileName returns plan for existing file", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.NotNil(t, plan)
		require.Equal(t, "test-plan.md", plan.FileName)
		require.Equal(t, "Test Plan", plan.Title)
	})

	t.Run("GetPlanByFileName returns error for non-existent file", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		_, err := service.GetPlanByFileName(ctx, "non-existent.md", sourcePlansDir)
		require.Error(t, err)
	})

	t.Run("GetPlanDetailByFileName returns plan with rendered HTML", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		detail, err := service.GetPlanDetailByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.NotNil(t, detail)
		require.Equal(t, "Test Plan", detail.Title)
		require.Contains(t, detail.RenderedHTML, "<h1>Test Plan</h1>")
		require.Contains(t, detail.RenderedHTML, "<h2>Section 1</h2>")
		require.Greater(t, detail.ReadingTime, 0)
	})

	t.Run("GetPlanDetailByFileName returns error for non-existent file", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		_, err := service.GetPlanDetailByFileName(ctx, "non-existent.md", sourcePlansDir)
		require.Error(t, err)
	})

	t.Run("GetPlanDetailByFileName calculates reading time correctly", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		longContent := "# Long Plan\n\n" + strings.Repeat("word ", 500)
		createTestPlanFile(t, sourcePlansDir, "long-plan.md", longContent)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		detail, err := service.GetPlanDetailByFileName(ctx, "long-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, 3, detail.ReadingTime)
	})
}

func TestUpdateOperations(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testUpdateOperations(t, b.setupFn)
		})
	}
}

func testUpdateOperations(t *testing.T, setup serviceSetupFn) {
	t.Helper()

	t.Run("UpdatePlan succeeds when no conflict", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)

		result, err := service.UpdatePlan(ctx, UpdatePlanRequest{
			SyncSource:       sourcePlansDir,
			FileName:         "test-plan.md",
			NewContent:       sampleMarkdownUpdated,
			LastModifiedTime: plan.ModifiedAt,
		})
		require.NoError(t, err)
		require.True(t, result.Success)
		require.False(t, result.HasConflict)

		updatedPlan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, "Updated Plan", updatedPlan.Title)

		sourceContent, err := os.ReadFile(filepath.Join(sourcePlansDir, "test-plan.md"))
		require.NoError(t, err)
		require.Equal(t, sampleMarkdownUpdated, string(sourceContent))

		viewerContent, err := os.ReadFile(plan.FilePath)
		require.NoError(t, err)
		require.Equal(t, sampleMarkdownUpdated, string(viewerContent))
	})

	t.Run("UpdatePlan detects conflict when file modified externally", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# External Change\n\nModified externally")

		result, err := service.UpdatePlan(ctx, UpdatePlanRequest{
			SyncSource:       sourcePlansDir,
			FileName:         "test-plan.md",
			NewContent:       sampleMarkdownUpdated,
			LastModifiedTime: plan.ModifiedAt,
		})
		require.NoError(t, err)
		require.False(t, result.Success)
		require.True(t, result.HasConflict)
		require.NotNil(t, result.ConflictInfo)
		require.Contains(t, result.ConflictInfo.Message, "modified externally")
	})

	t.Run("UpdatePlan recalculates word count", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		originalWordCount := plan.WordCount

		longContent := "# Updated\n\n" + strings.Repeat("word ", 300)
		result, err := service.UpdatePlan(ctx, UpdatePlanRequest{
			SyncSource:       sourcePlansDir,
			FileName:         "test-plan.md",
			NewContent:       longContent,
			LastModifiedTime: plan.ModifiedAt,
		})
		require.NoError(t, err)
		require.True(t, result.Success)

		updatedPlan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Greater(t, updatedPlan.WordCount, originalWordCount)
		require.Equal(t, int64(301), updatedPlan.WordCount)
	})

	t.Run("UpdatePlan updates database timestamps", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)

		result, err := service.UpdatePlan(ctx, UpdatePlanRequest{
			SyncSource:       sourcePlansDir,
			FileName:         "test-plan.md",
			NewContent:       sampleMarkdownUpdated,
			LastModifiedTime: plan.ModifiedAt,
		})
		require.NoError(t, err)
		require.True(t, result.Success)

		updatedPlan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.True(t, updatedPlan.ModifiedAt.After(plan.ModifiedAt) || updatedPlan.ModifiedAt.Equal(plan.ModifiedAt))
		require.True(t, updatedPlan.IndexedAt.After(plan.IndexedAt) || updatedPlan.IndexedAt.Equal(plan.IndexedAt))
	})
}

func TestPagination(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testPagination(t, b.setupFn)
		})
	}
}

func testPagination(t *testing.T, setup serviceSetupFn) {
	t.Helper()

	// Token encoding/decoding tests — pure functions, no DB involved.
	t.Run("EncodePaginationToken encodes offset to base64", func(t *testing.T) {
		token := EncodePaginationToken(0)
		require.NotEmpty(t, token)
		require.True(t, len(token) > 0)
		decoded, err := DecodePaginationToken(token)
		require.NoError(t, err)
		require.Equal(t, int64(0), decoded.Offset)
	})

	t.Run("EncodePaginationToken handles non-zero offsets", func(t *testing.T) {
		token := EncodePaginationToken(100)
		decoded, err := DecodePaginationToken(token)
		require.NoError(t, err)
		require.Equal(t, int64(100), decoded.Offset)
	})

	t.Run("DecodePaginationToken handles empty string as offset 0", func(t *testing.T) {
		decoded, err := DecodePaginationToken("")
		require.NoError(t, err)
		require.Equal(t, int64(0), decoded.Offset)
	})

	t.Run("DecodePaginationToken returns error for invalid base64", func(t *testing.T) {
		_, err := DecodePaginationToken("invalid!!!base64")
		require.Error(t, err)
	})

	t.Run("DecodePaginationToken clamps negative offset to 0", func(t *testing.T) {
		invalidToken := "eyJvZmZzZXQiOi0xMH0=" // base64 encoded {"offset":-10}
		decoded, err := DecodePaginationToken(invalidToken)
		require.NoError(t, err)
		require.Equal(t, int64(0), decoded.Offset)
	})

	t.Run("PaginationToken roundtrip preserves offset", func(t *testing.T) {
		testOffsets := []int64{0, 1, 20, 100, 1000, 999999}
		for _, offset := range testOffsets {
			token := EncodePaginationToken(offset)
			decoded, err := DecodePaginationToken(token)
			require.NoError(t, err)
			require.Equal(t, offset, decoded.Offset, "Offset mismatch for %d", offset)
		}
	})

	t.Run("ListAllPlansWithPaginationAndReadingTime returns first page", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		for i := 0; i < 25; i++ {
			filename := fmt.Sprintf("plan-%02d.md", i)
			content := fmt.Sprintf("# Plan %d\n\nContent for plan %d.", i, i)
			createTestPlanFile(t, sourcePlansDir, filename, content)
		}

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plans, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 20, 0)
		require.NoError(t, err)
		require.Equal(t, 20, len(plans))
	})

	t.Run("ListAllPlansWithPaginationAndReadingTime respects offset", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		for i := 0; i < 25; i++ {
			filename := fmt.Sprintf("plan-%02d.md", i)
			content := fmt.Sprintf("# Plan %d\n\nContent for plan %d.", i, i)
			createTestPlanFile(t, sourcePlansDir, filename, content)
		}

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		firstPage, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 20, 0)
		require.NoError(t, err)

		secondPage, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 20, 20)
		require.NoError(t, err)

		require.Equal(t, 5, len(secondPage))
		firstPageTitles := make(map[string]bool)
		for _, p := range firstPage {
			firstPageTitles[p.Title] = true
		}
		for _, p := range secondPage {
			require.False(t, firstPageTitles[p.Title], "Plan appears in both pages")
		}
	})

	t.Run("SearchPlansWithPaginationAndReadingTime returns matching results paginated", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		for i := 0; i < 15; i++ {
			filename := fmt.Sprintf("plan-%02d.md", i)
			content := fmt.Sprintf("# Backend Plan %d\n\nBackend implementation.", i)
			createTestPlanFile(t, sourcePlansDir, filename, content)
		}
		for i := 0; i < 10; i++ {
			filename := fmt.Sprintf("frontend-%02d.md", i)
			content := fmt.Sprintf("# Frontend Plan %d\n\nFrontend development.", i)
			createTestPlanFile(t, sourcePlansDir, filename, content)
		}

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		results, err := service.SearchPlansWithPaginationAndReadingTime(ctx, "Backend", 10, 0)
		require.NoError(t, err)
		require.Equal(t, 10, len(results))

		for _, plan := range results {
			require.Contains(t, plan.Title, "Backend")
		}

		nextResults, err := service.SearchPlansWithPaginationAndReadingTime(ctx, "Backend", 10, 10)
		require.NoError(t, err)
		require.Equal(t, 5, len(nextResults))
	})

	t.Run("SearchPlansWithPaginationAndReadingTime empty query returns all paginated", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		for i := 0; i < 25; i++ {
			filename := fmt.Sprintf("plan-%02d.md", i)
			content := fmt.Sprintf("# Plan %d\n\nContent.", i)
			createTestPlanFile(t, sourcePlansDir, filename, content)
		}

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		results, err := service.SearchPlansWithPaginationAndReadingTime(ctx, "", 20, 0)
		require.NoError(t, err)
		require.Equal(t, 20, len(results))
	})

	t.Run("PaginatedSearch includes reading time calculations", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		content := "# Test Plan\n\n" + strings.Repeat("word ", 400)
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", content)

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		results, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 20, 0)
		require.NoError(t, err)
		require.Equal(t, 1, len(results))

		plan := results[0]
		require.Greater(t, plan.ReadingTime, 0)
		require.GreaterOrEqual(t, plan.ReadingTime, 2)
	})

	t.Run("ListAllPlansWithPaginationAndReadingTime with offset beyond results", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		results, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 20, 1000)
		require.NoError(t, err)
		require.Equal(t, 0, len(results))
	})

	t.Run("Pagination with custom page size", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		for i := 0; i < 10; i++ {
			filename := fmt.Sprintf("plan-%d.md", i)
			content := fmt.Sprintf("# Plan %d\n\nContent.", i)
			createTestPlanFile(t, sourcePlansDir, filename, content)
		}

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		page1, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 3, 0)
		require.NoError(t, err)
		require.Equal(t, 3, len(page1))

		page2, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 3, 3)
		require.NoError(t, err)
		require.Equal(t, 3, len(page2))

		page4, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 3, 9)
		require.NoError(t, err)
		require.Equal(t, 1, len(page4))
	})
}

func TestVersionOperations(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			service, sourcePlansDir, _, cleanup := b.setupFn(t)
			defer cleanup()
			testVersionOperations(t, service, sourcePlansDir)
		})
	}
}

func testVersionOperations(t *testing.T, service *Service, sourcePlansDir string) {
	t.Helper()
	ctx := context.Background()

	t.Run("SavePlanVersion creates new version", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "test-plan.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SavePlanVersion(ctx, "test-plan.md", sourcePlansDir, sampleMarkdown)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		versionDir := service.versionsDirFor(plan.ID)
		entries, err := os.ReadDir(versionDir)
		require.NoError(t, err)
		require.Equal(t, 1, len(entries))
		require.True(t, strings.HasSuffix(entries[0].Name(), ".md"))
	})

	t.Run("GetLatestVersionNumber increments correctly", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "version-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SavePlanVersion(ctx, "version-test.md", sourcePlansDir, sampleMarkdown)
		require.NoError(t, err)

		err = service.SavePlanVersion(ctx, "version-test.md", sourcePlansDir, sampleMarkdownUpdated)
		require.NoError(t, err)

		count, err := service.GetVersionCount(ctx, "version-test.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, int64(2), count)
	})

	t.Run("GetPlanVersionHistory returns versions in reverse order", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "history-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		for i := 0; i < 3; i++ {
			err = service.SavePlanVersion(ctx, "history-test.md", sourcePlansDir, fmt.Sprintf("# Version %d\n\nContent %d", i+1, i+1))
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
		}

		versions, err := service.GetPlanVersionHistory(ctx, "history-test.md", sourcePlansDir, 0, 10)
		require.NoError(t, err)
		require.Equal(t, 3, len(versions))

		for i := 0; i < len(versions)-1; i++ {
			require.Greater(t, versions[i].VersionNumber, versions[i+1].VersionNumber)
		}
	})

	t.Run("GetPlanVersion retrieves specific version", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "specific-version.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		testContent := "# Specific Version\n\nThis is version 1"
		err = service.SavePlanVersion(ctx, "specific-version.md", sourcePlansDir, testContent)
		require.NoError(t, err)

		version, err := service.GetPlanVersion(ctx, "specific-version.md", sourcePlansDir, 1)
		require.NoError(t, err)
		require.Equal(t, int64(1), version.VersionNumber)
		require.Equal(t, testContent, version.Content)
	})

	t.Run("GetVersionCount returns correct count", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "count-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		count, err := service.GetVersionCount(ctx, "count-test.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, int64(0), count)

		for i := 0; i < 5; i++ {
			err = service.SavePlanVersion(ctx, "count-test.md", sourcePlansDir, fmt.Sprintf("Version %d", i+1))
			require.NoError(t, err)
		}

		count, err = service.GetVersionCount(ctx, "count-test.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, int64(5), count)
	})

	t.Run("SavePlanVersion maintains file/database consistency on DB failure", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "consistency-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SavePlanVersion(ctx, "consistency-test.md", sourcePlansDir, sampleMarkdown)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "consistency-test.md", sourcePlansDir)
		require.NoError(t, err)
		versionDir := service.versionsDirFor(plan.ID)
		entries, err := os.ReadDir(versionDir)
		require.NoError(t, err)
		initialCount := len(entries)

		count, err := service.GetVersionCount(ctx, "consistency-test.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, int64(1), count)
		require.Equal(t, initialCount, 1)
	})

	t.Run("CleanupOldVersions keeps most recent versions", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "cleanup-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		for i := 1; i <= 7; i++ {
			err = service.SavePlanVersion(ctx, "cleanup-test.md", sourcePlansDir, fmt.Sprintf("# Version %d", i))
			require.NoError(t, err)
		}

		count, err := service.GetVersionCount(ctx, "cleanup-test.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, int64(7), count)

		err = service.CleanupOldVersions(ctx, "cleanup-test.md", sourcePlansDir, 5)
		require.NoError(t, err)

		count, err = service.GetVersionCount(ctx, "cleanup-test.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, int64(5), count)

		versions, err := service.GetPlanVersionHistory(ctx, "cleanup-test.md", sourcePlansDir, 0, 10)
		require.NoError(t, err)
		require.Equal(t, 5, len(versions))
		require.Equal(t, int64(7), versions[0].VersionNumber)
	})

	t.Run("Version content preserves full markdown", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "markdown-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdownWithCode), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SavePlanVersion(ctx, "markdown-test.md", sourcePlansDir, sampleMarkdownWithCode)
		require.NoError(t, err)

		version, err := service.GetPlanVersion(ctx, "markdown-test.md", sourcePlansDir, 1)
		require.NoError(t, err)

		require.Equal(t, sampleMarkdownWithCode, version.Content)
		require.Contains(t, version.Content, "```go")
		require.Contains(t, version.Content, "func main()")
	})

	t.Run("Version word count is calculated correctly", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "wordcount-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SavePlanVersion(ctx, "wordcount-test.md", sourcePlansDir, sampleMarkdown)
		require.NoError(t, err)

		version, err := service.GetPlanVersion(ctx, "wordcount-test.md", sourcePlansDir, 1)
		require.NoError(t, err)

		require.Greater(t, version.WordCount, int64(0))
		expectedCount := CountWords(sampleMarkdown)
		require.Equal(t, int64(expectedCount), version.WordCount)
	})

	t.Run("GetPlanVersionHistory respects pagination", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "pagination-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		for i := 1; i <= 10; i++ {
			err = service.SavePlanVersion(ctx, "pagination-test.md", sourcePlansDir, fmt.Sprintf("# Version %d", i))
			require.NoError(t, err)
		}

		page1, err := service.GetPlanVersionHistory(ctx, "pagination-test.md", sourcePlansDir, 0, 3)
		require.NoError(t, err)
		require.Equal(t, 3, len(page1))

		page2, err := service.GetPlanVersionHistory(ctx, "pagination-test.md", sourcePlansDir, 3, 3)
		require.NoError(t, err)
		require.Equal(t, 3, len(page2))

		require.NotEqual(t, page1[0].ID, page2[0].ID)
	})

	t.Run("SavePlanVersion fails gracefully for non-existent plan", func(t *testing.T) {
		err := service.SavePlanVersion(ctx, "non-existent.md", sourcePlansDir, "some content")
		require.Error(t, err)
	})

	t.Run("SearchVersions returns versions matching query", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "search-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		v1Content := "# Planning\nThis is about project planning and design"
		v2Content := "# Implementation\nThis is about implementation details"
		v3Content := "# Testing\nThis is about test cases"

		require.NoError(t, service.SavePlanVersion(ctx, "search-test.md", sourcePlansDir, v1Content))
		require.NoError(t, service.SavePlanVersion(ctx, "search-test.md", sourcePlansDir, v2Content))
		require.NoError(t, service.SavePlanVersion(ctx, "search-test.md", sourcePlansDir, v3Content))

		results, err := service.SearchVersions(ctx, "search-test.md", sourcePlansDir, "planning")
		require.NoError(t, err)
		require.Equal(t, 1, len(results), "should find 1 version with 'planning'")
		require.Equal(t, int64(1), results[0].VersionNumber)

		results, err = service.SearchVersions(ctx, "search-test.md", sourcePlansDir, "implementation")
		require.NoError(t, err)
		require.Equal(t, 1, len(results), "should find 1 version with 'implementation'")
		require.Equal(t, int64(2), results[0].VersionNumber)

		results, err = service.SearchVersions(ctx, "search-test.md", sourcePlansDir, "nonexistent")
		require.NoError(t, err)
		require.Equal(t, 0, len(results), "should find no versions with 'nonexistent'")
	})
}

func TestConnectorManager(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testConnectorManager(t, b.setupFn)
		})
	}
}

func testConnectorManager(t *testing.T, setup serviceSetupFn) {
	t.Helper()

	t.Run("GetEnabledConnector returns nil when none enabled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		connector, err := manager.GetEnabledConnector(ctx)
		require.NoError(t, err)
		require.Nil(t, connector)
	})

	t.Run("EnableConnector fails for unregistered connector", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		registry := connectors.NewRegistry()
		manager := connectors.NewManager(registry, service.DB())

		err := manager.EnableConnector(ctx, "non-existent")
		require.Error(t, err)
		require.True(t, planviewer.IsConnectorNotFound(err), "expected not found error, got: %v", err)
	})

	t.Run("EnableConnector enables registered connector", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		err := manager.EnableConnector(ctx, "mock-connector")
		require.NoError(t, err)

		connector, err := manager.GetEnabledConnector(ctx)
		require.NoError(t, err)
		require.NotNil(t, connector)
		require.Equal(t, "mock-connector", connector.Name())
	})

	t.Run("DisableConnector disables all connectors", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		require.NoError(t, manager.EnableConnector(ctx, "mock-connector"))

		err := manager.DisableConnector(ctx)
		require.NoError(t, err)

		connector, err := manager.GetEnabledConnector(ctx)
		require.NoError(t, err)
		require.Nil(t, connector)
	})

	t.Run("SetConnectorSetting and GetConnectorSetting", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		err := manager.SetConnectorSetting(ctx, "mock-connector", "api_token", "test-token", true)
		require.NoError(t, err)

		value, exists, err := manager.GetConnectorSetting(ctx, "mock-connector", "api_token")
		require.NoError(t, err)
		require.True(t, exists)
		require.Equal(t, "test-token", value)
	})

	t.Run("GetConnectorSetting returns false for non-existent setting", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		_, exists, err := manager.GetConnectorSetting(ctx, "mock-connector", "non-existent")
		require.NoError(t, err)
		require.False(t, exists)
	})

	t.Run("ListAvailable returns all registered connectors with status", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		statuses, err := manager.ListAvailable(ctx)
		require.NoError(t, err)
		require.Len(t, statuses, 1)
		require.Equal(t, "mock-connector", statuses[0].Name)
		require.Equal(t, "Mock Connector", statuses[0].DisplayName)
		require.Nil(t, statuses[0].Role)
		require.False(t, statuses[0].Configured)
	})

	t.Run("ListAvailable shows configured status when required settings present", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		require.NoError(t, manager.SetConnectorSetting(ctx, "mock-connector", "api_token", "token", true))
		require.NoError(t, manager.SetConnectorSetting(ctx, "mock-connector", "channel_id", "123", false))

		statuses, err := manager.ListAvailable(ctx)
		require.NoError(t, err)
		require.Len(t, statuses, 1)
		require.True(t, statuses[0].Configured)
	})

	t.Run("EnsureConnectorExists creates connector record", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		err := manager.EnsureConnectorExists(ctx, "mock-connector")
		require.NoError(t, err)

		err = manager.EnsureConnectorExists(ctx, "mock-connector")
		require.NoError(t, err)
	})

	t.Run("EnsureConnectorExists fails for unregistered connector", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		registry := connectors.NewRegistry()
		manager := connectors.NewManager(registry, service.DB())

		err := manager.EnsureConnectorExists(ctx, "unregistered")
		require.Error(t, err)
		require.True(t, planviewer.IsConnectorNotFound(err), "expected not found error, got: %v", err)
	})

	t.Run("GetConnectorRequiredSettings returns settings definitions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		settings, err := manager.GetConnectorRequiredSettings("mock-connector")
		require.NoError(t, err)
		require.Len(t, settings, 2)
		require.Equal(t, "api_token", settings[0].Key)
		require.True(t, settings[0].Sensitive)
		require.Equal(t, "channel_id", settings[1].Key)
		require.False(t, settings[1].Sensitive)
	})

	t.Run("GetConnectorRequiredSettings fails for unknown connector", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()

		registry := connectors.NewRegistry()
		manager := connectors.NewManager(registry, service.DB())

		_, err := manager.GetConnectorRequiredSettings("unknown")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not found")
	})

	t.Run("Send fails when no connector enabled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		_, err := manager.Send(ctx, "Title", "Content")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no connector enabled")
	})

	t.Run("Send calls connector with correct arguments", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		mockConn.EXPECT().Validate().Return(nil).Times(1)
		msgID := "msg-123"
		mockConn.EXPECT().Send(gomock.Any(), "Test Title", "Test Content").Return(
			&connectors.SendResult{Success: true, MessageID: &msgID},
			nil,
		).Times(1)

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		require.NoError(t, manager.EnableConnector(ctx, "mock-connector"))

		result, err := manager.Send(ctx, "Test Title", "Test Content")
		require.NoError(t, err)
		require.True(t, result.Success)
		require.Equal(t, "msg-123", *result.MessageID)
	})

	t.Run("Send fails when connector validation fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		mockConn.EXPECT().Validate().Return(fmt.Errorf("missing api_token")).Times(1)

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		require.NoError(t, manager.EnableConnector(ctx, "mock-connector"))

		_, err := manager.Send(ctx, "Title", "Content")
		require.Error(t, err)
		require.Contains(t, err.Error(), "validation failed")
		require.Contains(t, err.Error(), "missing api_token")
	})

	t.Run("Send returns error from connector", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		mockConn.EXPECT().Validate().Return(nil).Times(1)
		mockConn.EXPECT().Send(gomock.Any(), gomock.Any(), gomock.Any()).Return(
			nil,
			fmt.Errorf("network timeout"),
		).Times(1)

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		require.NoError(t, manager.EnableConnector(ctx, "mock-connector"))

		_, err := manager.Send(ctx, "Title", "Content")
		require.Error(t, err)
		require.Contains(t, err.Error(), "network timeout")
	})
}

func TestServiceConnectorOperations(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testServiceConnectorOperations(t, b.setupFn)
		})
	}
}

func testServiceConnectorOperations(t *testing.T, setup serviceSetupFn) {
	t.Helper()

	t.Run("Operations fail when connector manager not set", func(t *testing.T) {
		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		err := service.EnableConnector(ctx, "any")
		require.Error(t, err)
		require.Contains(t, err.Error(), "not initialized")

		err = service.DisableConnector(ctx)
		require.Error(t, err)

		err = service.ConfigureConnector(ctx, "any", "key", "value", false)
		require.Error(t, err)

		_, err = service.GetConnectorSettings(ctx, "any")
		require.Error(t, err)

		err = service.ValidateConnector(ctx, "any")
		require.Error(t, err)
	})

	t.Run("ListConnectors returns nil when manager not set", func(t *testing.T) {
		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		infos, err := service.ListConnectors(ctx)
		require.NoError(t, err)
		require.Nil(t, infos)
	})

	t.Run("GetEnabledConnector returns nil when manager not set", func(t *testing.T) {
		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		info, err := service.GetEnabledConnector(ctx)
		require.NoError(t, err)
		require.Nil(t, info)
	})

	t.Run("Full connector workflow through service", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "test-conn", "Test Connector")

		mockConn.EXPECT().Validate().Return(nil).Times(1)
		sentID := "sent-123"
		mockConn.EXPECT().Send(gomock.Any(), "Test Plan", "# Test Plan\n\nContent to send").Return(
			&connectors.SendResult{Success: true, MessageID: &sentID},
			nil,
		).Times(1)

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)

		infos, err := service.ListConnectors(ctx)
		require.NoError(t, err)
		require.Len(t, infos, 1)
		require.Equal(t, "test-conn", infos[0].Name)

		err = service.EnableConnector(ctx, "test-conn")
		require.NoError(t, err)

		info, err := service.GetEnabledConnector(ctx)
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Equal(t, "test-conn", info.Name)
		require.True(t, info.IsTransmit())

		err = service.ConfigureConnector(ctx, "test-conn", "api_token", "my-secret-token", true)
		require.NoError(t, err)

		settings, err := service.GetConnectorSettings(ctx, "test-conn")
		require.NoError(t, err)
		require.Len(t, settings, 2)

		var tokenSetting ConnectorSettingInfo
		for _, s := range settings {
			if s.Key == "api_token" {
				tokenSetting = s
				break
			}
		}
		require.Equal(t, "api_token", tokenSetting.Key)
		require.Equal(t, "my-secret-token", tokenSetting.Value)
		require.True(t, tokenSetting.Sensitive)

		createTestPlanFile(t, sourcePlansDir, "connector-test.md", "# Test Plan\n\nContent to send")
		_, err = service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SendToConnector(ctx, "connector-test.md", sourcePlansDir)
		require.NoError(t, err)

		err = service.DisableConnector(ctx)
		require.NoError(t, err)

		info, err = service.GetEnabledConnector(ctx)
		require.NoError(t, err)
		require.Nil(t, info)
	})

	t.Run("SendToConnector fails when no connector enabled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "test-conn", "Test Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)

		createTestPlanFile(t, sourcePlansDir, "send-test.md", "# Test Plan")
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SendToConnector(ctx, "send-test.md", sourcePlansDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "no connector enabled")
	})

	t.Run("SendToConnector fails for non-existent plan", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "test-conn", "Test Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)
		require.NoError(t, manager.EnableConnector(ctx, "test-conn"))

		err := service.SendToConnector(ctx, "non-existent.md", sourcePlansDir)
		require.Error(t, err)
		require.True(t, dto.IsNotFound(err), "expected not found error, got: %v", err)
	})

	t.Run("ValidateConnector calls connector Validate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "test-conn", "Test Connector")

		mockConn.EXPECT().Validate().Return(fmt.Errorf("api_token is required")).Times(1)

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)

		err := service.ValidateConnector(ctx, "test-conn")
		require.Error(t, err)
		require.Contains(t, err.Error(), "api_token is required")
	})

	t.Run("ValidateConnector succeeds when connector is valid", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "test-conn", "Test Connector")

		mockConn.EXPECT().Validate().Return(nil).Times(1)

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)

		err := service.ValidateConnector(ctx, "test-conn")
		require.NoError(t, err)
	})

	t.Run("GenerateSummary returns summary when summarizer is configured", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "ollama", "Ollama (Local LLM)")

		summary := "**Goal**: ship the feature\n**Approach**: TDD\n**Outcome**: done"
		mockConn.EXPECT().Validate().Return(nil).Times(1)
		mockConn.EXPECT().Send(gomock.Any(), "Summary Plan", "# Summary Plan\n\nSome content").Return(
			&connectors.SendResult{Success: true, Response: &summary},
			nil,
		).Times(1)

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)

		require.NoError(t, service.SetSummaryConnector(ctx, "ollama"))

		createTestPlanFile(t, sourcePlansDir, "summary-plan.md", "# Summary Plan\n\nSome content")
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		result, err := service.GenerateSummary(ctx, "summary-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, summary, result)
	})

	t.Run("GenerateSummary fails when no summarizer is configured", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		registry := connectors.NewRegistry()
		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)

		createTestPlanFile(t, sourcePlansDir, "no-summarizer.md", "# No Summarizer\n\nContent")
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		_, err = service.GenerateSummary(ctx, "no-summarizer.md", sourcePlansDir)
		require.ErrorIs(t, err, planviewer.ErrNoSummarizerConfigured)
	})

	t.Run("ClearSummaryConnector clears the summary slot", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "ollama", "Ollama (Local LLM)")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)

		require.NoError(t, service.SetSummaryConnector(ctx, "ollama"))

		createTestPlanFile(t, sourcePlansDir, "clear-summary.md", "# Clear Summary\n\nContent")
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		require.NoError(t, service.ClearSummaryConnector(ctx))

		_, err = service.GenerateSummary(ctx, "clear-summary.md", sourcePlansDir)
		require.ErrorIs(t, err, planviewer.ErrNoSummarizerConfigured)
	})
}

func TestConcurrentVersionSaves(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			service, sourcePlansDir, _, cleanup := b.setupFn(t)
			defer cleanup()
			testConcurrentVersionSaves(t, service, sourcePlansDir)
		})
	}
}

func testConcurrentVersionSaves(t *testing.T, service *Service, sourcePlansDir string) {
	t.Helper()
	ctx := context.Background()

	t.Run("Concurrent version saves both trying to create same version number deletes orphaned file on conflict", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "concurrent-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SavePlanVersion(ctx, "concurrent-test.md", sourcePlansDir, "# Version 1")
		require.NoError(t, err)

		plan, err := service.db.GetPlanByFileName(ctx, "concurrent-test.md", sourcePlansDir)
		require.NoError(t, err)

		versionDir := service.versionsDirFor(plan.ID)

		fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
		err = service.db.InsertPlanVersion(ctx, dto.InsertPlanVersionParams{
			PlanID:        plan.ID,
			VersionNumber: 2,
			FilePath:      filepath.Join(versionDir, "2-12345.md"),
			Content:       "# Version 2 from first request",
			WordCount:     5,
			CreatedAt:     fixedTime,
		})
		require.NoError(t, err)

		originalProvider := service.nowProvider
		service.nowProvider = newMockNowProvider(fixedTime)

		versionFile := filepath.Join(versionDir, "2-"+strconv.FormatInt(fixedTime.Unix(), 10)+".md")
		require.NoError(t, os.WriteFile(versionFile, []byte("# Version 2 from second request"), 0o600))

		err2 := service.db.InsertPlanVersion(ctx, dto.InsertPlanVersionParams{
			PlanID:        plan.ID,
			VersionNumber: 2,
			FilePath:      versionFile,
			Content:       "# Version 2 from second request",
			WordCount:     5,
			CreatedAt:     fixedTime,
		})
		require.Error(t, err2)

		os.Remove(versionFile)

		_, err = os.Stat(versionFile)
		require.True(t, os.IsNotExist(err), "orphaned file should have been deleted")

		count, err := service.GetVersionCount(ctx, "concurrent-test.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, int64(2), count, "should have 2 versions (first request succeeded, second was cleaned up)")

		service.nowProvider = originalProvider
	})
}

func TestTagOperations(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			service, _, _, cleanup := b.setupFn(t)
			defer cleanup()
			testTagOperations(t, service, b.setupFn)
		})
	}
}

func testTagOperations(t *testing.T, service *Service, setup serviceSetupFn) {
	t.Helper()
	ctx := context.Background()

	t.Run("CreateTag with name only", func(t *testing.T) {
		tag, err := service.CreateTag(ctx, "backend", nil, nil)
		require.NoError(t, err)
		require.Equal(t, "backend", tag.Name)
		require.Nil(t, tag.Description)
		require.Nil(t, tag.Color)
	})

	t.Run("CreateTag with name description and color", func(t *testing.T) {
		desc := "Frontend development tasks"
		color := "#FF5733"
		tag, err := service.CreateTag(ctx, "frontend", &desc, &color)
		require.NoError(t, err)
		require.Equal(t, "frontend", tag.Name)
		require.NotNil(t, tag.Description)
		require.Equal(t, desc, *tag.Description)
		require.NotNil(t, tag.Color)
		require.Equal(t, color, *tag.Color)
	})

	t.Run("CreateTag normalizes tag name", func(t *testing.T) {
		tag, err := service.CreateTag(ctx, "  Database  ", nil, nil)
		require.NoError(t, err)
		require.Equal(t, "database", tag.Name)
	})

	t.Run("CreateTag fails with empty tag name", func(t *testing.T) {
		_, err := service.CreateTag(ctx, "   ", nil, nil)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid tag name")
	})

	t.Run("GetAllTags returns empty list when no tags", func(t *testing.T) {
		freshService, _, _, cleanup := setup(t)
		defer cleanup()

		tags, err := freshService.GetAllTags(ctx)
		require.NoError(t, err)
		require.Empty(t, tags)
	})

	t.Run("GetAllTags returns all tags sorted by name", func(t *testing.T) {
		freshService, _, _, cleanup := setup(t)
		defer cleanup()

		_, err := freshService.CreateTag(ctx, "zebra", nil, nil)
		require.NoError(t, err)
		_, err = freshService.CreateTag(ctx, "alpha", nil, nil)
		require.NoError(t, err)
		_, err = freshService.CreateTag(ctx, "charlie", nil, nil)
		require.NoError(t, err)

		tags, err := freshService.GetAllTags(ctx)
		require.NoError(t, err)
		require.Len(t, tags, 3)
		require.Equal(t, "alpha", tags[0].Name)
		require.Equal(t, "charlie", tags[1].Name)
		require.Equal(t, "zebra", tags[2].Name)
	})

	t.Run("DeleteTag removes tag by ID", func(t *testing.T) {
		freshService, _, _, cleanup := setup(t)
		defer cleanup()

		tag, err := freshService.CreateTag(ctx, "temporary", nil, nil)
		require.NoError(t, err)

		err = freshService.DeleteTag(ctx, tag.ID)
		require.NoError(t, err)

		tags, err := freshService.GetAllTags(ctx)
		require.NoError(t, err)
		require.Empty(t, tags)
	})
}

func TestPlanTagRelationship(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			service, sourcePlansDir, _, cleanup := b.setupFn(t)
			defer cleanup()
			testPlanTagRelationship(t, service, sourcePlansDir)
		})
	}
}

func testPlanTagRelationship(t *testing.T, service *Service, sourcePlansDir string) {
	t.Helper()
	ctx := context.Background()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)

	t.Run("GetPlanTags returns empty list for plan with no tags", func(t *testing.T) {
		tags, err := service.GetPlanTags(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Empty(t, tags)
	})

	t.Run("SetPlanTags creates new tags and associates with plan", func(t *testing.T) {
		err := service.SetPlanTags(ctx, "test-plan.md", sourcePlansDir, []string{"api", "database"})
		require.NoError(t, err)

		tags, err := service.GetPlanTags(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Len(t, tags, 2)

		tagNames := []string{tags[0].Name, tags[1].Name}
		sort.Strings(tagNames)
		require.Equal(t, tagNames[0], "api")
		require.Equal(t, tagNames[1], "database")
	})

	t.Run("SetPlanTags normalizes tag names", func(t *testing.T) {
		createTestPlanFile(t, sourcePlansDir, "normalize-test.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SetPlanTags(ctx, "normalize-test.md", sourcePlansDir, []string{"  Frontend  ", "BACKEND"})
		require.NoError(t, err)

		tags, err := service.GetPlanTags(ctx, "normalize-test.md", sourcePlansDir)
		require.NoError(t, err)
		require.Len(t, tags, 2)

		tagNames := []string{tags[0].Name, tags[1].Name}
		require.Contains(t, tagNames, "frontend")
		require.Contains(t, tagNames, "backend")
	})

	t.Run("SetPlanTags replaces existing tags", func(t *testing.T) {
		createTestPlanFile(t, sourcePlansDir, "replace-test.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SetPlanTags(ctx, "replace-test.md", sourcePlansDir, []string{"old-tag"})
		require.NoError(t, err)

		err = service.SetPlanTags(ctx, "replace-test.md", sourcePlansDir, []string{"new-tag1", "new-tag2"})
		require.NoError(t, err)

		tags, err := service.GetPlanTags(ctx, "replace-test.md", sourcePlansDir)
		require.NoError(t, err)
		require.Len(t, tags, 2)

		tagNames := []string{tags[0].Name, tags[1].Name}
		sort.Strings(tagNames)
		require.Equal(t, tagNames[0], "new-tag1")
		require.Equal(t, tagNames[1], "new-tag2")
		require.NotContains(t, tagNames, "old-tag")
	})

	t.Run("SetPlanTags reuses existing tags", func(t *testing.T) {
		createTestPlanFile(t, sourcePlansDir, "plan1.md", sampleMarkdown)
		createTestPlanFile(t, sourcePlansDir, "plan2.md", sampleMarkdownUpdated)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SetPlanTags(ctx, "plan1.md", sourcePlansDir, []string{"shared-tag"})
		require.NoError(t, err)

		allTagsBefore, err := service.GetAllTags(ctx)
		require.NoError(t, err)
		initialTagCount := len(allTagsBefore)

		err = service.SetPlanTags(ctx, "plan2.md", sourcePlansDir, []string{"shared-tag"})
		require.NoError(t, err)

		allTagsAfter, err := service.GetAllTags(ctx)
		require.NoError(t, err)
		require.Equal(t, initialTagCount, len(allTagsAfter))

		tags1, err := service.GetPlanTags(ctx, "plan1.md", sourcePlansDir)
		require.NoError(t, err)
		tags2, err := service.GetPlanTags(ctx, "plan2.md", sourcePlansDir)
		require.NoError(t, err)

		require.Equal(t, tags1[0].ID, tags2[0].ID)
	})

	t.Run("SetPlanTags fails for non-existent plan", func(t *testing.T) {
		err := service.SetPlanTags(ctx, "non-existent.md", sourcePlansDir, []string{"tag"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get plan")
	})

	t.Run("GetPlanTags fails for non-existent plan", func(t *testing.T) {
		_, err := service.GetPlanTags(ctx, "non-existent.md", sourcePlansDir)
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to get plan")
	})

	t.Run("DeleteTag removes tag from all plans", func(t *testing.T) {
		createTestPlanFile(t, sourcePlansDir, "delete-test.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		tag, err := service.CreateTag(ctx, "to-delete", nil, nil)
		require.NoError(t, err)

		err = service.SetPlanTags(ctx, "delete-test.md", sourcePlansDir, []string{"to-delete", "keep-tag"})
		require.NoError(t, err)

		err = service.DeleteTag(ctx, tag.ID)
		require.NoError(t, err)

		tags, err := service.GetPlanTags(ctx, "delete-test.md", sourcePlansDir)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		require.Equal(t, "keep-tag", tags[0].Name)
	})

	t.Run("SetPlanTags with empty list removes all tags", func(t *testing.T) {
		createTestPlanFile(t, sourcePlansDir, "clear-test.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SetPlanTags(ctx, "clear-test.md", sourcePlansDir, []string{"tag1", "tag2"})
		require.NoError(t, err)

		err = service.SetPlanTags(ctx, "clear-test.md", sourcePlansDir, []string{})
		require.NoError(t, err)

		tags, err := service.GetPlanTags(ctx, "clear-test.md", sourcePlansDir)
		require.NoError(t, err)
		require.Empty(t, tags)
	})

	t.Run("Multiple plans can have different tags", func(t *testing.T) {
		createTestPlanFile(t, sourcePlansDir, "backend-plan.md", sampleMarkdown)
		createTestPlanFile(t, sourcePlansDir, "frontend-plan.md", sampleMarkdownUpdated)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SetPlanTags(ctx, "backend-plan.md", sourcePlansDir, []string{"api", "database"})
		require.NoError(t, err)

		err = service.SetPlanTags(ctx, "frontend-plan.md", sourcePlansDir, []string{"ui", "react"})
		require.NoError(t, err)

		backendTags, err := service.GetPlanTags(ctx, "backend-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Len(t, backendTags, 2)

		frontendTags, err := service.GetPlanTags(ctx, "frontend-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Len(t, frontendTags, 2)

		backendTagNames := []string{backendTags[0].Name, backendTags[1].Name}
		sort.Strings(backendTagNames)
		frontendTagNames := []string{frontendTags[0].Name, frontendTags[1].Name}
		sort.Strings(frontendTagNames)

		require.Equal(t, backendTagNames[0], "api")
		require.Equal(t, backendTagNames[1], "database")
		require.Equal(t, frontendTagNames[0], "react")
		require.Equal(t, frontendTagNames[1], "ui")
	})
}

func TestSearchOverSetting(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testSearchOverSetting(t, b.setupFn)
		})
	}
}

func testSearchOverSetting(t *testing.T, setup serviceSetupFn) {
	t.Helper()
	service, sourcePlansDir, _, cleanup := setup(t)
	defer cleanup()
	ctx := context.Background()

	// plan1: unique term in heading (becomes `title` column); "AlphaTitleUnique" also appears in the
	// full file text stored in `content`, so content searches can find it too.
	// plan2: unique term only in the body, NOT in the heading, so `title` column won't contain it.
	createTestPlanFile(t, sourcePlansDir, "plan1.md", "# AlphaTitleUnique\n\nGeneric body text here.")
	createTestPlanFile(t, sourcePlansDir, "plan2.md", "# Generic Title\n\nBetaBodyUnique lives here.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)

	setScope := func(scope SearchField) {
		err := service.SetSetting(ctx, SettingSearchOver, SettingValues{StringValue: new(string(scope))})
		require.NoError(t, err)
	}

	t.Run("SearchOverAll finds by title", func(t *testing.T) {
		setScope(SearchOverAll)
		plans, err := service.SearchPlansWithReadingTime(ctx, "AlphaTitleUnique")
		require.NoError(t, err)
		require.Len(t, plans, 1)
		require.Equal(t, "plan1.md", plans[0].FileName)
	})

	t.Run("SearchOverAll finds by body content", func(t *testing.T) {
		setScope(SearchOverAll)
		plans, err := service.SearchPlansWithReadingTime(ctx, "BetaBodyUnique")
		require.NoError(t, err)
		require.Len(t, plans, 1)
		require.Equal(t, "plan2.md", plans[0].FileName)
	})

	t.Run("SearchOverPlanName finds by title", func(t *testing.T) {
		setScope(SearchOverPlanName)
		plans, err := service.SearchPlansWithReadingTime(ctx, "AlphaTitleUnique")
		require.NoError(t, err)
		require.Len(t, plans, 1)
		require.Equal(t, "plan1.md", plans[0].FileName)
	})

	t.Run("SearchOverPlanName does not find body-only term", func(t *testing.T) {
		setScope(SearchOverPlanName)
		// "BetaBodyUnique" is only in plan2's body, not in any heading
		plans, err := service.SearchPlansWithReadingTime(ctx, "BetaBodyUnique")
		require.NoError(t, err)
		require.Empty(t, plans)
	})

	t.Run("SearchOverContent finds body term", func(t *testing.T) {
		setScope(SearchOverContent)
		plans, err := service.SearchPlansWithReadingTime(ctx, "BetaBodyUnique")
		require.NoError(t, err)
		require.Len(t, plans, 1)
		require.Equal(t, "plan2.md", plans[0].FileName)
	})

	t.Run("SearchOverContent finds term that appears in full file text", func(t *testing.T) {
		setScope(SearchOverContent)
		// "AlphaTitleUnique" is in plan1's heading, which is also stored in the content column
		plans, err := service.SearchPlansWithReadingTime(ctx, "AlphaTitleUnique")
		require.NoError(t, err)
		require.Len(t, plans, 1)
		require.Equal(t, "plan1.md", plans[0].FileName)
	})
}

// ---- Multi-source setup helpers ----

type multiSourceSetup struct {
	sourceDir1 string
	sourceDir2 string
	viewerDir  string
}

// newMultiSourceTest builds a two-source-directory service for a given backend.
// The source directories, viewer directory, and repository all live under a
// single temp dir that is removed by the returned cleanup function.
func newMultiSourceTest(t *testing.T, b backendSetup) (*Service, multiSourceSetup, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp(os.TempDir(), "claude-viewer-multi-*")
	require.NoError(t, err)

	sourceDir1 := filepath.Join(tempDir, "source-a")
	sourceDir2 := filepath.Join(tempDir, "source-b")
	require.NoError(t, os.MkdirAll(sourceDir1, 0o755))
	require.NoError(t, os.MkdirAll(sourceDir2, 0o755))

	repo, repoCleanup := b.repoFn(t, tempDir)

	svc, viewerDir := newServiceFromRepo(t, tempDir, repo, []config.SyncDir{
		{Path: sourceDir1, Label: "dir-a"},
		{Path: sourceDir2, Label: "dir-b"},
	})

	cleanup := func() {
		if repoCleanup != nil {
			repoCleanup()
		}
		os.RemoveAll(tempDir)
	}
	return svc, multiSourceSetup{sourceDir1: sourceDir1, sourceDir2: sourceDir2, viewerDir: viewerDir}, cleanup
}

func TestLabelHelpers(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testLabelHelpers(t, b)
		})
	}
}

func testLabelHelpers(t *testing.T, b backendSetup) {
	t.Helper()
	svc, dirs, cleanup := newMultiSourceTest(t, b)
	defer cleanup()

	t.Run("LabelForSource known path returns configured label", func(t *testing.T) {
		require.Equal(t, "dir-a", svc.LabelForSource(dirs.sourceDir1))
		require.Equal(t, "dir-b", svc.LabelForSource(dirs.sourceDir2))
	})

	t.Run("LabelForSource unknown path falls back to base dir name", func(t *testing.T) {
		unknownPath := filepath.Join(dirs.viewerDir, "unknown", "nested")
		require.Equal(t, "nested", svc.LabelForSource(unknownPath))
	})

	t.Run("SourcePathForLabel known label returns path", func(t *testing.T) {
		require.Equal(t, dirs.sourceDir1, svc.SourcePathForLabel("dir-a"))
		require.Equal(t, dirs.sourceDir2, svc.SourcePathForLabel("dir-b"))
	})

	t.Run("SourcePathForLabel unknown label returns empty string", func(t *testing.T) {
		require.Equal(t, "", svc.SourcePathForLabel("nonexistent"))
	})
}

func TestMultiDirSyncPlans(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testMultiDirSyncPlans(t, b)
		})
	}
}

func testMultiDirSyncPlans(t *testing.T, b backendSetup) {
	t.Helper()
	ctx := context.Background()

	t.Run("plans from both dirs are indexed", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "plan-a.md", "# Plan A\n\nFrom dir a.")
		createTestPlanFile(t, dirs.sourceDir2, "plan-b.md", "# Plan B\n\nFrom dir b.")

		count, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		plans, err := svc.ListAllPlansWithReadingTime(ctx)
		require.NoError(t, err)
		require.Len(t, plans, 2)
	})

	t.Run("same filename in two dirs no collision", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "plan.md", "# Plan from A\n\nContent A.")
		createTestPlanFile(t, dirs.sourceDir2, "plan.md", "# Plan from B\n\nContent B.")

		count, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, count)

		planA, err := svc.GetPlanByFileName(ctx, "plan.md", dirs.sourceDir1)
		require.NoError(t, err)
		require.Equal(t, "Plan from A", planA.Title)

		planB, err := svc.GetPlanByFileName(ctx, "plan.md", dirs.sourceDir2)
		require.NoError(t, err)
		require.Equal(t, "Plan from B", planB.Title)
	})

	t.Run("sync_source stored correctly on each plan", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "mine.md", "# Mine\n\nContent.")
		createTestPlanFile(t, dirs.sourceDir2, "theirs.md", "# Theirs\n\nContent.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		planA, err := svc.GetPlanByFileName(ctx, "mine.md", dirs.sourceDir1)
		require.NoError(t, err)
		require.Equal(t, dirs.sourceDir1, planA.SyncSource)

		planB, err := svc.GetPlanByFileName(ctx, "theirs.md", dirs.sourceDir2)
		require.NoError(t, err)
		require.Equal(t, dirs.sourceDir2, planB.SyncSource)
	})

	t.Run("sync_label resolved on plan detail", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "label-test.md", "# Label Test\n\nContent.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		detail, err := svc.GetPlanDetailByFileName(ctx, "label-test.md", dirs.sourceDir1)
		require.NoError(t, err)
		require.Equal(t, "dir-a", detail.SyncLabel)
	})

	t.Run("viewer copies land in distinct id-keyed dirs, no cross-source collision", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "plan.md", "# Plan A\n\nContent A.")
		createTestPlanFile(t, dirs.sourceDir2, "plan.md", "# Plan B\n\nContent B.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		planA, err := svc.GetPlanByFileName(ctx, "plan.md", dirs.sourceDir1)
		require.NoError(t, err)
		planB, err := svc.GetPlanByFileName(ctx, "plan.md", dirs.sourceDir2)
		require.NoError(t, err)

		require.NotEqual(t, planA.ID, planB.ID)
		require.NotEqual(t, planA.FilePath, planB.FilePath, "mirror files for same-named plans from different sources must not collide")
		require.Equal(t, filepath.Join(dirs.viewerDir, "plans", strconv.FormatInt(planA.ID, 10), "plan.md"), planA.FilePath)
		require.Equal(t, filepath.Join(dirs.viewerDir, "plans", strconv.FormatInt(planB.ID, 10), "plan.md"), planB.FilePath)

		dataA, err := os.ReadFile(planA.FilePath)
		require.NoError(t, err, "viewer copy missing for plan A")
		require.Contains(t, string(dataA), "Content A.")

		dataB, err := os.ReadFile(planB.FilePath)
		require.NoError(t, err, "viewer copy missing for plan B")
		require.Contains(t, string(dataB), "Content B.")
	})

	t.Run("version histories for same-named plans from different sources do not collide", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "shared.md", "# Shared A\n\nOriginal A.")
		createTestPlanFile(t, dirs.sourceDir2, "shared.md", "# Shared B\n\nOriginal B.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		require.NoError(t, svc.SavePlanVersion(ctx, "shared.md", dirs.sourceDir1, "# Shared A\n\nVersion 1 of A."))
		require.NoError(t, svc.SavePlanVersion(ctx, "shared.md", dirs.sourceDir2, "# Shared B\n\nVersion 1 of B."))

		planA, err := svc.GetPlanByFileName(ctx, "shared.md", dirs.sourceDir1)
		require.NoError(t, err)
		planB, err := svc.GetPlanByFileName(ctx, "shared.md", dirs.sourceDir2)
		require.NoError(t, err)

		versionsA, err := svc.GetPlanVersionHistory(ctx, "shared.md", dirs.sourceDir1, 0, 10)
		require.NoError(t, err)
		versionsB, err := svc.GetPlanVersionHistory(ctx, "shared.md", dirs.sourceDir2, 0, 10)
		require.NoError(t, err)
		require.Len(t, versionsA, 1)
		require.Len(t, versionsB, 1)

		// Distinct, id-scoped versions directories — no shared "versions/shared.md/" bucket.
		require.NotEqual(t, versionsA[0].FilePath, versionsB[0].FilePath)
		require.Equal(t, svc.versionsDirFor(planA.ID), filepath.Dir(versionsA[0].FilePath))
		require.Equal(t, svc.versionsDirFor(planB.ID), filepath.Dir(versionsB[0].FilePath))

		// Each version file holds only its own plan's content — no cross-contamination.
		require.Equal(t, "# Shared A\n\nVersion 1 of A.", versionsA[0].Content)
		require.Equal(t, "# Shared B\n\nVersion 1 of B.", versionsB[0].Content)

		// Deleting plan A's version history must not touch plan B's.
		require.NoError(t, svc.DeletePlan(ctx, "shared.md", dirs.sourceDir1))
		_, err = os.Stat(svc.versionsDirFor(planB.ID))
		require.NoError(t, err, "deleting plan A must not remove plan B's versions directory")
		remainingB, err := svc.GetPlanVersionHistory(ctx, "shared.md", dirs.sourceDir2, 0, 10)
		require.NoError(t, err)
		require.Len(t, remainingB, 1)
	})

	t.Run("second sync is idempotent across dirs", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "idem.md", "# Idempotent\n\nContent.")
		createTestPlanFile(t, dirs.sourceDir2, "idem2.md", "# Idempotent 2\n\nContent.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		count, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("one dir empty count reflects only non-empty dir", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "only-a.md", "# Only A\n\nContent.")
		// sourceDir2 intentionally empty

		count, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})
}

func TestMultiDirRSyncPlans(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testMultiDirRSyncPlans(t, b)
		})
	}
}

func testMultiDirRSyncPlans(t *testing.T, b backendSetup) {
	t.Helper()
	ctx := context.Background()

	t.Run("restores plan to its original source dir", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "restore-me.md", "# Restore Me\n\nContent.")
		createTestPlanFile(t, dirs.sourceDir2, "stay.md", "# Stay\n\nContent.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		require.NoError(t, os.Remove(filepath.Join(dirs.sourceDir1, "restore-me.md")))

		count, err := svc.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		_, err = os.Stat(filepath.Join(dirs.sourceDir1, "restore-me.md"))
		require.NoError(t, err, "plan should be restored to its original source dir")
	})

	t.Run("does not restore plan to wrong source dir", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir2, "only-b.md", "# Only B\n\nContent.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		require.NoError(t, os.Remove(filepath.Join(dirs.sourceDir2, "only-b.md")))

		_, err = svc.RSyncPlans(ctx)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(dirs.sourceDir1, "only-b.md"))
		require.ErrorIs(t, err, os.ErrNotExist, "plan must not be placed in wrong source dir")
	})
}

func TestMultiDirUpdatePlan(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testMultiDirUpdatePlan(t, b)
		})
	}
}

func testMultiDirUpdatePlan(t *testing.T, b backendSetup) {
	t.Helper()
	ctx := context.Background()

	t.Run("update writes to correct source path", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "target.md", "# Original\n\nOriginal content.")
		createTestPlanFile(t, dirs.sourceDir2, "unrelated.md", "# Unrelated\n\nStay unchanged.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := svc.GetPlanByFileName(ctx, "target.md", dirs.sourceDir1)
		require.NoError(t, err)

		result, err := svc.UpdatePlan(ctx, UpdatePlanRequest{
			FileName:         "target.md",
			SyncSource:       dirs.sourceDir1,
			NewContent:       "# Updated\n\nNew content.",
			LastModifiedTime: plan.ModifiedAt,
		})
		require.NoError(t, err)
		require.True(t, result.Success)

		content, err := os.ReadFile(filepath.Join(dirs.sourceDir1, "target.md"))
		require.NoError(t, err)
		require.Equal(t, "# Updated\n\nNew content.", string(content))
	})

	t.Run("update in dir-a does not affect same filename in dir-b", func(t *testing.T) {
		svc, dirs, cleanup := newMultiSourceTest(t, b)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "shared-name.md", "# A Version\n\nContent from A.")
		createTestPlanFile(t, dirs.sourceDir2, "shared-name.md", "# B Version\n\nContent from B.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		planA, err := svc.GetPlanByFileName(ctx, "shared-name.md", dirs.sourceDir1)
		require.NoError(t, err)

		result, err := svc.UpdatePlan(ctx, UpdatePlanRequest{
			FileName:         "shared-name.md",
			SyncSource:       dirs.sourceDir1,
			NewContent:       "# A Updated\n\nUpdated content from A.",
			LastModifiedTime: planA.ModifiedAt,
		})
		require.NoError(t, err)
		require.True(t, result.Success)

		contentB, err := os.ReadFile(filepath.Join(dirs.sourceDir2, "shared-name.md"))
		require.NoError(t, err)
		require.Equal(t, "# B Version\n\nContent from B.", string(contentB))

		planB, err := svc.GetPlanByFileName(ctx, "shared-name.md", dirs.sourceDir2)
		require.NoError(t, err)
		require.Equal(t, "B Version", planB.Title)
	})
}

// setupMockConnector creates a gomock MockConnector with standard expectations.
func setupMockConnector(ctrl *gomock.Controller, name, displayName string) *connectors_test.MockConnector {
	mock := connectors_test.NewMockConnector(ctrl)
	mock.EXPECT().Name().Return(name).AnyTimes()
	mock.EXPECT().DisplayName().Return(displayName).AnyTimes()
	mock.EXPECT().RequiredSettings().Return([]connectors.SettingDefinition{
		{Key: "api_token", DisplayName: "API Token", Required: true, Sensitive: true},
		{Key: "channel_id", DisplayName: "Channel ID", Required: true, Sensitive: false},
	}).AnyTimes()
	return mock
}

// --- Comment tests ---

func TestCommentOperations(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			service, sourcePlansDir, _, cleanup := b.setupFn(t)
			defer cleanup()
			testCommentOperations(t, service, sourcePlansDir, b.setupFn)
		})
	}
}

func testCommentOperations(t *testing.T, service *Service, sourcePlansDir string, setup serviceSetupFn) {
	t.Helper()
	ctx := context.Background()

	// Seed a plan for all sub-tests.
	createTestPlanFile(t, sourcePlansDir, "commented.md", sampleMarkdown)
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)

	fileName := "commented.md"
	syncSource := sourcePlansDir

	t.Run("AddComment stores and retrieves comment", func(t *testing.T) {
		c, err := service.AddComment(ctx, fileName, syncSource, "This is my first note")
		require.NoError(t, err)
		require.Equal(t, "This is my first note", c.Content)
		require.False(t, c.CreatedAt.IsZero())

		comments, err := service.GetPlanComments(ctx, fileName, syncSource)
		require.NoError(t, err)
		require.Len(t, comments, 1)
		require.Equal(t, "This is my first note", comments[0].Content)
	})

	t.Run("GetPlanComments returns comments in ASC order", func(t *testing.T) {
		freshService, freshDir, _, cleanup := setup(t)
		defer cleanup()

		createTestPlanFile(t, freshDir, "ordered.md", sampleMarkdown)
		_, err := freshService.SyncPlans(ctx)
		require.NoError(t, err)

		now := time.Now()
		freshService.nowProvider = newMockNowProvider(now)
		_, err = freshService.AddComment(ctx, "ordered.md", freshDir, "first")
		require.NoError(t, err)

		freshService.nowProvider = newMockNowProvider(now.Add(time.Second))
		_, err = freshService.AddComment(ctx, "ordered.md", freshDir, "second")
		require.NoError(t, err)

		comments, err := freshService.GetPlanComments(ctx, "ordered.md", freshDir)
		require.NoError(t, err)
		require.Len(t, comments, 2)
		require.Equal(t, "first", comments[0].Content)
		require.Equal(t, "second", comments[1].Content)
		require.True(t, comments[0].CreatedAt.Before(comments[1].CreatedAt) || comments[0].CreatedAt.Equal(comments[1].CreatedAt))
	})

	t.Run("DeleteComment removes comment", func(t *testing.T) {
		freshService, freshDir, _, cleanup := setup(t)
		defer cleanup()

		createTestPlanFile(t, freshDir, "deleteme.md", sampleMarkdown)
		_, err := freshService.SyncPlans(ctx)
		require.NoError(t, err)

		c, err := freshService.AddComment(ctx, "deleteme.md", freshDir, "to be deleted")
		require.NoError(t, err)

		err = freshService.DeleteComment(ctx, c.ID)
		require.NoError(t, err)

		comments, err := freshService.GetPlanComments(ctx, "deleteme.md", freshDir)
		require.NoError(t, err)
		require.Empty(t, comments)
	})

	t.Run("CommentCount appears in plan summary", func(t *testing.T) {
		freshService, freshDir, _, cleanup := setup(t)
		defer cleanup()

		createTestPlanFile(t, freshDir, "planA.md", sampleMarkdown)
		createTestPlanFile(t, freshDir, "planB.md", sampleMarkdownUpdated)
		_, err := freshService.SyncPlans(ctx)
		require.NoError(t, err)

		_, err = freshService.AddComment(ctx, "planA.md", freshDir, "note one")
		require.NoError(t, err)
		_, err = freshService.AddComment(ctx, "planA.md", freshDir, "note two")
		require.NoError(t, err)

		plans, err := freshService.ListAllPlansWithReadingTime(ctx)
		require.NoError(t, err)

		counts := make(map[string]int)
		for _, p := range plans {
			counts[p.FileName] = p.CommentCount
		}
		require.Equal(t, 2, counts["planA.md"])
		require.Equal(t, 0, counts["planB.md"])
	})

	t.Run("Comments survive SyncPlans re-run", func(t *testing.T) {
		freshService, freshDir, _, cleanup := setup(t)
		defer cleanup()

		createTestPlanFile(t, freshDir, "survive.md", sampleMarkdown)
		_, err := freshService.SyncPlans(ctx)
		require.NoError(t, err)

		_, err = freshService.AddComment(ctx, "survive.md", freshDir, "survives re-sync")
		require.NoError(t, err)

		// Touch the file to force a re-sync.
		path := filepath.Join(freshDir, "survive.md")
		require.NoError(t, os.WriteFile(path, []byte(sampleMarkdown+" extra"), 0o600))

		_, err = freshService.SyncPlans(ctx)
		require.NoError(t, err)

		comments, err := freshService.GetPlanComments(ctx, "survive.md", freshDir)
		require.NoError(t, err)
		require.Len(t, comments, 1)
		require.Equal(t, "survives re-sync", comments[0].Content)
	})
}

func TestDeletePlanOperations(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			service, sourcePlansDir, _, cleanup := b.setupFn(t)
			defer cleanup()
			testDeletePlan(t, service, sourcePlansDir)
		})
	}
}

func testDeletePlan(t *testing.T, service *Service, sourcePlansDir string) {
	t.Helper()
	ctx := context.Background()

	t.Run("DeletePlan removes DB row, files, and versions", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "delete-me.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Seed a version (DB row + on-disk version file) and a comment.
		require.NoError(t, service.SavePlanVersion(ctx, "delete-me.md", sourcePlansDir, sampleMarkdown))
		_, err = service.AddComment(ctx, "delete-me.md", sourcePlansDir, "a comment")
		require.NoError(t, err)

		// Pre-conditions: everything present.
		plan, err := service.GetPlanByFileName(ctx, "delete-me.md", sourcePlansDir)
		require.NoError(t, err)
		mirrorPath := plan.FilePath
		versionDir := service.versionsDirFor(plan.ID)
		require.Equal(t, service.mirrorPathFor(plan.ID, "delete-me.md"), plan.FilePath)
		require.FileExists(t, testFile)
		require.FileExists(t, mirrorPath)
		require.DirExists(t, versionDir)

		commentCounts, err := service.DB().GetPlanCommentCounts(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, commentCounts[plan.ID])

		require.NoError(t, service.DeletePlan(ctx, "delete-me.md", sourcePlansDir))

		// DB row gone.
		_, err = service.GetPlanByFileName(ctx, "delete-me.md", sourcePlansDir)
		require.Error(t, err)

		// Source, mirror, and version files gone.
		require.NoFileExists(t, testFile)
		require.NoFileExists(t, mirrorPath)
		require.NoDirExists(t, versionDir)

		// Comments gone (would orphan on SQLite without the explicit delete,
		// since FK cascade is not enforced at runtime).
		commentCounts, err = service.DB().GetPlanCommentCounts(ctx)
		require.NoError(t, err)
		require.NotContains(t, commentCounts, plan.ID)
	})

	t.Run("file delete failure aborts before DB mutation (files-first ordering)", func(t *testing.T) {
		// A non-empty directory at the source path makes os.Remove fail with a
		// non-IsNotExist error, exercising the files-first abort path.
		const fileName = "stubborn.md"
		badPath := filepath.Join(sourcePlansDir, fileName)
		require.NoError(t, os.MkdirAll(badPath, 0o750))
		require.NoError(t, os.WriteFile(filepath.Join(badPath, "child"), []byte("x"), 0o600))

		now := service.nowProvider.Now()
		_, err := service.db.InsertPlan(ctx, dto.InsertPlanParams{
			FileName:   fileName,
			SyncSource: sourcePlansDir,
			Title:      "Stubborn",
			Content:    sampleMarkdown,
			CreatedAt:  now,
			ModifiedAt: now,
			IndexedAt:  now,
			FileSize:   1,
			WordCount:  1,
		}, func(int64) (string, error) { return badPath, nil })
		require.NoError(t, err)

		err = service.DeletePlan(ctx, fileName, sourcePlansDir)
		require.Error(t, err)

		// DB row must survive the failed delete (files-first ordering).
		_, err = service.GetPlanByFileName(ctx, fileName, sourcePlansDir)
		require.NoError(t, err)
	})
}

func TestRenamePlanFileOperations(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testRenamePlanFile(t, b.setupFn)
		})
	}
}

func testRenamePlanFile(t *testing.T, setup serviceSetupFn) {
	t.Helper()

	t.Run("moves files and preserves title, tags, comments, and versions", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "old-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		detail, err := service.GetPlanDetailByFileName(ctx, "old-plan.md", sourcePlansDir)
		require.NoError(t, err)
		originalTitle := detail.Title
		originalContent := detail.Content
		versionsDir := service.versionsDirFor(detail.ID)

		// Seed a tag, a comment, and a saved version.
		_, err = service.CreateTag(ctx, "refactor", nil, nil)
		require.NoError(t, err)
		require.NoError(t, service.SetPlanTags(ctx, "old-plan.md", sourcePlansDir, []string{"refactor"}))
		_, err = service.AddComment(ctx, "old-plan.md", sourcePlansDir, "a comment")
		require.NoError(t, err)
		require.NoError(t, service.SavePlanVersion(ctx, "old-plan.md", sourcePlansDir, sampleMarkdown))

		require.NoError(t, service.RenamePlanFile(ctx, "old-plan.md", sourcePlansDir, "new-plan.md"))

		// Old paths gone, new paths present with identical mirror content, same directory
		// (only the file name inside the plan's id-scoped dir changes).
		require.NoFileExists(t, filepath.Join(sourcePlansDir, "old-plan.md"))
		require.FileExists(t, filepath.Join(sourcePlansDir, "new-plan.md"))
		require.NoFileExists(t, detail.FilePath)
		newMirror := filepath.Join(filepath.Dir(detail.FilePath), "new-plan.md")
		mirrorBytes, err := os.ReadFile(newMirror)
		require.NoError(t, err)
		require.Equal(t, sampleMarkdown, string(mirrorBytes))

		// Versions dir is untouched by the rename — it's keyed by plan id, not file name.
		require.DirExists(t, versionsDir)

		// Old DB row gone; new row present with unchanged title/content and new path.
		_, err = service.GetPlanByFileName(ctx, "old-plan.md", sourcePlansDir)
		require.Error(t, err)
		renamed, err := service.GetPlanDetailByFileName(ctx, "new-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, originalTitle, renamed.Title)
		require.Equal(t, originalContent, renamed.Content)
		require.Equal(t, newMirror, renamed.FilePath)

		// Associations still resolve under the new name.
		tags, err := service.GetPlanTags(ctx, "new-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Len(t, tags, 1)
		require.Equal(t, "refactor", tags[0].Name)

		comments, err := service.GetPlanComments(ctx, "new-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Len(t, comments, 1)

		versions, err := service.GetPlanVersionHistory(ctx, "new-plan.md", sourcePlansDir, 0, 10)
		require.NoError(t, err)
		require.NotEmpty(t, versions)
		for _, v := range versions {
			// Version paths are unaffected by the rename — still under the plan's
			// id-scoped versions dir, which never depended on the file name.
			require.Equal(t, versionsDir, filepath.Dir(v.FilePath))
			require.FileExists(t, v.FilePath)
		}
	})

	t.Run("rename does not move or rewrite the versions directory", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "ver.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Save several versions so a multi-row scenario is exercised.
		const versionCount = 3
		for i := 0; i < versionCount; i++ {
			require.NoError(t, service.SavePlanVersion(ctx, "ver.md", sourcePlansDir,
				fmt.Sprintf("# Version %d\n\nbody %d", i+1, i+1)))
		}

		before, err := service.GetPlanVersionHistory(ctx, "ver.md", sourcePlansDir, 0, 100)
		require.NoError(t, err)
		require.Len(t, before, versionCount)

		// Snapshot each version's exact path and on-disk content before the rename.
		type versionSnapshot struct {
			path    string
			content []byte
		}
		snapshots := make(map[int64]versionSnapshot, len(before))
		for _, v := range before {
			content, readErr := os.ReadFile(v.FilePath)
			require.NoError(t, readErr)
			snapshots[v.VersionNumber] = versionSnapshot{path: v.FilePath, content: content}
		}

		detail, err := service.GetPlanDetailByFileName(ctx, "ver.md", sourcePlansDir)
		require.NoError(t, err)
		versionsDir := service.versionsDirFor(detail.ID)
		require.NoError(t, service.RenamePlanFile(ctx, "ver.md", sourcePlansDir, "ver-renamed.md"))

		// The versions dir is exactly where it was, still holding the same files.
		require.DirExists(t, versionsDir)
		entries, err := os.ReadDir(versionsDir)
		require.NoError(t, err)
		require.Len(t, entries, versionCount)

		// Every version row's path, and its file's content on disk, is unchanged by the rename.
		after, err := service.GetPlanVersionHistory(ctx, "ver-renamed.md", sourcePlansDir, 0, 100)
		require.NoError(t, err)
		require.Len(t, after, versionCount)
		for _, v := range after {
			orig, ok := snapshots[v.VersionNumber]
			require.True(t, ok)
			require.Equal(t, orig.path, v.FilePath)
			content, readErr := os.ReadFile(v.FilePath)
			require.NoError(t, readErr)
			require.Equal(t, orig.content, content)
		}
	})

	t.Run("appends .md and strips path separators", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "np.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// No .md and a path separator -> normalized to "renamed.md".
		require.NoError(t, service.RenamePlanFile(ctx, "np.md", sourcePlansDir, "sub/renamed"))
		_, err = service.GetPlanByFileName(ctx, "renamed.md", sourcePlansDir)
		require.NoError(t, err)
		require.FileExists(t, filepath.Join(sourcePlansDir, "renamed.md"))

		// An existing .md extension is preserved case-insensitively (not doubled to .MD.md).
		require.NoError(t, service.RenamePlanFile(ctx, "renamed.md", sourcePlansDir, "keepcase.MD"))
		_, err = service.GetPlanByFileName(ctx, "keepcase.MD", sourcePlansDir)
		require.NoError(t, err)
		_, err = service.GetPlanByFileName(ctx, "keepcase.MD.md", sourcePlansDir)
		require.Error(t, err)
	})

	t.Run("rejects renaming to the same name", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "same.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		require.Error(t, service.RenamePlanFile(ctx, "same.md", sourcePlansDir, "same.md"))
	})

	t.Run("rejects when a plan with the target name already exists", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "a.md", sampleMarkdown)
		createTestPlanFile(t, sourcePlansDir, "b.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		require.Error(t, service.RenamePlanFile(ctx, "a.md", sourcePlansDir, "b.md"))

		// Nothing changed: a.md still resolves and still on disk.
		require.FileExists(t, filepath.Join(sourcePlansDir, "a.md"))
		_, err = service.GetPlanByFileName(ctx, "a.md", sourcePlansDir)
		require.NoError(t, err)
	})

	t.Run("rolls back moves and DB when a file move fails", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "rb.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		require.NoError(t, service.SavePlanVersion(ctx, "rb.md", sourcePlansDir, sampleMarkdown))
		detail, err := service.GetPlanDetailByFileName(ctx, "rb.md", sourcePlansDir)
		require.NoError(t, err)
		versionsDir := service.versionsDirFor(detail.ID)

		// Force the mirror move (the second move) to fail by deleting the mirror file
		// before the rename. The source move succeeds first, so this exercises both the
		// LIFO file rollback (undoing the source move) and the DB rollback. Removing an
		// existing file avoids tripping the pre-flight collision checks.
		require.NoError(t, os.Remove(detail.FilePath))

		require.Error(t, service.RenamePlanFile(ctx, "rb.md", sourcePlansDir, "rb-new.md"))

		// The source move was rolled back.
		require.FileExists(t, filepath.Join(sourcePlansDir, "rb.md"))
		require.NoFileExists(t, filepath.Join(sourcePlansDir, "rb-new.md"))
		require.NoFileExists(t, filepath.Join(filepath.Dir(detail.FilePath), "rb-new.md"))

		// DB reverted.
		_, err = service.GetPlanByFileName(ctx, "rb.md", sourcePlansDir)
		require.NoError(t, err)
		_, err = service.GetPlanByFileName(ctx, "rb-new.md", sourcePlansDir)
		require.Error(t, err)

		// Versions dir untouched — it was never part of the rename to begin with.
		require.DirExists(t, versionsDir)
	})

	// The following two subtests drive the repository directly to prove the DB rename
	// and the file-move callback are one atomic unit: the transaction commits only if
	// the callback succeeds, so a failing callback leaves the DB completely unchanged
	// (no separate compensating write).

	t.Run("repository rolls back the DB when the rename callback fails", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "atomic.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		require.NoError(t, service.SavePlanVersion(ctx, "atomic.md", sourcePlansDir, sampleMarkdown))

		before, err := service.GetPlanDetailByFileName(ctx, "atomic.md", sourcePlansDir)
		require.NoError(t, err)
		versionsBefore, err := service.GetPlanVersionHistory(ctx, "atomic.md", sourcePlansDir, 0, 10)
		require.NoError(t, err)
		require.NotEmpty(t, versionsBefore)

		callbackRan := false
		err = service.DB().RenamePlanFile(ctx, dto.RenamePlanFileParams{
			OldFileName: "atomic.md",
			SyncSource:  sourcePlansDir,
			NewFileName: "atomic-renamed.md",
			NewFilePath: filepath.Join(filepath.Dir(before.FilePath), "atomic-renamed.md"),
		}, func() error {
			callbackRan = true
			return fmt.Errorf("callback boom")
		})
		require.ErrorContains(t, err, "callback boom")
		require.True(t, callbackRan, "renameFiles callback must run inside the transaction")

		// Plan row is unchanged: old name still resolves at the same path, new name does not.
		after, err := service.GetPlanDetailByFileName(ctx, "atomic.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, before.FilePath, after.FilePath)
		_, err = service.GetPlanByFileName(ctx, "atomic-renamed.md", sourcePlansDir)
		require.Error(t, err)

		// Versions were never touched by the rename in the first place.
		versionsAfter, err := service.GetPlanVersionHistory(ctx, "atomic.md", sourcePlansDir, 0, 10)
		require.NoError(t, err)
		require.Equal(t, versionsBefore[0].FilePath, versionsAfter[0].FilePath)
	})

	t.Run("repository commits the DB when the rename callback succeeds", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "commit.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		before, err := service.GetPlanDetailByFileName(ctx, "commit.md", sourcePlansDir)
		require.NoError(t, err)

		newMirror := filepath.Join(filepath.Dir(before.FilePath), "commit-ok.md")
		callbackRan := false
		// The callback is a no-op (this test isolates the DB behaviour, not the moves).
		err = service.DB().RenamePlanFile(ctx, dto.RenamePlanFileParams{
			OldFileName: "commit.md",
			SyncSource:  sourcePlansDir,
			NewFileName: "commit-ok.md",
			NewFilePath: newMirror,
		}, func() error {
			callbackRan = true
			return nil
		})
		require.NoError(t, err)
		require.True(t, callbackRan)

		// The rename was committed: new name resolves with the new path, old name is gone.
		renamed, err := service.GetPlanDetailByFileName(ctx, "commit-ok.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, newMirror, renamed.FilePath)
		_, err = service.GetPlanByFileName(ctx, "commit.md", sourcePlansDir)
		require.Error(t, err)
	})
}

func TestCopyToClipboardOperations(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testCopyToClipboard(t, b.setupFn)
		})
	}
}

func testCopyToClipboard(t *testing.T, setup serviceSetupFn) {
	t.Run("writes the text in auto mode by default", func(t *testing.T) {
		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		clip := clipboard_test.NewMockClipboard(ctrl)
		clip.EXPECT().Write("hello world", DefaultClipboardMode).Return(nil)
		service.clipboard = clip

		require.NoError(t, service.CopyToClipboard(ctx, "hello world"))
	})

	t.Run("passes the clipboard_mode setting through to the writer", func(t *testing.T) {
		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		mode := ClipboardModeOSC52
		require.NoError(t, service.SetSetting(ctx, SettingClipboardMode, SettingValues{StringValue: &mode}))

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		clip := clipboard_test.NewMockClipboard(ctrl)
		clip.EXPECT().Write(gomock.Any(), ClipboardModeOSC52).Return(nil)
		service.clipboard = clip

		require.NoError(t, service.CopyToClipboard(ctx, "hello"))
	})

	t.Run("propagates a clipboard writer error", func(t *testing.T) {
		service, _, _, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		ctrl := gomock.NewController(t)
		defer ctrl.Finish()
		clip := clipboard_test.NewMockClipboard(ctrl)
		clip.EXPECT().Write(gomock.Any(), gomock.Any()).Return(fmt.Errorf("no clipboard available"))
		service.clipboard = clip

		err := service.CopyToClipboard(ctx, "hello")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no clipboard available")
	})
}

// --- Storage layout migration tests ---

func TestMigrateStorageLayout(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testMigrateStorageLayout(t, b)
		})
	}
}

func testMigrateStorageLayout(t *testing.T, b backendSetup) {
	t.Helper()
	ctx := context.Background()

	t.Run("moves an old-layout mirror file to the id-keyed path and updates file_path", func(t *testing.T) {
		service, sourcePlansDir, viewerDir, cleanup := b.setupFn(t)
		defer cleanup()

		oldDir := filepath.Join(viewerDir, "old-label")
		require.NoError(t, os.MkdirAll(oldDir, 0o750))
		oldPath := filepath.Join(oldDir, "legacy.md")
		require.NoError(t, os.WriteFile(oldPath, []byte("# Legacy\n\nOld content."), 0o600))

		now := service.nowProvider.Now()
		id, err := service.db.InsertPlan(ctx, dto.InsertPlanParams{
			FileName:   "legacy.md",
			SyncSource: sourcePlansDir,
			Title:      "Legacy",
			Content:    "# Legacy\n\nOld content.",
			CreatedAt:  now,
			ModifiedAt: now,
			IndexedAt:  now,
			FileSize:   1,
			WordCount:  3,
		}, func(int64) (string, error) { return oldPath, nil })
		require.NoError(t, err)

		migrated, err := service.MigrateStorageLayout(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, migrated)

		plan, err := service.db.GetPlanByFileName(ctx, "legacy.md", sourcePlansDir)
		require.NoError(t, err)
		wantPath := service.mirrorPathFor(id, "legacy.md")
		require.Equal(t, wantPath, plan.FilePath)

		data, err := os.ReadFile(wantPath)
		require.NoError(t, err)
		require.Equal(t, "# Legacy\n\nOld content.", string(data))

		// The old file is removed once migrated — no duplicate left behind.
		require.NoFileExists(t, oldPath)

		// Idempotent: a second run finds nothing left to migrate, and doesn't
		// delete the file it just migrated (src == dst guard).
		migratedAgain, err := service.MigrateStorageLayout(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, migratedAgain)
		require.FileExists(t, wantPath)
	})

	t.Run("regenerates the mirror file from DB content when the old file is already gone", func(t *testing.T) {
		service, sourcePlansDir, viewerDir, cleanup := b.setupFn(t)
		defer cleanup()

		// Old file_path points somewhere that was never actually written on disk
		// (or was already lost to a historical cross-source collision) — migration
		// must still succeed by falling back to the DB's content column.
		oldPath := filepath.Join(viewerDir, "gone-label", "vanished.md")

		now := service.nowProvider.Now()
		id, err := service.db.InsertPlan(ctx, dto.InsertPlanParams{
			FileName:   "vanished.md",
			SyncSource: sourcePlansDir,
			Title:      "Vanished",
			Content:    "# Vanished\n\nRecovered from DB.",
			CreatedAt:  now,
			ModifiedAt: now,
			IndexedAt:  now,
			FileSize:   1,
			WordCount:  4,
		}, func(int64) (string, error) { return oldPath, nil })
		require.NoError(t, err)

		migrated, err := service.MigrateStorageLayout(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, migrated)

		wantPath := service.mirrorPathFor(id, "vanished.md")
		data, err := os.ReadFile(wantPath)
		require.NoError(t, err)
		require.Equal(t, "# Vanished\n\nRecovered from DB.", string(data))
	})

	t.Run("migrates version files alongside the mirror file", func(t *testing.T) {
		service, sourcePlansDir, viewerDir, cleanup := b.setupFn(t)
		defer cleanup()

		oldDir := filepath.Join(viewerDir, "old-label")
		require.NoError(t, os.MkdirAll(oldDir, 0o750))
		oldPath := filepath.Join(oldDir, "versioned.md")
		require.NoError(t, os.WriteFile(oldPath, []byte("# Versioned\n\nCurrent."), 0o600))

		now := service.nowProvider.Now()
		id, err := service.db.InsertPlan(ctx, dto.InsertPlanParams{
			FileName:   "versioned.md",
			SyncSource: sourcePlansDir,
			Title:      "Versioned",
			Content:    "# Versioned\n\nCurrent.",
			CreatedAt:  now,
			ModifiedAt: now,
			IndexedAt:  now,
			FileSize:   1,
			WordCount:  2,
		}, func(int64) (string, error) { return oldPath, nil })
		require.NoError(t, err)

		oldVersionsDir := filepath.Join(viewerDir, "versions", "versioned.md")
		require.NoError(t, os.MkdirAll(oldVersionsDir, 0o750))
		oldVersionPath := filepath.Join(oldVersionsDir, "1-1700000000.md")
		require.NoError(t, os.WriteFile(oldVersionPath, []byte("# Versioned\n\nOld version body."), 0o600))
		require.NoError(t, service.db.InsertPlanVersion(ctx, dto.InsertPlanVersionParams{
			PlanID:        id,
			VersionNumber: 1,
			FilePath:      oldVersionPath,
			Content:       "# Versioned\n\nOld version body.",
			WordCount:     3,
			CreatedAt:     now,
		}))

		migrated, err := service.MigrateStorageLayout(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, migrated)

		versions, err := service.db.ListPlanVersionsAll(ctx, id)
		require.NoError(t, err)
		require.Len(t, versions, 1)
		wantVersionPath := filepath.Join(service.versionsDirFor(id), "1-1700000000.md")
		require.Equal(t, wantVersionPath, versions[0].FilePath)

		data, err := os.ReadFile(wantVersionPath)
		require.NoError(t, err)
		require.Equal(t, "# Versioned\n\nOld version body.", string(data))

		// Both old files are removed once migrated.
		require.NoFileExists(t, oldPath)
		require.NoFileExists(t, oldVersionPath)
	})

	t.Run("finishes a version left unmigrated by an interrupted prior run", func(t *testing.T) {
		service, sourcePlansDir, viewerDir, cleanup := b.setupFn(t)
		defer cleanup()

		// Simulate a run that migrated the mirror (and committed) but crashed
		// before reaching this plan's versions: the plan row already has its
		// new, id-keyed file_path, but the version row still points at the old
		// flat versions/<fileName>/ layout.
		now := service.nowProvider.Now()
		id, err := service.db.InsertPlan(ctx, dto.InsertPlanParams{
			FileName:   "half-done.md",
			SyncSource: sourcePlansDir,
			Title:      "Half Done",
			Content:    "# Half Done\n\nCurrent.",
			CreatedAt:  now,
			ModifiedAt: now,
			IndexedAt:  now,
			FileSize:   1,
			WordCount:  2,
		}, func(planID int64) (string, error) {
			newPath := service.mirrorPathFor(planID, "half-done.md")
			require.NoError(t, os.MkdirAll(filepath.Dir(newPath), 0o750))
			require.NoError(t, os.WriteFile(newPath, []byte("# Half Done\n\nCurrent."), 0o600))
			return newPath, nil
		})
		require.NoError(t, err)

		oldVersionsDir := filepath.Join(viewerDir, "versions", "half-done.md")
		require.NoError(t, os.MkdirAll(oldVersionsDir, 0o750))
		oldVersionPath := filepath.Join(oldVersionsDir, "1-1700000000.md")
		require.NoError(t, os.WriteFile(oldVersionPath, []byte("# Half Done\n\nOld version body."), 0o600))
		require.NoError(t, service.db.InsertPlanVersion(ctx, dto.InsertPlanVersionParams{
			PlanID:        id,
			VersionNumber: 1,
			FilePath:      oldVersionPath,
			Content:       "# Half Done\n\nOld version body.",
			WordCount:     4,
			CreatedAt:     now,
		}))

		// The mirror is already on the new layout — the plan still counts as
		// migrated because its stranded version gets picked up and fixed.
		migrated, err := service.MigrateStorageLayout(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, migrated)

		versions, err := service.db.ListPlanVersionsAll(ctx, id)
		require.NoError(t, err)
		require.Len(t, versions, 1)
		wantVersionPath := filepath.Join(service.versionsDirFor(id), "1-1700000000.md")
		require.Equal(t, wantVersionPath, versions[0].FilePath)
		require.FileExists(t, wantVersionPath)
		require.NoFileExists(t, oldVersionPath)
	})

	t.Run("recovers correctly when two plans historically shared the same old path", func(t *testing.T) {
		service, sourcePlansDir, viewerDir, cleanup := b.setupFn(t)
		defer cleanup()

		// Simulate a historical pre-PR-#55 collision: two different sync sources
		// whose labels slugified to the same viewer subdirectory, so both plans'
		// file_path ended up pointing at the identical, last-writer-wins file.
		sharedOldPath := filepath.Join(viewerDir, "collided-label", "shared.md")
		require.NoError(t, os.MkdirAll(filepath.Dir(sharedOldPath), 0o750))
		require.NoError(t, os.WriteFile(sharedOldPath, []byte("# Shared\n\nWhichever plan wrote last."), 0o600))

		now := service.nowProvider.Now()
		otherSource := sourcePlansDir + "-other"
		contentA := "# Shared\n\nPlan A's own recorded content."
		contentB := "# Shared\n\nPlan B's own recorded content."

		idA, err := service.db.InsertPlan(ctx, dto.InsertPlanParams{
			FileName:   "shared.md",
			SyncSource: sourcePlansDir,
			Title:      "Plan A",
			Content:    contentA,
			CreatedAt:  now,
			ModifiedAt: now,
			IndexedAt:  now,
			FileSize:   1,
			WordCount:  4,
		}, func(int64) (string, error) { return sharedOldPath, nil })
		require.NoError(t, err)

		idB, err := service.db.InsertPlan(ctx, dto.InsertPlanParams{
			FileName:   "shared.md",
			SyncSource: otherSource,
			Title:      "Plan B",
			Content:    contentB,
			CreatedAt:  now,
			ModifiedAt: now,
			IndexedAt:  now,
			FileSize:   1,
			WordCount:  4,
		}, func(int64) (string, error) { return sharedOldPath, nil })
		require.NoError(t, err)

		migrated, err := service.MigrateStorageLayout(ctx)
		require.NoError(t, err)
		require.Equal(t, 2, migrated)

		wantPathA := service.mirrorPathFor(idA, "shared.md")
		wantPathB := service.mirrorPathFor(idB, "shared.md")
		require.NotEqual(t, wantPathA, wantPathB)

		dataA, err := os.ReadFile(wantPathA)
		require.NoError(t, err)
		dataB, err := os.ReadFile(wantPathB)
		require.NoError(t, err)

		// Processing order between A and B isn't guaranteed, so assert the
		// invariant rather than which one wins: whichever is processed first
		// recovers the still-present shared file; by the time the other is
		// processed, that file is already gone (deleted by the first plan's
		// migration step), so it must fall back to its own DB content rather
		// than fail, silently duplicate the first plan's content, or come up empty.
		const sharedContent = "# Shared\n\nWhichever plan wrote last."
		gotA, gotB := string(dataA), string(dataB)
		switch {
		case gotA == sharedContent:
			require.Equal(t, contentB, gotB, "the plan processed second must fall back to its own DB content")
		case gotB == sharedContent:
			require.Equal(t, contentA, gotA, "the plan processed second must fall back to its own DB content")
		default:
			t.Fatalf("neither plan recovered the shared on-disk content; got A=%q B=%q", gotA, gotB)
		}

		// The old shared file itself is gone either way.
		require.NoFileExists(t, sharedOldPath)
	})
}

// --- InsertPlan rollback tests ---

func TestInsertPlanRollbackOnWriteFileFailure(t *testing.T) {
	for _, b := range registeredBackends {
		t.Run(b.name, func(t *testing.T) {
			testInsertPlanRollbackOnWriteFileFailure(t, b)
		})
	}
}

// testInsertPlanRollbackOnWriteFileFailure verifies that a failing writeFile
// callback rolls back the whole insert — no zombie row with an unwritten
// file_path can ever persist — and that a subsequent, successful insert for
// the same file then works cleanly. Deliberately does not assert on the
// specific id value reused-or-skipped: whether SQLite reuses the rolled-back
// id (its sequence bump lives in the same transaction) or Postgres leaves a
// gap (its SERIAL sequence is non-transactional by design) is backend
// sequence internals the application doesn't depend on.
func testInsertPlanRollbackOnWriteFileFailure(t *testing.T, b backendSetup) {
	t.Helper()
	ctx := context.Background()

	service, sourcePlansDir, _, cleanup := b.setupFn(t)
	defer cleanup()

	now := service.nowProvider.Now()
	params := dto.InsertPlanParams{
		FileName:   "rollback-me.md",
		SyncSource: sourcePlansDir,
		Title:      "Rollback Me",
		Content:    "# Rollback Me\n\nContent.",
		CreatedAt:  now,
		ModifiedAt: now,
		IndexedAt:  now,
		FileSize:   1,
		WordCount:  2,
	}

	// First attempt: writeFile fails, so the insert must roll back entirely.
	_, err := service.db.InsertPlan(ctx, params, func(int64) (string, error) {
		return "", fmt.Errorf("simulated write failure")
	})
	require.Error(t, err)

	// No zombie row: the plan must not exist after a rolled-back insert.
	_, err = service.db.GetPlanByFileName(ctx, "rollback-me.md", sourcePlansDir)
	require.True(t, dto.IsNotFound(err), "a rolled-back insert must not leave a persisted row")

	// A subsequent, successful insert for the same file must work cleanly.
	id, err := service.db.InsertPlan(ctx, params, func(id int64) (string, error) {
		return service.mirrorPathFor(id, "rollback-me.md"), nil
	})
	require.NoError(t, err)
	require.NotZero(t, id)

	plan, err := service.db.GetPlanByFileName(ctx, "rollback-me.md", sourcePlansDir)
	require.NoError(t, err)
	require.Equal(t, id, plan.ID)
}
