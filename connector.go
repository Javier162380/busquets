// Package planviewer defines the shared connector domain types used by both
// the internal connector plugin system and the service layer, breaking the
// otherwise-inverted dependency from internal/ into services/.
package planviewer

import (
	"context"
	"errors"
)

// ConnectorRole identifies which functional slot a connector is assigned to.
type ConnectorRole string

const (
	ConnectorRoleTransmit ConnectorRole = "transmit_connector"
	ConnectorRoleSummary  ConnectorRole = "summary_connector"
)

// Store is the narrow database interface the connector Manager needs.
// Method signatures use only primitives and ConnectorRole so that no
// repository-layer types (params structs, models) leak into the root package.
// Both sqlite.Repository and postgres.Repository satisfy this interface via
// Go's implicit structural typing — no explicit declaration needed.
type Store interface {
	GetConnectorForRole(ctx context.Context, role ConnectorRole) (string, bool, error)
	SetConnectorForRole(ctx context.Context, name string, role ConnectorRole) error
	ClearConnectorForRole(ctx context.Context, role ConnectorRole) error
	UpsertConnector(ctx context.Context, name, displayName string, enabled bool) error
	GetConnectorSetting(ctx context.Context, connectorName, key string) (string, bool, error)
	UpsertConnectorSetting(ctx context.Context, connectorName, key, value string, isSecret bool) error
	DeleteConnectorSetting(ctx context.Context, connectorName, key string) error
	DeleteAllConnectorSettings(ctx context.Context, connectorName string) error
}

var (
	ErrConnectorNotFound      = errors.New("connector not found")
	ErrConnectorDisabled      = errors.New("connector not initialized")
	ErrNoConnectorEnabled     = errors.New("no connector enabled")
	ErrNoSummarizerConfigured = errors.New("no summarizer configured")
	ErrConnectorResponseEmpty = errors.New("connector returned no response")
)

// IsConnectorNotFound reports whether err is a connector-not-found error.
func IsConnectorNotFound(err error) bool {
	return errors.Is(err, ErrConnectorNotFound)
}
