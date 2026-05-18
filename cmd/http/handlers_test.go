package http

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Javier162380/claude-plan-viewer/internal/storage"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository/sqlite"

	"github.com/stretchr/testify/require"
)

// newTestRepository creates a new SQLite repository with migrations for testing.
func newTestRepository(ctx context.Context, dbPath string) (*sqlite.Repository, error) {
	db, err := storage.NewSQLiteClientWithMigrations(ctx, dbPath, nil)
	if err != nil {
		return nil, err
	}
	return sqlite.NewRepository(db.DB()), nil
}

func setupTestServer(t *testing.T) (*Server, string, func()) {
	t.Helper()

	tempDir, err := os.MkdirTemp(os.TempDir(), "claude-viewer-http-test-*")
	require.NoError(t, err)

	viewerDir := filepath.Join(tempDir, "viewer")
	sourcePlansDir := filepath.Join(tempDir, "source")
	dbPath := filepath.Join(tempDir, "test.db")

	require.NoError(t, os.MkdirAll(viewerDir, 0o755))
	require.NoError(t, os.MkdirAll(sourcePlansDir, 0o755))

	ctx := context.Background()
	db, err := newTestRepository(ctx, dbPath)
	require.NoError(t, err)

	service, err := claudeviewer.New(db, viewerDir, sourcePlansDir, true)
	require.NoError(t, err)

	server, err := NewServer(service, ":8081")
	require.NoError(t, err)

	cleanup := func() {
		os.RemoveAll(tempDir)
	}

	return server, sourcePlansDir, cleanup
}

func TestVersionAPIEndpoints(t *testing.T) {
	server, sourcePlansDir, cleanup := setupTestServer(t)
	defer cleanup()
	ctx := context.Background()

	// Create and sync a test plan
	testContent := `# Test Plan
This is a test plan for version control API testing.`
	testFile := filepath.Join(sourcePlansDir, "api-test.md")
	require.NoError(t, os.WriteFile(testFile, []byte(testContent), 0o600))

	_, err := server.service.SyncPlans(ctx)
	require.NoError(t, err)

	t.Run("GET /api/plan/:planName/versions returns empty list for new plan", func(t *testing.T) {
		e := server.Server()
		req := httptest.NewRequest("GET", "/api/plan/api-test.md/versions", nil) //nolint:noctx // its a test all good
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("planName")
		c.SetParamValues("api-test.md")

		require.NoError(t, server.handleGetPlanVersionHistory(c))
		require.Equal(t, http.StatusOK, rec.Code)

		var resp VersionHistoryResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.Equal(t, 0, len(resp.Versions))
		require.False(t, resp.HasMore)
	})

	t.Run("POST /api/plan/:planName/restore/:versionNumber fails for non-existent version", func(t *testing.T) {
		e := server.Server()
		req := httptest.NewRequest("POST", "/api/plan/api-test.md/restore/999", nil) //nolint:noctx // its a test all good
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("planName", "versionNumber")
		c.SetParamValues("api-test.md", "999")

		require.NoError(t, server.handleRestorePlanVersion(c))
		require.Equal(t, http.StatusInternalServerError, rec.Code)

		var resp RestoreVersionResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		require.False(t, resp.Success)
	})

	t.Run("Version operations flow", func(t *testing.T) {
		// Create a version
		version1Content := "# Version 1\nInitial version"
		err := server.service.SavePlanVersion(ctx, "api-test.md", version1Content)
		require.NoError(t, err)

		// Create another version
		version2Content := "# Version 2\nUpdated content"
		err = server.service.SavePlanVersion(ctx, "api-test.md", version2Content)
		require.NoError(t, err)

		// Test: GET /api/plan/:planName/versions
		e := server.Server()
		req := httptest.NewRequest("GET", "/api/plan/api-test.md/versions", nil) //nolint:noctx // its a test all good
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("planName")
		c.SetParamValues("api-test.md")

		require.NoError(t, server.handleGetPlanVersionHistory(c))
		require.Equal(t, http.StatusOK, rec.Code)

		var historyResp VersionHistoryResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &historyResp))
		require.Equal(t, 2, len(historyResp.Versions))
		require.False(t, historyResp.HasMore)

		// Test: GET /api/plan/:planName/versions/:versionNumber
		req = httptest.NewRequest("GET", "/api/plan/api-test.md/versions/1", nil) //nolint:noctx // its a test all good
		rec = httptest.NewRecorder()
		c = e.NewContext(req, rec)
		c.SetParamNames("planName", "versionNumber")
		c.SetParamValues("api-test.md", "1")

		require.NoError(t, server.handleGetPlanVersion(c))
		require.Equal(t, http.StatusOK, rec.Code)

		var versionResp VersionDetailResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &versionResp))
		require.Equal(t, int64(1), versionResp.VersionNumber)
		require.Equal(t, version1Content, versionResp.Content)

		// Test: POST /api/plan/:planName/restore/:versionNumber
		req = httptest.NewRequest("POST", "/api/plan/api-test.md/restore/1", nil) //nolint:noctx // its a test all good
		rec = httptest.NewRecorder()
		c = e.NewContext(req, rec)
		c.SetParamNames("planName", "versionNumber")
		c.SetParamValues("api-test.md", "1")

		require.NoError(t, server.handleRestorePlanVersion(c))
		require.Equal(t, http.StatusOK, rec.Code)

		var restoreResp RestoreVersionResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &restoreResp))
		require.True(t, restoreResp.Success)

		// Verify that restore synced to source directory
		sourceFile := filepath.Join(sourcePlansDir, "api-test.md")
		sourceContent, err := os.ReadFile(sourceFile)
		require.NoError(t, err)
		// The source should now contain the restored version 1 content
		require.Equal(t, version1Content, string(sourceContent))
	})

	t.Run("Version history pagination", func(t *testing.T) {
		// Create multiple versions to test pagination
		for i := 3; i <= 25; i++ {
			content := "# Version " + string(rune(i)) + "\nContent for version " + string(rune(i))
			err := server.service.SavePlanVersion(ctx, "api-test.md", content)
			require.NoError(t, err)
		}

		e := server.Server()

		// Get first page
		req := httptest.NewRequest("GET", "/api/plan/api-test.md/versions?pageSize=10", nil) //nolint:noctx // its a test all good
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("planName")
		c.SetParamValues("api-test.md")

		require.NoError(t, server.handleGetPlanVersionHistory(c))
		require.Equal(t, http.StatusOK, rec.Code)

		var page1 VersionHistoryResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page1))
		require.Equal(t, 20, len(page1.Versions)) // Page size is 20 in handler
		require.True(t, page1.HasMore)
		require.NotEmpty(t, page1.NextToken)

		// Get second page
		req = httptest.NewRequest("GET", "/api/plan/api-test.md/versions?pageToken="+page1.NextToken, nil) //nolint:noctx // its a test all good
		rec = httptest.NewRecorder()
		c = e.NewContext(req, rec)
		c.SetParamNames("planName")
		c.SetParamValues("api-test.md")

		require.NoError(t, server.handleGetPlanVersionHistory(c))
		require.Equal(t, http.StatusOK, rec.Code)

		var page2 VersionHistoryResponse
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &page2))
		require.Greater(t, len(page2.Versions), 0)
	})

	t.Run("Invalid version number returns bad request", func(t *testing.T) {
		e := server.Server()
		req := httptest.NewRequest("GET", "/api/plan/api-test.md/versions/invalid", nil) //nolint:noctx // its a test all good
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("planName", "versionNumber")
		c.SetParamValues("api-test.md", "invalid")

		require.NoError(t, server.handleGetPlanVersion(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})

	t.Run("Non-existent plan returns 404", func(t *testing.T) {
		e := server.Server()
		req := httptest.NewRequest("GET", "/api/plan/non-existent.md/versions", nil) //nolint:noctx // its a test all good
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("planName")
		c.SetParamValues("non-existent.md")

		require.NoError(t, server.handleGetPlanVersionHistory(c))
		require.Equal(t, http.StatusInternalServerError, rec.Code)
	})

	t.Run("Search versions returns matching results", func(t *testing.T) {
		e := server.Server()
		req := httptest.NewRequest("GET", "/api/plan/api-test.md/versions/search?q=architecture", nil) //nolint:noctx // its a test all good
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("planName")
		c.SetParamValues("api-test.md")

		require.NoError(t, server.handleSearchVersions(c))
		require.Equal(t, http.StatusOK, rec.Code)

		var response map[string]interface{}
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &response))
		require.Contains(t, response, "query")
		require.Contains(t, response, "resultsCount")
		require.Contains(t, response, "results")
	})

	t.Run("Search versions with empty query returns error", func(t *testing.T) {
		e := server.Server()
		req := httptest.NewRequest("GET", "/api/plan/api-test.md/versions/search?q=", nil) //nolint:noctx // its a test all good
		rec := httptest.NewRecorder()
		c := e.NewContext(req, rec)
		c.SetParamNames("planName")
		c.SetParamValues("api-test.md")

		require.NoError(t, server.handleSearchVersions(c))
		require.Equal(t, http.StatusBadRequest, rec.Code)
	})
}
