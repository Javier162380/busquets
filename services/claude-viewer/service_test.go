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

func setupTestServiceBackendSQLite(t *testing.T) (*Service, string, string, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp(os.TempDir(), "claude-viewer-test-*")
	require.NoError(t, err)

	viewerDir := filepath.Join(tempDir, "viewer")
	sourcePlansDir := filepath.Join(tempDir, "source")
	dbPath := filepath.Join(tempDir, "test.db")

	require.NoError(t, os.MkdirAll(viewerDir, 0o755))
	require.NoError(t, os.MkdirAll(sourcePlansDir, 0o755))

	ctx := context.Background()
	db, err := newTestRepository(ctx, dbPath)
	require.NoError(t, err)

	service, err := New(db, viewerDir, []config.SyncDir{{Path: sourcePlansDir, Label: "test"}}, true)
	require.NoError(t, err)

	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	service.nowProvider = newMockNowProvider(fixedTime)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return service, sourcePlansDir, viewerDir, cleanup
}

func createTestPlanFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
}

func setupTestServiceBackendPostgres(t *testing.T) (*Service, string, string, func()) {
	t.Helper()
	ctx := context.Background()

	// Each call gets its own isolated database within the shared container.
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

	repo := postgresrepo.NewRepository(pool.Pool())

	tempDir, err := os.MkdirTemp(os.TempDir(), "claude-viewer-pg-test-*")
	require.NoError(t, err)

	viewerDir := filepath.Join(tempDir, "viewer")
	sourcePlansDir := filepath.Join(tempDir, "source")
	require.NoError(t, os.MkdirAll(viewerDir, 0o755))
	require.NoError(t, os.MkdirAll(sourcePlansDir, 0o755))

	svc, err := New(repo, viewerDir, []config.SyncDir{{Path: sourcePlansDir, Label: "test"}}, true)
	require.NoError(t, err)

	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	svc.nowProvider = newMockNowProvider(fixedTime)

	cleanup := func() {
		pool.Close()
		dropPool, err := pgxpool.New(ctx, adminConnStr)
		if err == nil {
			_, _ = dropPool.Exec(ctx, "DROP DATABASE "+dbName+" WITH (FORCE)")
			dropPool.Close()
		}
		os.RemoveAll(tempDir)
	}
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

type backendSetup struct {
	name    string
	setupFn serviceSetupFn
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
		service, sourcePlansDir, viewerDir, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		// Sync indexes the plan and copies it to viewerDir.
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Simulate the plan being deleted from sourcePlansDir.
		require.NoError(t, os.Remove(filepath.Join(sourcePlansDir, "test-plan.md")))

		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		content, err := os.ReadFile(filepath.Join(sourcePlansDir, "test-plan.md"))
		require.NoError(t, err)
		require.Equal(t, sampleMarkdown, string(content))
		content, err = os.ReadFile(filepath.Join(viewerDir, "test", "test-plan.md"))
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
		service, sourcePlansDir, viewerDir, cleanup := setup(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Remove from both locations — nothing can be restored.
		require.NoError(t, os.Remove(filepath.Join(sourcePlansDir, "test-plan.md")))
		require.NoError(t, os.Remove(filepath.Join(viewerDir, "test", "test-plan.md")))

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
		service, sourcePlansDir, viewerDir, cleanup := setup(t)
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

		viewerContent, err := os.ReadFile(filepath.Join(viewerDir, "test", "test-plan.md"))
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

		versionDir := filepath.Join(service.viewerDir, "versions", "test-plan.md")
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

		versionDir := filepath.Join(service.viewerDir, "versions", "consistency-test.md")
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

	t.Run("Version directory is created during sync", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "sync-dir-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		versionBaseDir := filepath.Join(service.viewerDir, "versions")
		info, err := os.Stat(versionBaseDir)
		require.NoError(t, err)
		require.True(t, info.IsDir())
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

		versionDir := filepath.Join(service.viewerDir, "versions", "concurrent-test.md")

		plan, err := service.db.GetPlanByFileName(ctx, "concurrent-test.md", sourcePlansDir)
		require.NoError(t, err)

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

type multiSourceSetupFn func(t *testing.T) (*Service, multiSourceSetup, func())

func setupMultiSourceSQLite(t *testing.T) (*Service, multiSourceSetup, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp(os.TempDir(), "claude-viewer-multi-*")
	require.NoError(t, err)

	viewerDir := filepath.Join(tempDir, "viewer")
	sourceDir1 := filepath.Join(tempDir, "source-a")
	sourceDir2 := filepath.Join(tempDir, "source-b")
	dbPath := filepath.Join(tempDir, "test.db")

	require.NoError(t, os.MkdirAll(viewerDir, 0o755))
	require.NoError(t, os.MkdirAll(sourceDir1, 0o755))
	require.NoError(t, os.MkdirAll(sourceDir2, 0o755))

	ctx := context.Background()
	db, err := newTestRepository(ctx, dbPath)
	require.NoError(t, err)

	svc, err := New(db, viewerDir, []config.SyncDir{
		{Path: sourceDir1, Label: "dir-a"},
		{Path: sourceDir2, Label: "dir-b"},
	}, true)
	require.NoError(t, err)

	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	svc.nowProvider = newMockNowProvider(fixedTime)

	return svc, multiSourceSetup{
		sourceDir1: sourceDir1,
		sourceDir2: sourceDir2,
		viewerDir:  viewerDir,
	}, func() { os.RemoveAll(tempDir) }
}

func setupMultiSourcePostgres(t *testing.T) (*Service, multiSourceSetup, func()) {
	t.Helper()
	ctx := context.Background()

	dbName := fmt.Sprintf("testdb_multi_%d", time.Now().UnixNano())
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

	repo := postgresrepo.NewRepository(pool.Pool())

	tempDir, err := os.MkdirTemp(os.TempDir(), "claude-viewer-pg-multi-*")
	require.NoError(t, err)

	viewerDir := filepath.Join(tempDir, "viewer")
	sourceDir1 := filepath.Join(tempDir, "source-a")
	sourceDir2 := filepath.Join(tempDir, "source-b")

	require.NoError(t, os.MkdirAll(viewerDir, 0o755))
	require.NoError(t, os.MkdirAll(sourceDir1, 0o755))
	require.NoError(t, os.MkdirAll(sourceDir2, 0o755))

	svc, err := New(repo, viewerDir, []config.SyncDir{
		{Path: sourceDir1, Label: "dir-a"},
		{Path: sourceDir2, Label: "dir-b"},
	}, true)
	require.NoError(t, err)

	fixedTime := time.Date(2024, 1, 15, 12, 0, 0, 0, time.UTC)
	svc.nowProvider = newMockNowProvider(fixedTime)

	cleanup := func() {
		pool.Close()
		dropPool, err := pgxpool.New(ctx, adminConnStr)
		if err == nil {
			_, _ = dropPool.Exec(ctx, "DROP DATABASE "+dbName+" WITH (FORCE)")
			dropPool.Close()
		}
		os.RemoveAll(tempDir)
	}
	return svc, multiSourceSetup{
		sourceDir1: sourceDir1,
		sourceDir2: sourceDir2,
		viewerDir:  viewerDir,
	}, cleanup
}

func multiSourceTestBackends() []struct {
	name    string
	setupFn multiSourceSetupFn
} {
	backends := []struct {
		name    string
		setupFn multiSourceSetupFn
	}{
		{"sqlite", setupMultiSourceSQLite},
	}
	if os.Getenv("INTEGRATION") != "" {
		backends = append(backends, struct {
			name    string
			setupFn multiSourceSetupFn
		}{"postgres", setupMultiSourcePostgres})
	}
	return backends
}

// ---- Multi-source tests ----

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plans", "plans"},
		{"My Plans", "my-plans"},
		{"WORK", "work"},
		{"A B C", "a-b-c"},
		{"already-slug", "already-slug"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			require.Equal(t, tc.want, slugify(tc.input))
		})
	}
}

func TestLabelHelpers(t *testing.T) {
	for _, b := range multiSourceTestBackends() {
		t.Run(b.name, func(t *testing.T) {
			testLabelHelpers(t, b.setupFn)
		})
	}
}

func testLabelHelpers(t *testing.T, setup multiSourceSetupFn) {
	t.Helper()
	svc, dirs, cleanup := setup(t)
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

	t.Run("viewerSubdirFor returns slugified label", func(t *testing.T) {
		require.Equal(t, "dir-a", svc.viewerSubdirFor(dirs.sourceDir1))
		require.Equal(t, "dir-b", svc.viewerSubdirFor(dirs.sourceDir2))
	})
}

func TestMultiDirSyncPlans(t *testing.T) {
	for _, b := range multiSourceTestBackends() {
		t.Run(b.name, func(t *testing.T) {
			testMultiDirSyncPlans(t, b.setupFn)
		})
	}
}

func testMultiDirSyncPlans(t *testing.T, setup multiSourceSetupFn) {
	t.Helper()
	ctx := context.Background()

	t.Run("plans from both dirs are indexed", func(t *testing.T) {
		svc, dirs, cleanup := setup(t)
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
		svc, dirs, cleanup := setup(t)
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
		svc, dirs, cleanup := setup(t)
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
		svc, dirs, cleanup := setup(t)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "label-test.md", "# Label Test\n\nContent.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		detail, err := svc.GetPlanDetailByFileName(ctx, "label-test.md", dirs.sourceDir1)
		require.NoError(t, err)
		require.Equal(t, "dir-a", detail.SyncLabel)
	})

	t.Run("viewer copies land in separate subdirs", func(t *testing.T) {
		svc, dirs, cleanup := setup(t)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "plan.md", "# Plan A\n\nContent.")
		createTestPlanFile(t, dirs.sourceDir2, "plan.md", "# Plan B\n\nContent.")
		_, err := svc.SyncPlans(ctx)
		require.NoError(t, err)

		_, err = os.Stat(filepath.Join(dirs.viewerDir, "dir-a", "plan.md"))
		require.NoError(t, err, "viewer copy missing from dir-a subdir")

		_, err = os.Stat(filepath.Join(dirs.viewerDir, "dir-b", "plan.md"))
		require.NoError(t, err, "viewer copy missing from dir-b subdir")
	})

	t.Run("second sync is idempotent across dirs", func(t *testing.T) {
		svc, dirs, cleanup := setup(t)
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
		svc, dirs, cleanup := setup(t)
		defer cleanup()

		createTestPlanFile(t, dirs.sourceDir1, "only-a.md", "# Only A\n\nContent.")
		// sourceDir2 intentionally empty

		count, err := svc.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})
}

func TestMultiDirRSyncPlans(t *testing.T) {
	for _, b := range multiSourceTestBackends() {
		t.Run(b.name, func(t *testing.T) {
			testMultiDirRSyncPlans(t, b.setupFn)
		})
	}
}

func testMultiDirRSyncPlans(t *testing.T, setup multiSourceSetupFn) {
	t.Helper()
	ctx := context.Background()

	t.Run("restores plan to its original source dir", func(t *testing.T) {
		svc, dirs, cleanup := setup(t)
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
		svc, dirs, cleanup := setup(t)
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
	for _, b := range multiSourceTestBackends() {
		t.Run(b.name, func(t *testing.T) {
			testMultiDirUpdatePlan(t, b.setupFn)
		})
	}
}

func testMultiDirUpdatePlan(t *testing.T, setup multiSourceSetupFn) {
	t.Helper()
	ctx := context.Background()

	t.Run("update writes to correct source path", func(t *testing.T) {
		svc, dirs, cleanup := setup(t)
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
		svc, dirs, cleanup := setup(t)
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
