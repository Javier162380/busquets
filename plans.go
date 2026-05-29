package planviewer

// SearchField identifies which plan fields to search over.
type SearchField string

const (
	SearchOverAll      SearchField = "all"       // title OR content (default)
	SearchOverPlanName SearchField = "plan_name" // title only
	SearchOverContent  SearchField = "content"   // content only
	DefaultSearchOver              = SearchOverAll
)
