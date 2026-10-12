package dto

import domain "github.com/Javier162380/busquets"

// ConnectorRole is re-exported from the root package as a type alias so
// existing call sites continue to compile without modification.
type ConnectorRole = domain.ConnectorRole

const (
	ConnectorRoleTransmit = domain.ConnectorRoleTransmit
	ConnectorRoleSummary  = domain.ConnectorRoleSummary
)
