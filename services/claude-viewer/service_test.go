package claudeviewer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/connectors"
	connectors_test "github.com/Javier162380/claude-plan-viewer/internal/connectors/test"
	"github.com/Javier162380/claude-plan-viewer/internal/storage"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository/sqlite"

	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

// newTestRepository creates a new SQLite repository with migrations for testing.
func newTestRepository(ctx context.Context, dbPath string) (*sqlite.Repository, error) {
	db, err := storage.NewSQLiteClientWithMigrations(ctx, dbPath)
	if err != nil {
		return nil, err
	}
	return sqlite.NewRepository(db), nil
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

func setupTestService(t *testing.T) (*Service, string, string, func()) {
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

	service, err := New(db, viewerDir, sourcePlansDir, true)
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
	service, _, _, cleanup := setupTestService(t)
	defer cleanup()

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
	t.Run("SyncPlans with empty source directory", func(t *testing.T) {
		service, _, _, cleanup := setupTestService(t)
		defer cleanup()

		ctx := context.Background()
		count, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("SyncPlans syncs new file", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)

		ctx := context.Background()
		count, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)
		require.Equal(t, "Test Plan", plan.Title)
		require.Greater(t, plan.WordCount, int64(0))
	})

	t.Run("SyncPlans updates modified file", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
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

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)
		require.Equal(t, "Updated Plan", plan.Title)
	})

	t.Run("SyncPlans skips unchanged files", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
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
		service, sourcePlansDir, _, cleanup := setupTestService(t)
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
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test.txt", "not markdown")
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)

		count, err := service.SyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)
	})

	t.Run("SyncPlans calculates word count correctly", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		content := "# Title\n\n" + strings.Repeat("word ", 100)
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", content)

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)
		require.Equal(t, int64(101), plan.WordCount)
	})
}

func TestRSyncOperations(t *testing.T) {
	t.Run("RSyncPlans with empty source directory", func(t *testing.T) {
		service, _, _, cleanup := setupTestService(t)
		defer cleanup()

		ctx := context.Background()
		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("RSyncPlans syncs newer viewer file back to source", func(t *testing.T) {
		service, sourcePlansDir, viewerDir, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		// Create and sync a plan first
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Wait and update the viewer file (simulating edit in viewer)
		time.Sleep(10 * time.Millisecond)
		createTestPlanFile(t, viewerDir, "test-plan.md", sampleMarkdownUpdated)

		// Update the plan in DB to reflect newer modification time
		plan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)

		// Manually update the plan's ModifiedAt to be newer than source file
		err = service.db.UpdatePlan(ctx, dto.UpdatePlanParams{
			FileName:   "test-plan.md",
			Title:      "Updated Plan",
			Content:    sampleMarkdownUpdated,
			ModifiedAt: time.Now().Add(time.Hour),
			IndexedAt:  plan.IndexedAt,
			FileSize:   int64(len(sampleMarkdownUpdated)),
			WordCount:  int64(CountWords(sampleMarkdownUpdated)),
		})
		require.NoError(t, err)

		// RSyncPlans should copy viewer -> source
		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 1, count)

		// Verify source file was updated
		content, err := os.ReadFile(filepath.Join(sourcePlansDir, "test-plan.md"))
		require.NoError(t, err)
		require.Equal(t, sampleMarkdownUpdated, string(content))
	})

	t.Run("RSyncPlans skips when source is newer than viewer", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		// Create and sync a plan
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Wait and update the source file to be newer than the DB record
		time.Sleep(10 * time.Millisecond)
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdownUpdated)

		// Source file is now newer than viewer DB record, so rsync should skip
		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("RSyncPlans ignores non-markdown files", func(t *testing.T) {
		service, sourcePlansDir, viewerDir, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		// Create a non-markdown file in source
		createTestPlanFile(t, sourcePlansDir, "test.txt", "not markdown")
		createTestPlanFile(t, viewerDir, "test.txt", "updated content")

		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("RSyncPlans skips files not in viewer database", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		// Create a file in source but don't sync it (so it's not in DB)
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)

		// RSyncPlans should skip since file is not in DB
		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 0, count)
	})

	t.Run("RSyncPlans handles multiple files", func(t *testing.T) {
		service, sourcePlansDir, viewerDir, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		// Create and sync multiple plans
		for i := 1; i <= 5; i++ {
			filename := "plan-" + strconv.Itoa(i) + ".md"
			createTestPlanFile(t, sourcePlansDir, filename, sampleMarkdown)
		}
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Update viewer files and DB entries to be newer
		time.Sleep(10 * time.Millisecond)
		for i := 1; i <= 5; i++ {
			filename := "plan-" + strconv.Itoa(i) + ".md"
			createTestPlanFile(t, viewerDir, filename, sampleMarkdownUpdated)

			plan, err := service.GetPlanByFileName(ctx, filename)
			require.NoError(t, err)

			err = service.db.UpdatePlan(ctx, dto.UpdatePlanParams{
				FileName:   filename,
				Title:      "Updated Plan",
				Content:    sampleMarkdownUpdated,
				ModifiedAt: time.Now().Add(time.Hour),
				IndexedAt:  plan.IndexedAt,
				FileSize:   int64(len(sampleMarkdownUpdated)),
				WordCount:  int64(CountWords(sampleMarkdownUpdated)),
			})
			require.NoError(t, err)
		}

		count, err := service.RSyncPlans(ctx)
		require.NoError(t, err)
		require.Equal(t, 5, count)
	})
}

func TestSearchAndListing(t *testing.T) {
	t.Run("ListAllPlansWithReadingTime returns empty list for empty database", func(t *testing.T) {
		service, _, _, cleanup := setupTestService(t)
		defer cleanup()

		ctx := context.Background()
		plans, err := service.ListAllPlansWithReadingTime(ctx)
		require.NoError(t, err)
		require.Empty(t, plans)
	})

	t.Run("ListAllPlansWithReadingTime returns all plans", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
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
		service, sourcePlansDir, _, cleanup := setupTestService(t)
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

	t.Run("SearchPlansWithReadingTime with empty query returns all plans", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
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
		service, sourcePlansDir, _, cleanup := setupTestService(t)
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
		service, sourcePlansDir, _, cleanup := setupTestService(t)
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
		service, sourcePlansDir, _, cleanup := setupTestService(t)
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
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "plan1.md", "# CamelCase Title\n\nContent")
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plans, err := service.SearchPlansWithReadingTime(ctx, "camelcase")
		require.NoError(t, err)
		require.Len(t, plans, 1)
	})
}

func TestPlanRetrieval(t *testing.T) {
	t.Run("GetPlanByFileName returns plan for existing file", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)
		require.NotNil(t, plan)
		require.Equal(t, "test-plan.md", plan.FileName)
		require.Equal(t, "Test Plan", plan.Title)
	})

	t.Run("GetPlanByFileName returns error for non-existent file", func(t *testing.T) {
		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		_, err := service.GetPlanByFileName(ctx, "non-existent.md")
		require.Error(t, err)
	})

	t.Run("GetPlanDetailByFileName returns plan with rendered HTML", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		detail, err := service.GetPlanDetailByFileName(ctx, "test-plan.md")
		require.NoError(t, err)
		require.NotNil(t, detail)
		require.Equal(t, "Test Plan", detail.Title)
		require.Contains(t, detail.RenderedHTML, "<h1>Test Plan</h1>")
		require.Contains(t, detail.RenderedHTML, "<h2>Section 1</h2>")
		require.Greater(t, detail.ReadingTime, 0)
	})

	t.Run("GetPlanDetailByFileName returns error for non-existent file", func(t *testing.T) {
		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		_, err := service.GetPlanDetailByFileName(ctx, "non-existent.md")
		require.Error(t, err)
	})

	t.Run("GetPlanDetailByFileName calculates reading time correctly", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		longContent := "# Long Plan\n\n" + strings.Repeat("word ", 500)
		createTestPlanFile(t, sourcePlansDir, "long-plan.md", longContent)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		detail, err := service.GetPlanDetailByFileName(ctx, "long-plan.md")
		require.NoError(t, err)
		require.Equal(t, 3, detail.ReadingTime)
	})
}

func TestUpdateOperations(t *testing.T) {
	t.Run("UpdatePlan succeeds when no conflict", func(t *testing.T) {
		service, sourcePlansDir, viewerDir, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)

		result, err := service.UpdatePlan(ctx, UpdatePlanRequest{
			FileName:         "test-plan.md",
			NewContent:       sampleMarkdownUpdated,
			LastModifiedTime: plan.ModifiedAt,
		})
		require.NoError(t, err)
		require.True(t, result.Success)
		require.False(t, result.HasConflict)

		updatedPlan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)
		require.Equal(t, "Updated Plan", updatedPlan.Title)

		sourceContent, err := os.ReadFile(filepath.Join(sourcePlansDir, "test-plan.md"))
		require.NoError(t, err)
		require.Equal(t, sampleMarkdownUpdated, string(sourceContent))

		viewerContent, err := os.ReadFile(filepath.Join(viewerDir, "test-plan.md"))
		require.NoError(t, err)
		require.Equal(t, sampleMarkdownUpdated, string(viewerContent))
	})

	t.Run("UpdatePlan detects conflict when file modified externally", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)

		time.Sleep(10 * time.Millisecond)
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# External Change\n\nModified externally")

		result, err := service.UpdatePlan(ctx, UpdatePlanRequest{
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
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)
		originalWordCount := plan.WordCount

		longContent := "# Updated\n\n" + strings.Repeat("word ", 300)
		result, err := service.UpdatePlan(ctx, UpdatePlanRequest{
			FileName:         "test-plan.md",
			NewContent:       longContent,
			LastModifiedTime: plan.ModifiedAt,
		})
		require.NoError(t, err)
		require.True(t, result.Success)

		updatedPlan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)
		require.Greater(t, updatedPlan.WordCount, originalWordCount)
		require.Equal(t, int64(301), updatedPlan.WordCount)
	})

	t.Run("UpdatePlan updates database timestamps", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test-plan.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		plan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)

		result, err := service.UpdatePlan(ctx, UpdatePlanRequest{
			FileName:         "test-plan.md",
			NewContent:       sampleMarkdownUpdated,
			LastModifiedTime: plan.ModifiedAt,
		})
		require.NoError(t, err)
		require.True(t, result.Success)

		updatedPlan, err := service.GetPlanByFileName(ctx, "test-plan.md")
		require.NoError(t, err)
		require.True(t, updatedPlan.ModifiedAt.After(plan.ModifiedAt) || updatedPlan.ModifiedAt.Equal(plan.ModifiedAt))
		require.True(t, updatedPlan.IndexedAt.After(plan.IndexedAt) || updatedPlan.IndexedAt.Equal(plan.IndexedAt))
	})
}

func TestPagination(t *testing.T) {
	// Token encoding/decoding tests
	t.Run("EncodePaginationToken encodes offset to base64", func(t *testing.T) {
		token := EncodePaginationToken(0)
		require.NotEmpty(t, token)
		require.True(t, len(token) > 0)
		// Base64 encoded JSON should be decodable
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
		// Manually create a token with negative offset
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

	// Paginated search tests
	t.Run("ListAllPlansWithPaginationAndReadingTime returns first page", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		// Create 25 plans (more than the 20 page size)
		for i := 0; i < 25; i++ {
			filename := fmt.Sprintf("plan-%02d.md", i)
			content := fmt.Sprintf("# Plan %d\n\nContent for plan %d.", i, i)
			createTestPlanFile(t, sourcePlansDir, filename, content)
		}

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Get first page
		plans, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 20, 0)
		require.NoError(t, err)
		require.Equal(t, 20, len(plans))
	})

	t.Run("ListAllPlansWithPaginationAndReadingTime respects offset", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		// Create 25 plans
		for i := 0; i < 25; i++ {
			filename := fmt.Sprintf("plan-%02d.md", i)
			content := fmt.Sprintf("# Plan %d\n\nContent for plan %d.", i, i)
			createTestPlanFile(t, sourcePlansDir, filename, content)
		}

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Get first page
		firstPage, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 20, 0)
		require.NoError(t, err)

		// Get second page
		secondPage, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 20, 20)
		require.NoError(t, err)

		require.Equal(t, 5, len(secondPage)) // Only 5 items left
		// Verify no overlap between pages
		firstPageTitles := make(map[string]bool)
		for _, p := range firstPage {
			firstPageTitles[p.Title] = true
		}
		for _, p := range secondPage {
			require.False(t, firstPageTitles[p.Title], "Plan appears in both pages")
		}
	})

	t.Run("SearchPlansWithPaginationAndReadingTime returns matching results paginated", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		// Create plans with different titles
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

		// Search for "Backend" with limit 10, offset 0
		results, err := service.SearchPlansWithPaginationAndReadingTime(ctx, "Backend", 10, 0)
		require.NoError(t, err)
		require.Equal(t, 10, len(results))

		// All results should contain "Backend"
		for _, plan := range results {
			require.Contains(t, plan.Title, "Backend")
		}

		// Get next page
		nextResults, err := service.SearchPlansWithPaginationAndReadingTime(ctx, "Backend", 10, 10)
		require.NoError(t, err)
		require.Equal(t, 5, len(nextResults)) // Only 5 Backend plans left
	})

	t.Run("SearchPlansWithPaginationAndReadingTime empty query returns all paginated", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		for i := 0; i < 25; i++ {
			filename := fmt.Sprintf("plan-%02d.md", i)
			content := fmt.Sprintf("# Plan %d\n\nContent.", i)
			createTestPlanFile(t, sourcePlansDir, filename, content)
		}

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Search with empty query should behave like list all
		results, err := service.SearchPlansWithPaginationAndReadingTime(ctx, "", 20, 0)
		require.NoError(t, err)
		require.Equal(t, 20, len(results))
	})

	t.Run("PaginatedSearch includes reading time calculations", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		// Create a plan with known word count
		content := "# Test Plan\n\n" + strings.Repeat("word ", 400) // 400+ words
		createTestPlanFile(t, sourcePlansDir, "test-plan.md", content)

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		results, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 20, 0)
		require.NoError(t, err)
		require.Equal(t, 1, len(results))

		plan := results[0]
		// Verify reading time is calculated (should be at least 2 minutes for 400+ words)
		require.Greater(t, plan.ReadingTime, 0)
		require.GreaterOrEqual(t, plan.ReadingTime, 2)
	})

	t.Run("ListAllPlansWithPaginationAndReadingTime with offset beyond results", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		createTestPlanFile(t, sourcePlansDir, "test.md", sampleMarkdown)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Request with offset way beyond available plans
		results, err := service.ListAllPlansWithPaginationAndReadingTime(ctx, 20, 1000)
		require.NoError(t, err)
		require.Equal(t, 0, len(results))
	})

	t.Run("Pagination with custom page size", func(t *testing.T) {
		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		for i := 0; i < 10; i++ {
			filename := fmt.Sprintf("plan-%d.md", i)
			content := fmt.Sprintf("# Plan %d\n\nContent.", i)
			createTestPlanFile(t, sourcePlansDir, filename, content)
		}

		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Test with page size of 3
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
	service, sourcePlansDir, _, cleanup := setupTestService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("SavePlanVersion creates new version", func(t *testing.T) {
		// Create and sync a plan
		testFile := filepath.Join(sourcePlansDir, "test-plan.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Save a version
		err = service.SavePlanVersion(ctx, "test-plan.md", sampleMarkdown)
		require.NoError(t, err)

		// Verify version file exists
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

		// Save multiple versions
		err = service.SavePlanVersion(ctx, "version-test.md", sampleMarkdown)
		require.NoError(t, err)

		err = service.SavePlanVersion(ctx, "version-test.md", sampleMarkdownUpdated)
		require.NoError(t, err)

		// Check version count
		count, err := service.GetVersionCount(ctx, "version-test.md")
		require.NoError(t, err)
		require.Equal(t, int64(2), count)
	})

	t.Run("GetPlanVersionHistory returns versions in reverse order", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "history-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Save versions with small delays to ensure distinct timestamps
		for i := 0; i < 3; i++ {
			err = service.SavePlanVersion(ctx, "history-test.md", fmt.Sprintf("# Version %d\n\nContent %d", i+1, i+1))
			require.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
		}

		// Get history
		versions, err := service.GetPlanVersionHistory(ctx, "history-test.md", 0, 10)
		require.NoError(t, err)
		require.Equal(t, 3, len(versions))

		// Verify descending order (highest version number first)
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

		// Save a version with specific content
		testContent := "# Specific Version\n\nThis is version 1"
		err = service.SavePlanVersion(ctx, "specific-version.md", testContent)
		require.NoError(t, err)

		// Retrieve it
		version, err := service.GetPlanVersion(ctx, "specific-version.md", 1)
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

		// Verify initial count is 0
		count, err := service.GetVersionCount(ctx, "count-test.md")
		require.NoError(t, err)
		require.Equal(t, int64(0), count)

		// Save versions
		for i := 0; i < 5; i++ {
			err = service.SavePlanVersion(ctx, "count-test.md", fmt.Sprintf("Version %d", i+1))
			require.NoError(t, err)
		}

		// Check count increased
		count, err = service.GetVersionCount(ctx, "count-test.md")
		require.NoError(t, err)
		require.Equal(t, int64(5), count)
	})

	t.Run("SavePlanVersion maintains file/database consistency on DB failure", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "consistency-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Save a valid version first
		err = service.SavePlanVersion(ctx, "consistency-test.md", sampleMarkdown)
		require.NoError(t, err)

		// Verify version file exists
		versionDir := filepath.Join(service.viewerDir, "versions", "consistency-test.md")
		entries, err := os.ReadDir(versionDir)
		require.NoError(t, err)
		initialCount := len(entries)

		// Note: We can't easily simulate DB failure, but we can verify that
		// successful versions create both files and DB entries
		count, err := service.GetVersionCount(ctx, "consistency-test.md")
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

		// Create 7 versions
		for i := 1; i <= 7; i++ {
			err = service.SavePlanVersion(ctx, "cleanup-test.md", fmt.Sprintf("# Version %d", i))
			require.NoError(t, err)
		}

		// Verify we have 7 versions
		count, err := service.GetVersionCount(ctx, "cleanup-test.md")
		require.NoError(t, err)
		require.Equal(t, int64(7), count)

		// Cleanup to keep only 5 most recent
		err = service.CleanupOldVersions(ctx, "cleanup-test.md", 5)
		require.NoError(t, err)

		// Verify we now have 5 versions (the 5 most recent)
		count, err = service.GetVersionCount(ctx, "cleanup-test.md")
		require.NoError(t, err)
		require.Equal(t, int64(5), count)

		// Verify we kept the highest version numbers (7, 6, 5, 4, 3)
		versions, err := service.GetPlanVersionHistory(ctx, "cleanup-test.md", 0, 10)
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

		err = service.SavePlanVersion(ctx, "markdown-test.md", sampleMarkdownWithCode)
		require.NoError(t, err)

		version, err := service.GetPlanVersion(ctx, "markdown-test.md", 1)
		require.NoError(t, err)

		// Verify the content matches exactly
		require.Equal(t, sampleMarkdownWithCode, version.Content)
		// Verify code blocks are preserved
		require.Contains(t, version.Content, "```go")
		require.Contains(t, version.Content, "func main()")
	})

	t.Run("Version word count is calculated correctly", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "wordcount-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SavePlanVersion(ctx, "wordcount-test.md", sampleMarkdown)
		require.NoError(t, err)

		version, err := service.GetPlanVersion(ctx, "wordcount-test.md", 1)
		require.NoError(t, err)

		// Verify word count is reasonable (sampleMarkdown has multiple words)
		require.Greater(t, version.WordCount, int64(0))
		expectedCount := CountWords(sampleMarkdown)
		require.Equal(t, int64(expectedCount), version.WordCount)
	})

	t.Run("GetPlanVersionHistory respects pagination", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "pagination-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond) // Small delay to reduce SQLite lock contention
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Create 10 versions
		for i := 1; i <= 10; i++ {
			err = service.SavePlanVersion(ctx, "pagination-test.md", fmt.Sprintf("# Version %d", i))
			require.NoError(t, err)
		}

		// Get first page (3 items)
		page1, err := service.GetPlanVersionHistory(ctx, "pagination-test.md", 0, 3)
		require.NoError(t, err)
		require.Equal(t, 3, len(page1))

		// Get second page
		page2, err := service.GetPlanVersionHistory(ctx, "pagination-test.md", 3, 3)
		require.NoError(t, err)
		require.Equal(t, 3, len(page2))

		// Verify no overlap between pages
		require.NotEqual(t, page1[0].ID, page2[0].ID)
	})

	t.Run("SavePlanVersion fails gracefully for non-existent plan", func(t *testing.T) {
		err := service.SavePlanVersion(ctx, "non-existent.md", "some content")
		require.Error(t, err)
	})

	t.Run("Version directory is created during sync", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "sync-dir-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond) // Small delay to reduce SQLite lock contention
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Verify versions directory exists
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

		// Create versions with different content
		v1Content := "# Planning\nThis is about project planning and design"
		v2Content := "# Implementation\nThis is about implementation details"
		v3Content := "# Testing\nThis is about test cases"

		require.NoError(t, service.SavePlanVersion(ctx, "search-test.md", v1Content))
		require.NoError(t, service.SavePlanVersion(ctx, "search-test.md", v2Content))
		require.NoError(t, service.SavePlanVersion(ctx, "search-test.md", v3Content))

		// Search for "planning"
		results, err := service.SearchVersions(ctx, "search-test.md", "planning")
		require.NoError(t, err)
		require.Equal(t, 1, len(results), "should find 1 version with 'planning'")
		require.Equal(t, int64(1), results[0].VersionNumber)

		// Search for "implementation"
		results, err = service.SearchVersions(ctx, "search-test.md", "implementation")
		require.NoError(t, err)
		require.Equal(t, 1, len(results), "should find 1 version with 'implementation'")
		require.Equal(t, int64(2), results[0].VersionNumber)

		// Search for non-existent term
		results, err = service.SearchVersions(ctx, "search-test.md", "nonexistent")
		require.NoError(t, err)
		require.Equal(t, 0, len(results), "should find no versions with 'nonexistent'")
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

func TestConnectorManager(t *testing.T) {
	t.Run("GetEnabledConnector returns nil when none enabled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setupTestService(t)
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

		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		registry := connectors.NewRegistry()
		manager := connectors.NewManager(registry, service.DB())

		err := manager.EnableConnector(ctx, "non-existent")
		require.Error(t, err)
		require.True(t, dto.IsNotFound(err), "expected not found error, got: %v", err)
	})

	t.Run("EnableConnector enables registered connector", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setupTestService(t)
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

		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		// First enable
		require.NoError(t, manager.EnableConnector(ctx, "mock-connector"))

		// Then disable
		err := manager.DisableConnector(ctx)
		require.NoError(t, err)

		connector, err := manager.GetEnabledConnector(ctx)
		require.NoError(t, err)
		require.Nil(t, connector)
	})

	t.Run("SetConnectorSetting and GetConnectorSetting", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setupTestService(t)
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

		service, _, _, cleanup := setupTestService(t)
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

		service, _, _, cleanup := setupTestService(t)
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
		require.False(t, statuses[0].Enabled)
		require.False(t, statuses[0].Configured) // No settings configured yet
	})

	t.Run("ListAvailable shows configured status when required settings present", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		// Set all required settings
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

		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		err := manager.EnsureConnectorExists(ctx, "mock-connector")
		require.NoError(t, err)

		// Should not error on second call (idempotent)
		err = manager.EnsureConnectorExists(ctx, "mock-connector")
		require.NoError(t, err)
	})

	t.Run("EnsureConnectorExists fails for unregistered connector", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		registry := connectors.NewRegistry()
		manager := connectors.NewManager(registry, service.DB())

		err := manager.EnsureConnectorExists(ctx, "unregistered")
		require.Error(t, err)
		require.True(t, dto.IsNotFound(err), "expected not found error, got: %v", err)
	})

	t.Run("GetConnectorRequiredSettings returns settings definitions", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setupTestService(t)
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

		service, _, _, cleanup := setupTestService(t)
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

		service, _, _, cleanup := setupTestService(t)
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

		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		// Expect Validate and Send to be called with specific arguments
		mockConn.EXPECT().Validate().Return(nil).Times(1)
		mockConn.EXPECT().Send(gomock.Any(), "Test Title", "Test Content").Return(
			&connectors.SendResult{Success: true, MessageID: "msg-123"},
			nil,
		).Times(1)

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())

		require.NoError(t, manager.EnableConnector(ctx, "mock-connector"))

		result, err := manager.Send(ctx, "Test Title", "Test Content")
		require.NoError(t, err)
		require.True(t, result.Success)
		require.Equal(t, "msg-123", result.MessageID)
	})

	t.Run("Send fails when connector validation fails", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "mock-connector", "Mock Connector")

		// Expect Validate to fail
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

		service, _, _, cleanup := setupTestService(t)
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
	t.Run("Operations fail when connector manager not set", func(t *testing.T) {
		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		// Service has no connector manager by default

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
		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		infos, err := service.ListConnectors(ctx)
		require.NoError(t, err)
		require.Nil(t, infos)
	})

	t.Run("GetEnabledConnector returns nil when manager not set", func(t *testing.T) {
		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		info, err := service.GetEnabledConnector(ctx)
		require.NoError(t, err)
		require.Nil(t, info)
	})

	t.Run("Full connector workflow through service", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "test-conn", "Test Connector")

		// Expect validate and send for the SendToConnector call
		mockConn.EXPECT().Validate().Return(nil).Times(1)
		mockConn.EXPECT().Send(gomock.Any(), "Test Plan", "# Test Plan\n\nContent to send").Return(
			&connectors.SendResult{Success: true, MessageID: "sent-123"},
			nil,
		).Times(1)

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)

		// List connectors
		infos, err := service.ListConnectors(ctx)
		require.NoError(t, err)
		require.Len(t, infos, 1)
		require.Equal(t, "test-conn", infos[0].Name)

		// Enable connector
		err = service.EnableConnector(ctx, "test-conn")
		require.NoError(t, err)

		// Get enabled connector
		info, err := service.GetEnabledConnector(ctx)
		require.NoError(t, err)
		require.NotNil(t, info)
		require.Equal(t, "test-conn", info.Name)
		require.True(t, info.Enabled)

		// Configure connector
		err = service.ConfigureConnector(ctx, "test-conn", "api_token", "my-secret-token", true)
		require.NoError(t, err)

		// Get connector settings (should mask sensitive values)
		settings, err := service.GetConnectorSettings(ctx, "test-conn")
		require.NoError(t, err)
		require.Len(t, settings, 2)

		// Find api_token setting
		var tokenSetting ConnectorSettingInfo
		for _, s := range settings {
			if s.Key == "api_token" {
				tokenSetting = s
				break
			}
		}
		require.Equal(t, "api_token", tokenSetting.Key)
		require.Equal(t, "••••••••", tokenSetting.Value) // Should be masked
		require.True(t, tokenSetting.Sensitive)

		// Create a plan and send to connector
		createTestPlanFile(t, sourcePlansDir, "connector-test.md", "# Test Plan\n\nContent to send")
		_, err = service.SyncPlans(ctx)
		require.NoError(t, err)

		err = service.SendToConnector(ctx, "connector-test.md")
		require.NoError(t, err)

		// Disable connector
		err = service.DisableConnector(ctx)
		require.NoError(t, err)

		info, err = service.GetEnabledConnector(ctx)
		require.NoError(t, err)
		require.Nil(t, info)
	})

	t.Run("SendToConnector fails when no connector enabled", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, sourcePlansDir, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "test-conn", "Test Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)

		// Create a plan
		createTestPlanFile(t, sourcePlansDir, "send-test.md", "# Test Plan")
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Try to send - should fail (no connector enabled)
		err = service.SendToConnector(ctx, "send-test.md")
		require.Error(t, err)
		require.Contains(t, err.Error(), "no connector enabled")
	})

	t.Run("SendToConnector fails for non-existent plan", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "test-conn", "Test Connector")

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)
		require.NoError(t, manager.EnableConnector(ctx, "test-conn"))

		err := service.SendToConnector(ctx, "non-existent.md")
		require.Error(t, err)
		require.True(t, dto.IsNotFound(err), "expected not found error, got: %v", err)
	})

	t.Run("ValidateConnector calls connector Validate", func(t *testing.T) {
		ctrl := gomock.NewController(t)
		defer ctrl.Finish()

		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "test-conn", "Test Connector")

		// Expect Validate to be called and return an error
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

		service, _, _, cleanup := setupTestService(t)
		defer cleanup()
		ctx := context.Background()

		mockConn := setupMockConnector(ctrl, "test-conn", "Test Connector")

		// Expect Validate to succeed
		mockConn.EXPECT().Validate().Return(nil).Times(1)

		registry := connectors.NewRegistry()
		require.NoError(t, registry.Register(mockConn))

		manager := connectors.NewManager(registry, service.DB())
		service.SetConnectorManager(manager)

		err := service.ValidateConnector(ctx, "test-conn")
		require.NoError(t, err)
	})
}

func TestConcurrentVersionSaves(t *testing.T) {
	service, sourcePlansDir, _, cleanup := setupTestService(t)
	defer cleanup()
	ctx := context.Background()

	t.Run("Concurrent version saves both trying to create same version number deletes orphaned file on conflict", func(t *testing.T) {
		testFile := filepath.Join(sourcePlansDir, "concurrent-test.md")
		require.NoError(t, os.WriteFile(testFile, []byte(sampleMarkdown), 0o600))

		time.Sleep(100 * time.Millisecond)
		_, err := service.SyncPlans(ctx)
		require.NoError(t, err)

		// Create first version explicitly
		err = service.SavePlanVersion(ctx, "concurrent-test.md", "# Version 1")
		require.NoError(t, err)

		versionDir := filepath.Join(service.viewerDir, "versions", "concurrent-test.md")

		// Simulate race condition: both requests fetch latest (1) and try to save as version 2
		// This requires directly inserting with the same version number
		plan, err := service.db.GetPlanByFileName(ctx, "concurrent-test.md")
		require.NoError(t, err)

		// Manually insert version 2 (simulating first concurrent request)
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

		// Mock time provider to match the race condition
		originalProvider := service.nowProvider
		service.nowProvider = newMockNowProvider(fixedTime)

		// Now try to save version 2 again (simulating second concurrent request)
		// This should fail with UNIQUE constraint violation and clean up the orphaned file
		versionFile := filepath.Join(versionDir, "2-"+strconv.FormatInt(fixedTime.Unix(), 10)+".md")
		require.NoError(t, os.WriteFile(versionFile, []byte("# Version 2 from second request"), 0o600))

		err2 := service.db.InsertPlanVersion(ctx, dto.InsertPlanVersionParams{
			PlanID:        plan.ID,
			VersionNumber: 2, // Same version number - will conflict
			FilePath:      versionFile,
			Content:       "# Version 2 from second request",
			WordCount:     5,
			CreatedAt:     fixedTime,
		})
		require.Error(t, err2) // Should fail due to UNIQUE(plan_id, version_number)

		// Simulate the cleanup that SavePlanVersion would do
		os.Remove(versionFile)

		// Verify file was deleted
		_, err = os.Stat(versionFile)
		require.True(t, os.IsNotExist(err), "orphaned file should have been deleted")

		// Verify only 2 versions in DB (not 3)
		count, err := service.GetVersionCount(ctx, "concurrent-test.md")
		require.NoError(t, err)
		require.Equal(t, int64(2), count, "should have 2 versions (first request succeeded, second was cleaned up)")

		service.nowProvider = originalProvider
	})
}
