package http

import (
	"html/template"
	"time"

	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
)

// === Template Data Types ===

// IndexPageData contains data for the index template.
type IndexPageData struct {
	Plans []claudeviewer.PlanSummary
	Query string
}

// ViewPlanData contains data for the view plan template.
type ViewPlanData struct {
	PlanDetail   *claudeviewer.PlanDetail
	RenderedHTML template.HTML
}

// === API Request/Response Types ===

// UpdatePlanRequest represents the request body for updating a plan.
type UpdatePlanRequest struct {
	FileName         string    `json:"fileName"`
	Content          string    `json:"content"`
	LastModifiedTime time.Time `json:"lastModifiedTime"`
	Force            bool      `json:"force,omitempty"`
}

// UpdatePlanResponse represents the response for a successful update.
type UpdatePlanResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// ConflictResponse represents a conflict error response.
type ConflictResponse struct {
	Success     bool            `json:"success"`
	HasConflict bool            `json:"hasConflict"`
	Conflict    ConflictDetails `json:"conflict"`
}

// ConflictDetails contains information about a detected conflict.
type ConflictDetails struct {
	Message         string    `json:"message"`
	CurrentModTime  time.Time `json:"currentModTime"`
	ExpectedModTime time.Time `json:"expectedModTime"`
}

// ErrorResponse represents a generic error response.
type ErrorResponse struct {
	Error   string `json:"error"`
	Details string `json:"details,omitempty"`
}

// SyncResponse represents the response for sync operations.
type SyncResponse struct {
	Success bool   `json:"success"`
	Count   int    `json:"count"`
	Message string `json:"message"`
}

// === Settings API Types ===

// GetSettingResponse returns the requested setting.
type GetSettingResponse struct {
	VariableName string                     `json:"variableName"`
	VariableType string                     `json:"variableType"`
	Values       claudeviewer.SettingValues `json:"values"`
	Default      interface{}                `json:"default"`
}

// UpdateSettingRequest payload for POST.
type UpdateSettingRequest struct {
	StringValue   *string  `json:"stringValue,omitempty"`
	NumberValue   *float64 `json:"numberValue,omitempty"`
	BooleanValue  *bool    `json:"booleanValue,omitempty"`
	DateTimeValue *string  `json:"dateTimeValue,omitempty"` // ISO 8601 format
}

// UpdateSettingResponse returns success/error.
type UpdateSettingResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}

// === Version Control API Types ===

// VersionSummary represents a single version in the history.
type VersionSummary struct {
	ID            int64     `json:"id"`
	VersionNumber int64     `json:"versionNumber"`
	WordCount     int64     `json:"wordCount"`
	CreatedAt     time.Time `json:"createdAt"`
}

// VersionHistoryResponse represents paginated version history.
type VersionHistoryResponse struct {
	Versions  []VersionSummary `json:"versions"`
	NextToken string           `json:"nextToken,omitempty"` // Base64-encoded pagination token
	HasMore   bool             `json:"hasMore"`
}

// VersionDetailResponse represents a complete version with content.
type VersionDetailResponse struct {
	ID            int64     `json:"id"`
	VersionNumber int64     `json:"versionNumber"`
	Content       string    `json:"content"`
	WordCount     int64     `json:"wordCount"`
	CreatedAt     time.Time `json:"createdAt"`
}

// RestoreVersionRequest represents a request to restore a plan to a previous version.
type RestoreVersionRequest struct {
	PlanName      string `param:"planName"`
	VersionNumber int64  `param:"versionNumber"`
}

// RestoreVersionResponse represents the response after restoring a version.
type RestoreVersionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Error   string `json:"error,omitempty"`
}
