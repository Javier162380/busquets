package dto

import planviewer "github.com/Javier162380/claude-plan-viewer"

// ConnectorRole is re-exported from the root package as a type alias so
// existing call sites continue to compile without modification.
type ConnectorRole = planviewer.ConnectorRole

const (
	ConnectorRoleTransmit = planviewer.ConnectorRoleTransmit
	ConnectorRoleSummary  = planviewer.ConnectorRoleSummary
)
