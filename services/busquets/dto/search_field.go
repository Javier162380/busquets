package dto

import domain "github.com/Javier162380/busquets"

// SearchField is re-exported from the root package as a type alias so
// the repository layer always uses dto.* types without a direct root import.
type SearchField = domain.SearchField

const (
	SearchOverAll      = domain.SearchOverAll
	SearchOverPlanName = domain.SearchOverPlanName
	SearchOverContent  = domain.SearchOverContent
	DefaultSearchOver  = domain.DefaultSearchOver
)
