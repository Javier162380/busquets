package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/Javier162380/busquets/internal/config"
	"github.com/Javier162380/busquets/internal/storage"
	"github.com/Javier162380/busquets/services/busquets"
	"github.com/Javier162380/busquets/services/busquets/repository/sqlite"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// setupTestService creates a test service with a real SQLite database.
func setupTestService(t *testing.T) (*busquets.Service, string, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp(os.TempDir(), "mcp-test-*")
	require.NoError(t, err)

	viewerDir := filepath.Join(tempDir, "viewer")
	sourcePlansDir := filepath.Join(tempDir, "source")
	dbPath := filepath.Join(tempDir, "test.db")

	require.NoError(t, os.MkdirAll(viewerDir, 0o755))
	require.NoError(t, os.MkdirAll(sourcePlansDir, 0o755))

	ctx := context.Background()
	db, err := storage.NewSQLiteClientWithMigrations(ctx, dbPath, nil)
	require.NoError(t, err)

	repo := sqlite.NewRepository(db.DB())
	service, err := busquets.New(ctx, repo, viewerDir, []config.SyncDir{{Path: sourcePlansDir, Label: "test"}}, true)
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return service, sourcePlansDir, cleanup
}

// createTestPlanFile creates a test plan file in the source directory.
func createTestPlanFile(t *testing.T, dir, filename, content string) {
	t.Helper()
	path := filepath.Join(dir, filename)
	err := os.WriteFile(path, []byte(content), 0o600)
	require.NoError(t, err)
}

func TestSearchPlansHandler(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	// Create test plan files with different content
	createTestPlanFile(t, sourcePlansDir, "test-plan-1.md", "# Test Plan 1\n\nThis is a test plan about authentication.")
	createTestPlanFile(t, sourcePlansDir, "test-plan-2.md", "# Test Plan 2\n\nThis is a UI components plan.")
	createTestPlanFile(t, sourcePlansDir, "backend-auth.md", "# Backend Auth\n\nAuthentication system design.")

	// Create many plans for limit testing (need more than 50 to test clamping)
	for i := 0; i < 60; i++ {
		content := "# Plan " + string(rune('0'+(i/10))) + string(rune('0'+(i%10))) + "\n\nSome content for testing limits."
		createTestPlanFile(t, sourcePlansDir, "plan-limit-"+string(rune('0'+(i/10)))+string(rune('0'+(i%10)))+".md", content)
	}

	// Sync plans to database
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)

	// Assign tags explicitly (tags are DB-only, not read from file content)
	require.NoError(t, service.SetPlanTags(ctx, "test-plan-1.md", sourcePlansDir, []string{"api", "backend"}))
	require.NoError(t, service.SetPlanTags(ctx, "test-plan-2.md", sourcePlansDir, []string{"frontend", "ui"}))
	require.NoError(t, service.SetPlanTags(ctx, "backend-auth.md", sourcePlansDir, []string{"api", "auth", "backend"}))

	// Create handler
	handler := &Handler{
		service: service,
		server:  nil,
	}

	t.Run("successful search with default limit", func(t *testing.T) {
		args := SearchPlansArgs{
			Query: "test",
			Limit: 0, // Should default to 20
		}

		result, plans, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Greater(t, len(plans), 0)
		require.LessOrEqual(t, len(plans), 20)
		require.NotEmpty(t, result.Content)
	})

	t.Run("search with custom limit", func(t *testing.T) {
		args := SearchPlansArgs{
			Query: "",
			Limit: 10,
		}

		result, plans, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, 10, len(plans))
	})

	t.Run("search with limit exceeding max (50)", func(t *testing.T) {
		args := SearchPlansArgs{
			Query: "",
			Limit: 100, // Should be clamped to 50
		}

		result, plans, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, 50, len(plans))
	})

	t.Run("search with tags AND logic", func(t *testing.T) {
		args := SearchPlansArgs{
			Query:    "",
			Tags:     []string{"api", "auth"},
			MatchAll: true,
			Limit:    20,
		}

		result, plans, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Verify all returned plans have both tags
		for _, plan := range plans {
			tagNames := make([]string, len(plan.Tags))
			for i, tag := range plan.Tags {
				tagNames[i] = tag.Name
			}
			require.Contains(t, tagNames, "api")
			require.Contains(t, tagNames, "auth")
		}
	})

	t.Run("search with tags OR logic", func(t *testing.T) {
		args := SearchPlansArgs{
			Query:    "",
			Tags:     []string{"frontend", "ui"},
			MatchAll: false,
			Limit:    20,
		}

		result, plans, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Greater(t, len(plans), 0, "should find plans with either tag")
	})

	t.Run("empty results", func(t *testing.T) {
		args := SearchPlansArgs{
			Query: "zzz-nonexistent-query-zzz",
			Limit: 20,
		}

		result, plans, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Empty(t, plans)
	})

	t.Run("verify TOON format output", func(t *testing.T) {
		args := SearchPlansArgs{
			Query: "test",
			Limit: 5,
		}

		result, _, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotEmpty(t, result.Content)

		// Verify it's text content
		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		require.Contains(t, textContent.Text, "plans")
	})
}

func TestGetPlanHandler(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	// Create test plan files
	testContent := `# Test Plan

Tags: test, integration

This is a comprehensive test plan with multiple sections.

## Section 1

Some content here about testing.

## Section 2

More testing details.`

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", testContent)

	// Sync plans to database
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)

	// Create handler
	handler := &Handler{
		service: service,
		server:  nil,
	}

	t.Run("successful get plan", func(t *testing.T) {
		args := GetPlanArgs{
			FileName:   "test-plan.md",
			SyncSource: "test",
		}

		result, plan, err := handler.GetPlanHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, plan)
		require.Equal(t, "test-plan.md", plan.FileName)
		require.Equal(t, "Test Plan", plan.Title)
		require.Contains(t, plan.Content, "comprehensive test plan")
		require.NotEmpty(t, result.Content)
	})

	t.Run("empty filename", func(t *testing.T) {
		args := GetPlanArgs{
			FileName: "",
		}

		result, plan, err := handler.GetPlanHandler(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, plan)
		require.Contains(t, err.Error(), "fileName is required")
	})

	t.Run("plan not found", func(t *testing.T) {
		args := GetPlanArgs{
			FileName:   "nonexistent-plan.md",
			SyncSource: "test",
		}

		result, plan, err := handler.GetPlanHandler(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, plan)
		require.Contains(t, err.Error(), "failed to get plan")
	})

	t.Run("unknown source returns error", func(t *testing.T) {
		args := GetPlanArgs{
			FileName:   "test-plan.md",
			SyncSource: "does-not-exist",
		}

		result, plan, err := handler.GetPlanHandler(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, plan)
		require.Contains(t, err.Error(), "unknown source")
	})

	t.Run("verify TOON format with content", func(t *testing.T) {
		args := GetPlanArgs{
			FileName:   "test-plan.md",
			SyncSource: "test",
		}

		result, _, err := handler.GetPlanHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotEmpty(t, result.Content)

		// Verify it's text content
		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok, "result should be TextContent")
		require.Contains(t, textContent.Text, "plan")
		require.Contains(t, textContent.Text, "content:")
		require.Contains(t, textContent.Text, "comprehensive test plan")
	})
}

func TestAddPlanComment(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test Plan\n\nContent.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)

	handler := &Handler{
		service: service,
		server:  nil,
	}

	t.Run("successful add comment", func(t *testing.T) {
		args := AddPlanCommentArgs{
			FileName:   "test-plan.md",
			SyncSource: sourcePlansDir,
			Comment:    "looks good",
		}

		result, comment, err := handler.AddPlanComment(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, comment)
		require.Equal(t, "looks good", comment.Content)
		require.NotEmpty(t, result.Content)
	})

	t.Run("empty comment", func(t *testing.T) {
		args := AddPlanCommentArgs{
			FileName:   "test-plan.md",
			SyncSource: sourcePlansDir,
		}

		result, comment, err := handler.AddPlanComment(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, comment)
		require.Equal(t, "invalid argument, comment can't be empty", err.Error())
	})

	t.Run("empty filename", func(t *testing.T) {
		args := AddPlanCommentArgs{
			SyncSource: sourcePlansDir,
			Comment:    "looks good",
		}

		result, comment, err := handler.AddPlanComment(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, comment)
		require.Equal(t, "fileName is required", err.Error())
	})

	t.Run("empty sync source", func(t *testing.T) {
		args := AddPlanCommentArgs{
			FileName: "test-plan.md",
			Comment:  "looks good",
		}

		result, comment, err := handler.AddPlanComment(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, comment)
		require.Equal(t, "syncSource is required", err.Error())
	})

	t.Run("plan not found", func(t *testing.T) {
		args := AddPlanCommentArgs{
			FileName:   "nonexistent-plan.md",
			SyncSource: sourcePlansDir,
			Comment:    "looks good",
		}

		result, comment, err := handler.AddPlanComment(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, comment)
		require.Equal(t, "failed to add comment: failed to get plan: not found", err.Error())
	})
}
