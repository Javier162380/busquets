package mcp

import "github.com/Javier162380/busquets/services/busquets"

// SearchPlansArgs contains arguments for the search_plans tool.
type SearchPlansArgs struct {
	Query    string   `json:"query,omitempty"`    // Text search query (optional)
	Tags     []string `json:"tags,omitempty"`     // Tag names to filter (optional)
	MatchAll bool     `json:"matchAll,omitempty"` // AND vs OR for tags (default: false)
	Limit    int64    `json:"limit,omitempty"`    // Max results (default: 20, max: 50)
}

// GetPlanArgs contains arguments for the get_plan tool.
type GetPlanArgs struct {
	FileName   string `json:"fileName"`   // Plan filename (e.g., "my-plan.md")
	SyncSource string `json:"syncSource"` // Source directory path, as shown in sync_source of search_plans results
}

// AddPlanCommentArgs contains arguments for the add_plan_comment tool.
type AddPlanCommentArgs struct {
	FileName   string `json:"fileName"`
	SyncSource string `json:"syncSource"`
	Comment    string `json:"comment"`
}

// GetPlanCommentsArgs contains arguments for the get_plan_comments tool.
type GetPlanCommentsArgs struct {
	FileName   string `json:"fileName"`
	SyncSource string `json:"syncSource"`
}

// DeleteCommentArgs contains arguments for the delete_comment tool.
type DeleteCommentArgs struct {
	CommentID int64 `json:"commentId"`
}

// GetPlanVersionHistoryArgs contains arguments for the get_plan_version_history tool.
type GetPlanVersionHistoryArgs struct {
	FileName   string `json:"fileName"`
	SyncSource string `json:"syncSource"`
	Offset     int64  `json:"offset,omitempty"`
	Limit      int64  `json:"limit,omitempty"` // default 20, max 50
}

// GetPlanVersionArgs contains arguments for the get_plan_version tool.
type GetPlanVersionArgs struct {
	FileName      string `json:"fileName"`
	SyncSource    string `json:"syncSource"`
	VersionNumber int64  `json:"versionNumber"`
}

// RestorePlanVersionArgs contains arguments for the restore_plan_version tool.
type RestorePlanVersionArgs struct {
	FileName      string `json:"fileName"`
	SyncSource    string `json:"syncSource"`
	VersionNumber int64  `json:"versionNumber"`
}

// DeletePlanTagArgs contains arguments for the delete_tag tool (removes one tag from one plan).
type DeletePlanTagArgs struct {
	FileName   string `json:"fileName"`
	SyncSource string `json:"syncSource"`
	Tag        string `json:"tag"`
}

// SetPlanTagsArgs contains arguments for the set_plan_tags tool.
// Replaces the plan's full tag list — not additive.
type SetPlanTagsArgs struct {
	FileName   string   `json:"fileName"`
	SyncSource string   `json:"syncSource"`
	Tags       []string `json:"tags"`
}

// GetPlanTagsArgs contains arguments for the get_plan_tags tool.
type GetPlanTagsArgs struct {
	FileName   string `json:"fileName"`
	SyncSource string `json:"syncSource"`
}

// Result wrapper types: a bare slice isn't a valid MCP output schema (must be struct/map,
// per the go-sdk's schema inference). Each slice-returning handler wraps its data in one
// of these so the tool gets a real, useful output schema.

// PlanCommentsResult wraps the comment list returned by get_plan_comments.
type PlanCommentsResult struct {
	Comments []busquets.Comment `json:"comments"`
}

// TagsResult wraps the tag list returned by get_all_tags, get_plan_tags, set_plan_tags, and delete_tag.
type TagsResult struct {
	Tags []busquets.Tag `json:"tags"`
}

// PlanVersionHistoryResult wraps the version list returned by get_plan_version_history.
type PlanVersionHistoryResult struct {
	Versions []busquets.PlanVersionDetail `json:"versions"`
}

// PlanSearchResult wraps the plan list returned by search_plans.
type PlanSearchResult struct {
	Plans []busquets.PlanSummary `json:"plans"`
}
