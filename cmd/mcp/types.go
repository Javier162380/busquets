package mcp

// SearchPlansArgs contains arguments for the search_plans tool.
type SearchPlansArgs struct {
	Query    string   `json:"query"`    // Text search query (optional)
	Tags     []string `json:"tags"`     // Tag names to filter (optional)
	MatchAll bool     `json:"matchAll"` // AND vs OR for tags (default: false)
	Limit    int64    `json:"limit"`    // Max results (default: 20, max: 50)
}

// GetPlanArgs contains arguments for the get_plan tool.
type GetPlanArgs struct {
	FileName   string `json:"fileName"`   // Plan filename (e.g., "my-plan.md")
	SyncSource string `json:"syncSource"` // Source label as shown in sync_label of search_plans results (e.g., "personal")
}
