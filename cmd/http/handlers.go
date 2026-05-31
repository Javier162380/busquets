package http

import (
	"context"
	"fmt"
	"html/template"
	"net/http"

	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	"github.com/labstack/echo/v4"
)

func (s *Server) handleIndex(c echo.Context) error {
	ctx := c.Request().Context()
	query := c.QueryParam("q")

	plans, err := s.service.SearchPlansWithReadingTime(ctx, query)
	if err != nil {
		return c.String(http.StatusInternalServerError, "Failed to fetch plans")
	}

	data := IndexPageData{
		Plans: plans,
		Query: query,
	}

	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	return s.templates.ExecuteTemplate(c.Response().Writer, "index", data)
}

func (s *Server) handleViewPlan(c echo.Context) error {
	ctx := c.Request().Context()
	fileName := c.Param("filename")
	syncSource := s.service.SourcePathForLabel(c.Param("source"))

	planDetail, err := s.service.GetPlanDetailByFileName(ctx, fileName, syncSource)
	if err != nil {
		return c.String(http.StatusNotFound, "Plan not found")
	}

	data := ViewPlanData{
		PlanDetail: planDetail,
		//nolint:gosec // G203: Intentional HTML rendering from trusted markdown source
		RenderedHTML: template.HTML(planDetail.RenderedHTML),
	}

	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	return s.templates.ExecuteTemplate(c.Response().Writer, "view", data)
}

func (s *Server) handleSync(c echo.Context) error {
	ctx := c.Request().Context()

	count, err := s.service.SyncPlans(ctx)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to sync plans",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, SyncResponse{
		Success: true,
		Count:   count,
		Message: fmt.Sprintf("Successfully synced %d plans", count),
	})
}

func (s *Server) handleUpdatePlan(c echo.Context) error {
	ctx := c.Request().Context()

	var req UpdatePlanRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
		})
	}

	result, err := s.service.UpdatePlan(ctx, claudeviewer.UpdatePlanRequest{
		FileName:         req.FileName,
		SyncSource:       s.service.SourcePathForLabel(req.SyncSource),
		NewContent:       req.Content,
		LastModifiedTime: req.LastModifiedTime,
		Force:            req.Force,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update plan",
			Details: err.Error(),
		})
	}

	if result.HasConflict {
		return c.JSON(http.StatusConflict, ConflictResponse{
			Success:     false,
			HasConflict: true,
			Conflict: ConflictDetails{
				Message:         result.ConflictInfo.Message,
				CurrentModTime:  result.ConflictInfo.CurrentModTime,
				ExpectedModTime: result.ConflictInfo.ExpectedModTime,
			},
		})
	}

	return c.JSON(http.StatusOK, UpdatePlanResponse{
		Success: true,
		Message: "Plan updated successfully",
	})
}

func (s *Server) handleSavePlanLocal(c echo.Context) error {
	ctx := c.Request().Context()

	var req UpdatePlanRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
		})
	}

	result, err := s.service.SavePlanLocal(ctx, claudeviewer.UpdatePlanRequest{
		FileName:         req.FileName,
		SyncSource:       s.service.SourcePathForLabel(req.SyncSource),
		NewContent:       req.Content,
		LastModifiedTime: req.LastModifiedTime,
		Force:            req.Force,
	})
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to save plan",
			Details: err.Error(),
		})
	}

	if result.HasConflict {
		return c.JSON(http.StatusConflict, ConflictResponse{
			Success:     false,
			HasConflict: true,
			Conflict: ConflictDetails{
				Message:         result.ConflictInfo.Message,
				CurrentModTime:  result.ConflictInfo.CurrentModTime,
				ExpectedModTime: result.ConflictInfo.ExpectedModTime,
			},
		})
	}

	return c.JSON(http.StatusOK, UpdatePlanResponse{
		Success: true,
		Message: "Plan saved successfully",
	})
}

func (s *Server) handleSetting(c echo.Context) error {
	ctx := c.Request().Context()
	variableName := c.Param("variableName")

	if c.Request().Method == http.MethodGet {
		return s.handleGetSetting(c, variableName)
	}

	return s.handlePostSetting(c, ctx, variableName)
}

func (s *Server) handleGetSetting(c echo.Context, variableName string) error {
	ctx := c.Request().Context()

	var varType string
	var defaultValue interface{}
	values := claudeviewer.SettingValues{}

	// Get the setting using the generic method.
	setting, exists, err := s.service.GetSetting(ctx, variableName)
	switch {
	case dto.IsNotFound(err):
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Setting not found",
			Details: err.Error(),
		})
	case err != nil:
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch setting",
			Details: err.Error(),
		})
	}

	switch variableName {
	case claudeviewer.SettingReadingSpeedWPM:
		varType = claudeviewer.SettingTypeNumber
		if exists && setting.IsNumber() {
			v := setting.GetNumberValue()
			values.NumberValue = &v
		}
		defaultValue = claudeviewer.DefaultReadingSpeedWPM

	case claudeviewer.SettingDarkModeEnabled:
		varType = claudeviewer.SettingTypeBoolean
		if exists && setting.IsBoolean() {
			v := setting.GetBooleanValue()
			values.BooleanValue = &v
		}
		defaultValue = true // Default to dark mode

	default:
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Setting not found",
		})
	}

	response := GetSettingResponse{
		VariableName: variableName,
		VariableType: varType,
		Values:       values,
		Default:      defaultValue,
	}

	return c.JSON(http.StatusOK, response)
}

func (s *Server) handlePostSetting(c echo.Context, ctx any, variableName string) error {
	reqCtx := ctx.(context.Context) //nolint:forcetypeassert // Vibecoding.

	var req UpdateSettingRequest
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid request body",
		})
	}

	var values claudeviewer.SettingValues

	switch variableName {
	case claudeviewer.SettingReadingSpeedWPM:
		values = claudeviewer.SettingValues{
			NumberValue: req.NumberValue,
		}

	case claudeviewer.SettingDarkModeEnabled:
		values = claudeviewer.SettingValues{
			BooleanValue: req.BooleanValue,
		}

	default:
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Setting not found",
		})
	}

	if req.DateTimeValue != nil {
		parsedTime, err := claudeviewer.ValidateDateTimeValue(*req.DateTimeValue)
		if err != nil {
			return c.JSON(http.StatusBadRequest, ErrorResponse{
				Error:   "Invalid datetime format",
				Details: err.Error(),
			})
		}
		values.DateTimeValue = &parsedTime
	}

	if err := s.service.SetSetting(reqCtx, variableName, values); err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to update setting",
			Details: err.Error(),
		})
	}

	return c.JSON(http.StatusOK, UpdateSettingResponse{
		Success: true,
		Message: "Setting updated successfully",
	})
}

func (s *Server) handleListPlansWithPagination(c echo.Context) error {
	ctx := c.Request().Context()
	query := c.QueryParam("q")
	pageToken := c.QueryParam("pageToken")

	// Decode pagination token
	token, err := claudeviewer.DecodePaginationToken(pageToken)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Invalid page token",
			Details: err.Error(),
		})
	}

	limit := int64(20)
	offset := token.Offset

	plans, err := s.service.SearchPlansWithPaginationAndReadingTime(ctx, query, limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch plans",
			Details: err.Error(),
		})
	}

	// Calculate next page token
	nextPageToken := ""
	if len(plans) == int(limit) {
		nextPageToken = claudeviewer.EncodePaginationToken(offset + limit)
	}

	data := map[string]interface{}{
		"Plans":         plans,
		"Query":         query,
		"NextPageToken": nextPageToken,
		"CurrentOffset": offset,
	}

	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	return s.templates.ExecuteTemplate(c.Response().Writer, "plan-list-partial", data)
}

// handleGetPlanVersionHistory retrieves version history for a plan with pagination.
func (s *Server) handleGetPlanVersionHistory(c echo.Context) error {
	ctx := c.Request().Context()
	planName := c.Param("planName")
	pageToken := c.QueryParam("pageToken")
	pageSize := int64(20) // Default page size

	// Decode pagination token
	token, err := claudeviewer.DecodePaginationToken(pageToken)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid pagination token",
		})
	}

	syncSource := s.service.SourcePathForLabel(c.Param("source"))
	// Fetch versions with pagination
	versions, err := s.service.GetPlanVersionHistory(ctx, planName, syncSource, token.Offset, pageSize)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Failed to fetch version history",
			Details: err.Error(),
		})
	}

	// Build response
	summaries := make([]VersionSummary, len(versions))
	for i, v := range versions {
		summaries[i] = VersionSummary{
			ID:            v.ID,
			VersionNumber: v.VersionNumber,
			WordCount:     v.WordCount,
			CreatedAt:     v.CreatedAt,
		}
	}

	response := VersionHistoryResponse{
		Versions: summaries,
		HasMore:  int64(len(versions)) >= pageSize,
	}

	// Generate next page token if there are more results
	if response.HasMore {
		response.NextToken = claudeviewer.EncodePaginationToken(token.Offset + pageSize)
	}

	return c.JSON(http.StatusOK, response)
}

// handleGetPlanVersion retrieves a specific version of a plan.
func (s *Server) handleGetPlanVersion(c echo.Context) error {
	ctx := c.Request().Context()
	planName := c.Param("planName")
	versionNumberStr := c.Param("versionNumber")

	// Parse version number
	var versionNumber int64
	_, err := fmt.Sscanf(versionNumberStr, "%d", &versionNumber)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid version number",
		})
	}

	syncSource := s.service.SourcePathForLabel(c.Param("source"))
	// Get version from service
	version, err := s.service.GetPlanVersion(ctx, planName, syncSource, versionNumber)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "Version not found",
			Details: err.Error(),
		})
	}

	response := VersionDetailResponse{
		ID:            version.ID,
		VersionNumber: version.VersionNumber,
		Content:       version.Content,
		WordCount:     version.WordCount,
		CreatedAt:     version.CreatedAt,
	}

	return c.JSON(http.StatusOK, response)
}

// handleRestorePlanVersion restores a plan to a previous version.
func (s *Server) handleRestorePlanVersion(c echo.Context) error {
	ctx := c.Request().Context()
	planName := c.Param("planName")
	versionNumberStr := c.Param("versionNumber")

	// Parse version number
	var versionNumber int64
	_, err := fmt.Sscanf(versionNumberStr, "%d", &versionNumber)
	if err != nil {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Invalid version number",
		})
	}

	syncSource := s.service.SourcePathForLabel(c.Param("source"))
	// Restore the version
	err = s.service.RestorePlanVersion(ctx, planName, syncSource, versionNumber)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, RestoreVersionResponse{
			Success: false,
			Error:   err.Error(),
		})
	}

	return c.JSON(http.StatusOK, RestoreVersionResponse{
		Success: true,
		Message: fmt.Sprintf("Plan '%s' restored to version %d", planName, versionNumber),
	})
}

// handleViewPlanVersions displays the version history page for a plan.
func (s *Server) handleViewPlanVersions(c echo.Context) error {
	ctx := c.Request().Context()
	planName := c.Param("planName")
	syncSource := s.service.SourcePathForLabel(c.Param("source"))

	_, err := s.service.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return c.String(http.StatusNotFound, "Plan not found")
	}

	data := map[string]string{
		"PlanName": planName,
	}

	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	return s.templates.ExecuteTemplate(c.Response().Writer, "versions", data)
}

// handleSearchVersions searches across all versions of a plan.
func (s *Server) handleSearchVersions(c echo.Context) error {
	ctx := c.Request().Context()
	planName := c.Param("planName")
	syncSource := s.service.SourcePathForLabel(c.Param("source"))
	query := c.QueryParam("q")

	_, err := s.service.GetPlanByFileName(ctx, planName, syncSource)
	if err != nil {
		return c.JSON(http.StatusNotFound, ErrorResponse{
			Error: "Plan not found",
		})
	}

	if query == "" {
		return c.JSON(http.StatusBadRequest, ErrorResponse{
			Error: "Search query is required",
		})
	}

	versions, err := s.service.SearchVersions(ctx, planName, syncSource, query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "Search failed",
			Details: err.Error(),
		})
	}

	// Build response with version summaries
	results := make([]map[string]interface{}, 0, len(versions))
	for _, v := range versions {
		results = append(results, map[string]interface{}{
			"versionNumber": v.VersionNumber,
			"createdAt":     v.CreatedAt,
			"wordCount":     v.WordCount,
			"preview":       v.Content[:min(len(v.Content), 200)],
		})
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"query":        query,
		"resultsCount": len(results),
		"results":      results,
	})
}

// min returns the minimum of two integers.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
