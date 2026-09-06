package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
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

		result, searchResult, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		// 62 plans match "test" (test-plan-1.md, test-plan-2.md, and 60 plan-limit-*.md
		// files, all matched via LIKE '%test%' since "testing" contains "test"); default
		// limit clamps that down to exactly 20.
		require.Equal(t, 20, len(searchResult.Plans))
		require.NotEmpty(t, result.Content)
	})

	t.Run("search with custom limit", func(t *testing.T) {
		args := SearchPlansArgs{
			Query: "",
			Limit: 10,
		}

		result, searchResult, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, 10, len(searchResult.Plans))
	})

	t.Run("search with limit exceeding max (50)", func(t *testing.T) {
		args := SearchPlansArgs{
			Query: "",
			Limit: 100, // Should be clamped to 50
		}

		result, searchResult, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, 50, len(searchResult.Plans))
	})

	t.Run("search with tags AND logic", func(t *testing.T) {
		args := SearchPlansArgs{
			Query:    "",
			Tags:     []string{"api", "auth"},
			MatchAll: true,
			Limit:    20,
		}

		result, searchResult, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Only backend-auth.md carries both "api" and "auth".
		require.Len(t, searchResult.Plans, 1)
		require.Equal(t, "backend-auth.md", searchResult.Plans[0].FileName)

		tagNames := make([]string, len(searchResult.Plans[0].Tags))
		for i, tag := range searchResult.Plans[0].Tags {
			tagNames[i] = tag.Name
		}
		require.ElementsMatch(t, []string{"api", "auth", "backend"}, tagNames)
	})

	t.Run("search with tags OR logic", func(t *testing.T) {
		args := SearchPlansArgs{
			Query:    "",
			Tags:     []string{"frontend", "ui"},
			MatchAll: false,
			Limit:    20,
		}

		result, searchResult, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		// Only test-plan-2.md carries "frontend" or "ui".
		require.Len(t, searchResult.Plans, 1)
		require.Equal(t, "test-plan-2.md", searchResult.Plans[0].FileName)
	})

	t.Run("empty results", func(t *testing.T) {
		args := SearchPlansArgs{
			Query: "zzz-nonexistent-query-zzz",
			Limit: 20,
		}

		result, searchResult, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Empty(t, searchResult.Plans)
	})

	t.Run("verify TOON format output", func(t *testing.T) {
		args := SearchPlansArgs{
			Query: "test",
			Limit: 5,
		}

		result, searchResult, err := handler.SearchPlansHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotEmpty(t, result.Content)
		require.Len(t, searchResult.Plans, 5)

		// Verify it's text content, with the exact TOON array header (row order isn't
		// asserted here since it depends on modified_at, which can tie across files
		// created in the same test run).
		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok)
		header, _, found := strings.Cut(textContent.Text, "\n")
		require.True(t, found)
		require.Equal(t, "plans[5]{file_name,sync_source,sync_label,title,tags,modified_at,reading_time}:", header)
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
			SyncSource: sourcePlansDir,
		}

		result, plan, err := handler.GetPlanHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, plan)
		require.Equal(t, "test-plan.md", plan.FileName)
		require.Equal(t, "Test Plan", plan.Title)
		require.Equal(t, testContent, plan.Content)
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
		require.Equal(t, "fileName is required", err.Error())
	})

	t.Run("empty sync source", func(t *testing.T) {
		args := GetPlanArgs{
			FileName: "test-plan.md",
		}

		result, plan, err := handler.GetPlanHandler(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, plan)
		require.Equal(t, "syncSource is required", err.Error())
	})

	t.Run("plan not found", func(t *testing.T) {
		args := GetPlanArgs{
			FileName:   "nonexistent-plan.md",
			SyncSource: sourcePlansDir,
		}

		result, plan, err := handler.GetPlanHandler(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, plan)
		require.Equal(t, "failed to get plan: not found", err.Error())
	})

	t.Run("verify TOON format with content", func(t *testing.T) {
		args := GetPlanArgs{
			FileName:   "test-plan.md",
			SyncSource: sourcePlansDir,
		}

		result, plan, err := handler.GetPlanHandler(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotEmpty(t, result.Content)

		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok, "result should be TextContent")

		firstLine, _, found := strings.Cut(textContent.Text, "\n")
		require.True(t, found)
		require.Equal(t, "plan:", firstLine)

		_, content, found := strings.Cut(textContent.Text, "\n\ncontent:\n")
		require.True(t, found)
		require.Equal(t, plan.Content, content)
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

func TestGetPlanComments(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)
	_, err = service.AddComment(ctx, "test-plan.md", sourcePlansDir, "first comment")
	require.NoError(t, err)

	handler := &Handler{service: service, server: nil}

	t.Run("lists comments", func(t *testing.T) {
		args := GetPlanCommentsArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir}
		result, res, err := handler.GetPlanComments(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, res.Comments, 1)
		require.Equal(t, "first comment", res.Comments[0].Content)
	})

	t.Run("missing syncSource", func(t *testing.T) {
		args := GetPlanCommentsArgs{FileName: "test-plan.md"}
		result, res, err := handler.GetPlanComments(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Empty(t, res.Comments)
		require.Equal(t, "syncSource is required", err.Error())
	})
}

func TestDeletePlanComment(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)
	comment, err := service.AddComment(ctx, "test-plan.md", sourcePlansDir, "to be deleted")
	require.NoError(t, err)

	handler := &Handler{service: service, server: nil}

	t.Run("deletes the comment", func(t *testing.T) {
		args := DeleteCommentArgs{CommentID: comment.ID}
		result, _, err := handler.DeletePlanComment(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		remaining, err := service.GetPlanComments(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Empty(t, remaining)
	})

	t.Run("missing commentId", func(t *testing.T) {
		result, _, err := handler.DeletePlanComment(ctx, &mcp.CallToolRequest{}, DeleteCommentArgs{})
		require.Error(t, err)
		require.Nil(t, result)
		require.Equal(t, "commentId is required", err.Error())
	})
}

func TestSyncPlansHandler(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent.")

	handler := &Handler{service: service, server: nil}

	result, res, err := handler.SyncPlans(ctx, &mcp.CallToolRequest{}, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, 1, res.Count)
}

func TestRSyncPlansHandler(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)

	handler := &Handler{service: service, server: nil}

	result, _, err := handler.RSyncPlans(ctx, &mcp.CallToolRequest{}, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestGetPlanVersionHistory(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent v1.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)
	require.NoError(t, service.SavePlanVersion(ctx, "test-plan.md", sourcePlansDir, "Content v1."))
	require.NoError(t, service.SavePlanVersion(ctx, "test-plan.md", sourcePlansDir, "Content v2."))

	handler := &Handler{service: service, server: nil}

	t.Run("lists version history", func(t *testing.T) {
		args := GetPlanVersionHistoryArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir}
		result, res, err := handler.GetPlanVersionHistory(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, res.Versions, 2)
	})

	t.Run("missing syncSource", func(t *testing.T) {
		args := GetPlanVersionHistoryArgs{FileName: "test-plan.md"}
		result, res, err := handler.GetPlanVersionHistory(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Empty(t, res.Versions)
		require.Equal(t, "syncSource is required", err.Error())
	})
}

func TestGetPlanVersion(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent v1.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)
	require.NoError(t, service.SavePlanVersion(ctx, "test-plan.md", sourcePlansDir, "Content v1."))

	handler := &Handler{service: service, server: nil}

	t.Run("gets the version", func(t *testing.T) {
		args := GetPlanVersionArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir, VersionNumber: 1}
		result, version, err := handler.GetPlanVersion(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, version)
		require.Equal(t, "Content v1.", version.Content)
	})

	t.Run("missing versionNumber", func(t *testing.T) {
		args := GetPlanVersionArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir}
		result, version, err := handler.GetPlanVersion(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, version)
		require.Equal(t, "versionNumber is required", err.Error())
	})
}

func TestRestorePlanVersion(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent v1.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)
	require.NoError(t, service.SavePlanVersion(ctx, "test-plan.md", sourcePlansDir, "Content v1."))

	handler := &Handler{service: service, server: nil}

	t.Run("restores the version", func(t *testing.T) {
		args := RestorePlanVersionArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir, VersionNumber: 1}
		result, _, err := handler.RestorePlanVersion(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		plan, err := service.GetPlanDetailByFileName(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Equal(t, "Content v1.", plan.Content)
	})

	t.Run("missing versionNumber", func(t *testing.T) {
		args := RestorePlanVersionArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir}
		result, _, err := handler.RestorePlanVersion(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Equal(t, "versionNumber is required", err.Error())
	})
}

func TestDiffPlanVersions(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent v1.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)
	require.NoError(t, service.SavePlanVersion(ctx, "test-plan.md", sourcePlansDir, "Content v1."))
	require.NoError(t, service.SavePlanVersion(ctx, "test-plan.md", sourcePlansDir, "Content v2."))

	handler := &Handler{service: service, server: nil}

	t.Run("diffs two versions", func(t *testing.T) {
		args := DiffPlanVersionsArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir, FromVersion: 1, ToVersion: 2}
		result, res, err := handler.DiffPlanVersions(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Equal(t, "--- Version 1\n+++ Version 2\n@@ -1 +1 @@\n-Content v1.\n+Content v2.\n", res.Diff)
		require.Equal(t, int64(1), res.FromVersion)
		require.Equal(t, int64(2), res.ToVersion)
	})

	t.Run("missing fileName", func(t *testing.T) {
		args := DiffPlanVersionsArgs{SyncSource: sourcePlansDir, FromVersion: 1, ToVersion: 2}
		result, res, err := handler.DiffPlanVersions(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Empty(t, res.Diff)
		require.Equal(t, "fileName is required", err.Error())
	})

	t.Run("missing syncSource", func(t *testing.T) {
		args := DiffPlanVersionsArgs{FileName: "test-plan.md", FromVersion: 1, ToVersion: 2}
		result, res, err := handler.DiffPlanVersions(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Empty(t, res.Diff)
		require.Equal(t, "syncSource is required", err.Error())
	})

	t.Run("version not found", func(t *testing.T) {
		args := DiffPlanVersionsArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir, FromVersion: 1, ToVersion: 99}
		result, res, err := handler.DiffPlanVersions(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Empty(t, res.Diff)
		require.Equal(t, "failed to diff versions: version 99 not found: not found", err.Error())
	})
}

func TestGetAllTags(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)
	require.NoError(t, service.SetPlanTags(ctx, "test-plan.md", sourcePlansDir, []string{"api", "backend"}))

	handler := &Handler{service: service, server: nil}

	result, res, err := handler.GetAllTags(ctx, &mcp.CallToolRequest{}, struct{}{})
	require.NoError(t, err)
	require.NotNil(t, result)

	names := make([]string, len(res.Tags))
	for i, tag := range res.Tags {
		names[i] = tag.Name
	}
	require.ElementsMatch(t, []string{"api", "backend"}, names)
}

func TestGetPlanTags(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)
	require.NoError(t, service.SetPlanTags(ctx, "test-plan.md", sourcePlansDir, []string{"api"}))

	handler := &Handler{service: service, server: nil}

	t.Run("gets the plan's tags", func(t *testing.T) {
		args := GetPlanTagsArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir}
		result, res, err := handler.GetPlanTags(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.Len(t, res.Tags, 1)
		require.Equal(t, "api", res.Tags[0].Name)
	})

	t.Run("missing syncSource", func(t *testing.T) {
		args := GetPlanTagsArgs{FileName: "test-plan.md"}
		result, res, err := handler.GetPlanTags(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Empty(t, res.Tags)
		require.Equal(t, "syncSource is required", err.Error())
	})
}

func TestSetPlanTags(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)

	handler := &Handler{service: service, server: nil}

	t.Run("sets tags", func(t *testing.T) {
		args := SetPlanTagsArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir, Tags: []string{"api", "backend"}}
		result, res, err := handler.SetPlanTags(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		names := make([]string, len(res.Tags))
		for i, tag := range res.Tags {
			names[i] = tag.Name
		}
		require.ElementsMatch(t, []string{"api", "backend"}, names)
	})

	t.Run("second call replaces, not adds", func(t *testing.T) {
		args := SetPlanTagsArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir, Tags: []string{"frontend"}}
		_, res, err := handler.SetPlanTags(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)

		names := make([]string, len(res.Tags))
		for i, tag := range res.Tags {
			names[i] = tag.Name
		}
		require.Equal(t, []string{"frontend"}, names)
	})
}

func TestDeleteTag(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	createTestPlanFile(t, sourcePlansDir, "test-plan.md", "# Test\n\nContent.")
	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)
	require.NoError(t, service.SetPlanTags(ctx, "test-plan.md", sourcePlansDir, []string{"api", "backend"}))

	handler := &Handler{service: service, server: nil}

	t.Run("removes only the named tag", func(t *testing.T) {
		args := DeletePlanTagArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir, Tag: "api"}
		result, res, err := handler.DeleteTag(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)

		names := make([]string, len(res.Tags))
		for i, tag := range res.Tags {
			names[i] = tag.Name
		}
		require.Equal(t, []string{"backend"}, names)

		remaining, err := service.GetPlanTags(ctx, "test-plan.md", sourcePlansDir)
		require.NoError(t, err)
		require.Len(t, remaining, 1)
		require.Equal(t, "backend", remaining[0].Name)
	})

	t.Run("missing tag", func(t *testing.T) {
		args := DeletePlanTagArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir}
		result, res, err := handler.DeleteTag(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Empty(t, res.Tags)
		require.Equal(t, "tag is required", err.Error())
	})

	t.Run("tag does not exist", func(t *testing.T) {
		args := DeletePlanTagArgs{FileName: "test-plan.md", SyncSource: sourcePlansDir, Tag: "nonexistent"}
		result, res, err := handler.DeleteTag(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Empty(t, res.Tags)
		require.Equal(t, `failed to remove tag: tag "nonexistent" not found`, err.Error())
	})
}
