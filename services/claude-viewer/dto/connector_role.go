package dto

// ConnectorRole identifies which slot a connector is assigned to.
type ConnectorRole string

const (
	// ConnectorRoleTransmit is the connector used by the transmit command.
	ConnectorRoleTransmit ConnectorRole = "transmit_connector"

	// ConnectorRoleSummary is the connector used to generate TLDR summaries.
	ConnectorRoleSummary ConnectorRole = "summary_connector"
)
