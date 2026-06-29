package connectors

import (
	"context"
	"fmt"
	"time"

	planviewer "github.com/Javier162380/claude-plan-viewer"
	"github.com/Javier162380/claude-plan-viewer/internal/cache"
)

const (
	validationCacheTTL = 5 * time.Minute
	validationCacheGC  = 10 * time.Minute
)

// Manager orchestrates connector operations.
type Manager struct {
	registry        *Registry
	db              planviewer.Store
	validationCache *cache.MuxCache[bool]
}

// NewManager creates a new connector manager.
func NewManager(registry *Registry, db planviewer.Store) *Manager {
	return &Manager{
		registry:        registry,
		db:              db,
		validationCache: cache.New[bool](validationCacheTTL, validationCacheGC),
	}
}

// Close stops the background cache GC goroutine.
func (m *Manager) Close() {
	m.validationCache.Stop()
}

// Execute dispatches a request to the connector assigned to req.Role.
func (m *Manager) Execute(ctx context.Context, req ConnectorRequest) (*ConnectorResult, error) {
	name, found, err := m.db.GetConnectorForRole(ctx, req.Role)
	if err != nil {
		return nil, fmt.Errorf("failed to get connector for role %q: %w", req.Role, err)
	}
	if !found || name == "" {
		return nil, planviewer.ErrNoConnectorEnabled
	}
	connector, ok := m.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("connector %q not found in registry", name)
	}
	if err := m.ensureValid(ctx, connector); err != nil {
		return nil, err
	}
	return connector.Execute(ctx, req)
}

// ensureValid loads config and validates on cache miss; skips both on a warm hit.
func (m *Manager) ensureValid(ctx context.Context, c Connector) error {
	if _, err := m.validationCache.Get(c.Name()); err == nil {
		return nil
	}
	if cfg, ok := c.(ConfigurableConnector); ok {
		if err := cfg.LoadConfig(ctx, m); err != nil {
			return fmt.Errorf("failed to load config for %q: %w", c.Name(), err)
		}
	}
	if err := c.Validate(); err != nil {
		return fmt.Errorf("connector %q validation failed: %w", c.Name(), err)
	}
	m.validationCache.Set(c.Name(), true)
	return nil
}

// GetEnabledConnector returns the transmit connector, if any.
func (m *Manager) GetEnabledConnector(ctx context.Context) (Connector, error) {
	name, found, err := m.db.GetConnectorForRole(ctx, planviewer.ConnectorRoleTransmit)
	if err != nil {
		return nil, fmt.Errorf("failed to get transmit connector: %w", err)
	}
	if !found || name == "" {
		return nil, nil
	}

	connector, ok := m.registry.Get(name)
	if !ok {
		return nil, fmt.Errorf("transmit connector %q not found in registry", name)
	}

	if cfg, ok := connector.(ConfigurableConnector); ok {
		if err := cfg.LoadConfig(ctx, m); err != nil {
			return nil, fmt.Errorf("failed to load config for %q: %w", name, err)
		}
	}

	return connector, nil
}

// EnableConnector sets a connector as the transmit connector.
func (m *Manager) EnableConnector(ctx context.Context, name string) error {
	connector, ok := m.registry.Get(name)
	if !ok {
		return planviewer.ErrConnectorNotFound
	}

	if err := m.db.UpsertConnector(ctx, name, connector.DisplayName(), false); err != nil {
		return fmt.Errorf("failed to upsert connector: %w", err)
	}

	if err := m.db.SetConnectorForRole(ctx, name, planviewer.ConnectorRoleTransmit); err != nil {
		return fmt.Errorf("failed to set transmit connector: %w", err)
	}

	return nil
}

// DisableConnector clears the transmit connector slot.
func (m *Manager) DisableConnector(ctx context.Context) error {
	return m.db.ClearConnectorForRole(ctx, planviewer.ConnectorRoleTransmit)
}

// SetSummaryConnector assigns a connector to the summary slot.
func (m *Manager) SetSummaryConnector(ctx context.Context, name string) error {
	connector, ok := m.registry.Get(name)
	if !ok {
		return planviewer.ErrConnectorNotFound
	}

	if err := m.db.UpsertConnector(ctx, name, connector.DisplayName(), false); err != nil {
		return fmt.Errorf("failed to upsert connector: %w", err)
	}

	if err := m.db.SetConnectorForRole(ctx, name, planviewer.ConnectorRoleSummary); err != nil {
		return fmt.Errorf("failed to set summary connector: %w", err)
	}

	return nil
}

// SetConnectorSetting saves a setting and invalidates the validation cache so the
// next Execute reloads config from the DB.
func (m *Manager) SetConnectorSetting(ctx context.Context, connectorName, key, value string, isSecret bool) error {
	if err := m.db.UpsertConnectorSetting(ctx, connectorName, key, value, isSecret); err != nil {
		return err
	}
	m.validationCache.Delete(connectorName)
	return nil
}

// GetConnectorSetting retrieves a setting for a specific connector.
func (m *Manager) GetConnectorSetting(ctx context.Context, connectorName, key string) (string, bool, error) {
	return m.db.GetConnectorSetting(ctx, connectorName, key)
}

// Send sends content through the transmit connector (kept for backward compatibility with service layer).
func (m *Manager) Send(ctx context.Context, title, content string) (*ConnectorResult, error) {
	return m.Execute(ctx, ConnectorRequest{
		Role:     planviewer.ConnectorRoleTransmit,
		Transmit: &TransmitPayload{Title: title, Content: content},
	})
}

// GenerateSummary invokes the summary connector and returns its generated text response.
func (m *Manager) GenerateSummary(ctx context.Context, title, content string) (string, error) {
	result, err := m.Execute(ctx, ConnectorRequest{
		Role:    planviewer.ConnectorRoleSummary,
		Summary: &SummaryPayload{Title: title, Content: content},
	})
	if err != nil {
		return "", err
	}
	if result.Text == nil {
		return "", planviewer.ErrConnectorResponseEmpty
	}
	return *result.Text, nil
}

// ListAvailable returns all registered connectors with their status.
func (m *Manager) ListAvailable(ctx context.Context) ([]ConnectorStatus, error) {
	connectors := m.registry.All()
	statuses := make([]ConnectorStatus, len(connectors))

	transmitName, _, err := m.db.GetConnectorForRole(ctx, planviewer.ConnectorRoleTransmit)
	if err != nil {
		return nil, fmt.Errorf("failed to get transmit connector role: %w", err)
	}
	summaryName, _, err := m.db.GetConnectorForRole(ctx, planviewer.ConnectorRoleSummary)
	if err != nil {
		return nil, fmt.Errorf("failed to get summary connector role: %w", err)
	}

	for i, c := range connectors {
		var role *planviewer.ConnectorRole
		switch c.Name() {
		case transmitName:
			r := planviewer.ConnectorRoleTransmit
			role = &r
		case summaryName:
			r := planviewer.ConnectorRoleSummary
			role = &r
		}
		statuses[i] = ConnectorStatus{
			Name:        c.Name(),
			DisplayName: c.DisplayName(),
			Role:        role,
			Configured:  m.isConfigured(ctx, c),
		}
	}
	return statuses, nil
}

// isConfigured checks if all required settings are present for a connector.
func (m *Manager) isConfigured(ctx context.Context, c Connector) bool {
	for _, def := range c.RequiredSettings() {
		if def.Required {
			_, exists, _ := m.GetConnectorSetting(ctx, c.Name(), def.Key)
			if !exists {
				return false
			}
		}
	}
	return true
}

// EnsureConnectorExists ensures a connector record exists in the database.
func (m *Manager) EnsureConnectorExists(ctx context.Context, name string) error {
	connector, ok := m.registry.Get(name)
	if !ok {
		return planviewer.ErrConnectorNotFound
	}

	return m.db.UpsertConnector(ctx, name, connector.DisplayName(), false)
}

// GetConnectorRequiredSettings returns the required settings for a connector.
func (m *Manager) GetConnectorRequiredSettings(connectorName string) ([]SettingDefinition, error) {
	connector, ok := m.registry.Get(connectorName)
	if !ok {
		return nil, planviewer.ErrConnectorNotFound
	}
	return connector.RequiredSettings(), nil
}

// ValidateConnector validates a connector's configuration.
func (m *Manager) ValidateConnector(ctx context.Context, connectorName string) error {
	connector, ok := m.registry.Get(connectorName)
	if !ok {
		return planviewer.ErrConnectorNotFound
	}
	return m.ensureValid(ctx, connector)
}
