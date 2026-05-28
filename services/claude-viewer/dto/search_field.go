package dto

import planviewer "github.com/Javier162380/claude-plan-viewer"

// SearchField is re-exported from the root package as a type alias so
// the repository layer always uses dto.* types without a direct root import.
type SearchField = planviewer.SearchField

const (
	SearchOverAll      = planviewer.SearchOverAll
	SearchOverPlanName = planviewer.SearchOverPlanName
	SearchOverContent  = planviewer.SearchOverContent
	DefaultSearchOver  = planviewer.DefaultSearchOver
)
