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

// GenerateTLDRPromptArgs contains arguments for the generate_tldr_prompt tool.
type GenerateTLDRPromptArgs struct {
	FileName   string `json:"fileName"`
	SyncSource string `json:"syncSource"`
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

// DiffPlanVersionsArgs contains arguments for the diff_plan_versions tool.
type DiffPlanVersionsArgs struct {
	FileName    string `json:"fileName"`
	SyncSource  string `json:"syncSource"`
	FromVersion int64  `json:"fromVersion"`
	ToVersion   int64  `json:"toVersion"`
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

// DiffResult wraps the unified diff text and which two versions were compared,
// returned by diff_plan_versions.
type DiffResult struct {
	Diff          string `json:"diff"`
	FromVersion   int64  `json:"fromVersion"`
	FromCreatedAt string `json:"fromCreatedAt"`
	ToVersion     int64  `json:"toVersion"`
	ToCreatedAt   string `json:"toCreatedAt"`
}

// TLDRPrompt bundles a plan's TLDR-generation request as the two-part shape any
// LLM call actually takes: SystemPrompt (the fixed instructions) and UserPrompt
// (the plan-specific request, title + content already folded in — mirrors exactly
// what ollama.Connector.Send builds as its Prompt field). generate_tldr_prompt does
// not call an LLM itself — the calling assistant is expected to write the summary
// from these two fields, then optionally persist it via add_comment.
type TLDRPrompt struct {
	FileName     string `json:"fileName"`
	SyncSource   string `json:"syncSource"`
	SystemPrompt string `json:"systemPrompt"`
	UserPrompt   string `json:"userPrompt"`
}

// GetPlanMemoryArgs contains arguments for the get_plan_memory tool.
type GetPlanMemoryArgs struct {
	FileName   string `json:"fileName"`
	SyncSource string `json:"syncSource"`
}

// GenerateMemoryPromptArgs contains arguments for the generate_memory_prompt tool.
type GenerateMemoryPromptArgs struct {
	FileName   string `json:"fileName"`
	SyncSource string `json:"syncSource"`
	// Rebuild asks for every event rather than only those not yet narrated.
	Rebuild bool `json:"rebuild,omitempty"`
}

// SaveMemoryEventArg is one narrative supplied by the caller. Only the event's
// identity and its prose are accepted — timestamps and change stats are
// computed by busquets and ignored if sent.
type SaveMemoryEventArg struct {
	EventKind string `json:"eventKind"` // "version", "restore" or "comment"
	RefID     int64  `json:"refId"`
	Narrative string `json:"narrative"`
}

// SavePlanMemoryArgs contains arguments for the save_plan_memory tool.
type SavePlanMemoryArgs struct {
	FileName   string               `json:"fileName"`
	SyncSource string               `json:"syncSource"`
	Mode       string               `json:"mode,omitempty"` // "incremental" (default) or "rebuild"
	Summary    string               `json:"summary,omitempty"`
	Events     []SaveMemoryEventArg `json:"events"`
}

// DeletePlanMemoryArgs contains arguments for the delete_plan_memory tool.
type DeletePlanMemoryArgs struct {
	FileName   string `json:"fileName"`
	SyncSource string `json:"syncSource"`
}

// MemoryPromptResult is what generate_memory_prompt returns: the prompts plus
// the events still to narrate. No LLM is called and no key is needed — the
// calling assistant writes the narratives and persists them via save_plan_memory.
type MemoryPromptResult struct {
	FileName            string              `json:"fileName"`
	SyncSource          string              `json:"syncSource"`
	PlanTitle           string              `json:"planTitle"`
	Mode                string              `json:"mode"`
	SummarySystemPrompt string              `json:"summarySystemPrompt"`
	SummaryUserPrompt   string              `json:"summaryUserPrompt"`
	EventSystemPrompt   string              `json:"eventSystemPrompt"`
	Events              []MemoryEventPrompt `json:"events"`
}

// MemoryEventPrompt is one event to narrate, with its request.
type MemoryEventPrompt struct {
	EventKind     string `json:"eventKind"`
	RefID         int64  `json:"refId"`
	VersionNumber *int64 `json:"versionNumber,omitempty"`
	OccurredAt    string `json:"occurredAt"`
	LinesAdded    int    `json:"linesAdded"`
	LinesRemoved  int    `json:"linesRemoved"`
	UserPrompt    string `json:"userPrompt"`
}

// PlanMemoryResult wraps a memory and its computed timeline.
type PlanMemoryResult struct {
	FileName            string             `json:"fileName"`
	SyncSource          string             `json:"syncSource"`
	PlanTitle           string             `json:"planTitle"`
	Exists              bool               `json:"exists"`
	Summary             string             `json:"summary,omitempty"`
	Content             string             `json:"content,omitempty"`
	CoversUpToVersion   int64              `json:"coversUpToVersion"`
	CoversUpToCommentID int64              `json:"coversUpToCommentId"`
	GeneratedBy         string             `json:"generatedBy,omitempty"`
	NewVersions         int                `json:"newVersions"`
	NewComments         int                `json:"newComments"`
	Events              []MemoryEventEntry `json:"events"`
}

// MemoryEventEntry is one timeline entry as returned to an MCP caller.
type MemoryEventEntry struct {
	EventKind     string `json:"eventKind"`
	RefID         int64  `json:"refId"`
	VersionNumber *int64 `json:"versionNumber,omitempty"`
	RestoredFrom  *int64 `json:"restoredFrom,omitempty"`
	OccurredAt    string `json:"occurredAt"`
	LinesAdded    int    `json:"linesAdded"`
	LinesRemoved  int    `json:"linesRemoved"`
	WordCount     int    `json:"wordCount"`
}
