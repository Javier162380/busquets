package claudeviewer

import (
	"context"
	"errors"

	planviewer "github.com/Javier162380/claude-plan-viewer"
	"github.com/Javier162380/claude-plan-viewer/internal/connectors"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// ConnectorInfo represents connector status.
type ConnectorInfo struct {
	Name        string
	DisplayName string
	Role        *dto.ConnectorRole
	Configured  bool
}

// IsTransmit reports whether this connector is assigned to the transmit slot.
func (c ConnectorInfo) IsTransmit() bool {
	return c.Role != nil && *c.Role == dto.ConnectorRoleTransmit
}

// IsSummarizer reports whether this connector is assigned to the summary slot.
func (c ConnectorInfo) IsSummarizer() bool {
	return c.Role != nil && *c.Role == dto.ConnectorRoleSummary
}

// ConnectorSettingInfo represents a connector setting with its current value.
type ConnectorSettingInfo struct {
	Key         string
	DisplayName string
	Description string
	Value       string
	Required    bool
	Sensitive   bool
}

// SendToConnector sends a plan to the transmit connector.
func (s *Service) SendToConnector(ctx context.Context, planFileName, syncSource string) error {
	if s.connectorManager == nil {
		return dto.ErrConnectorDisabled
	}

	plan, err := s.GetPlanDetailByFileName(ctx, planFileName, syncSource)
	if err != nil {
		return err
	}

	_, err = s.connectorManager.Execute(ctx, connectors.ConnectorRequest{
		Role:     connectors.ConnectorRoleTransmit,
		Transmit: &connectors.TransmitPayload{Title: plan.Title, Content: plan.Content},
	})
	return err
}

// GetEnabledConnector returns the currently enabled connector info.
func (s *Service) GetEnabledConnector(ctx context.Context) (*ConnectorInfo, error) {
	if s.connectorManager == nil {
		return nil, nil
	}

	connector, err := s.connectorManager.GetEnabledConnector(ctx)
	if err != nil {
		return nil, err
	}
	if connector == nil {
		return nil, nil
	}

	role := dto.ConnectorRoleTransmit
	return &ConnectorInfo{
		Name:        connector.Name(),
		DisplayName: connector.DisplayName(),
		Role:        &role,
		Configured:  true,
	}, nil
}

// ListConnectors returns all available connectors with their status.
func (s *Service) ListConnectors(ctx context.Context) ([]ConnectorInfo, error) {
	if s.connectorManager == nil {
		return nil, nil
	}

	statuses, err := s.connectorManager.ListAvailable(ctx)
	if err != nil {
		return nil, err
	}

	infos := make([]ConnectorInfo, len(statuses))
	for i, status := range statuses {
		infos[i] = ConnectorInfo{
			Name:        status.Name,
			DisplayName: status.DisplayName,
			Role:        status.Role,
			Configured:  status.Configured,
		}
	}
	return infos, nil
}

// EnableConnector enables a specific connector.
func (s *Service) EnableConnector(ctx context.Context, name string) error {
	if s.connectorManager == nil {
		return dto.ErrConnectorDisabled
	}
	return s.connectorManager.EnableConnector(ctx, name)
}

// DisableConnector disables all connectors.
func (s *Service) DisableConnector(ctx context.Context) error {
	if s.connectorManager == nil {
		return dto.ErrConnectorDisabled
	}
	return s.connectorManager.DisableConnector(ctx)
}

// ConfigureConnector sets a configuration value for a connector.
func (s *Service) ConfigureConnector(ctx context.Context, connectorName, key, value string, isSecret bool) error {
	if s.connectorManager == nil {
		return dto.ErrConnectorDisabled
	}
	return s.connectorManager.SetConnectorSetting(ctx, connectorName, key, value, isSecret)
}

// GetConnectorSettings returns the settings for a connector with their current values.
func (s *Service) GetConnectorSettings(ctx context.Context, connectorName string) ([]ConnectorSettingInfo, error) {
	if s.connectorManager == nil {
		return nil, dto.ErrConnectorDisabled
	}

	// Get required settings definitions from the connector
	definitions, err := s.connectorManager.GetConnectorRequiredSettings(connectorName)
	if err != nil {
		return nil, err
	}

	// Build result with current values
	result := make([]ConnectorSettingInfo, len(definitions))
	for i, def := range definitions {
		value, _, conErr := s.connectorManager.GetConnectorSetting(ctx, connectorName, def.Key)
		if conErr != nil {
			return nil, conErr
		}

		result[i] = ConnectorSettingInfo{
			Key:         def.Key,
			DisplayName: def.DisplayName,
			Description: def.Description,
			Value:       value,
			Required:    def.Required,
			Sensitive:   def.Sensitive,
		}
	}

	return result, nil
}

// ValidateConnector validates a connector's configuration.
func (s *Service) ValidateConnector(ctx context.Context, connectorName string) error {
	if s.connectorManager == nil {
		return dto.ErrConnectorDisabled
	}
	return s.connectorManager.ValidateConnector(ctx, connectorName)
}

// GenerateSummary generates a TLDR summary of a plan using the configured summarizer connector.
// On a cache hit the result is returned immediately without calling the connector.
func (s *Service) GenerateSummary(ctx context.Context, planFileName, syncSource string) (string, error) {
	if s.connectorManager == nil {
		return "", dto.ErrConnectorDisabled
	}
	cacheKey := planFileName + ":" + syncSource
	if cached, err := s.summaryCache.Get(cacheKey); err == nil {
		return cached, nil
	}
	plan, err := s.GetPlanDetailByFileName(ctx, planFileName, syncSource)
	if err != nil {
		return "", err
	}
	result, err := s.connectorManager.Execute(ctx, connectors.ConnectorRequest{
		Role:    connectors.ConnectorRoleSummary,
		Summary: &connectors.SummaryPayload{Title: plan.Title, Content: plan.Content},
	})
	if err != nil {
		if errors.Is(err, planviewer.ErrNoConnectorEnabled) {
			return "", planviewer.ErrNoSummarizerConfigured
		}
		return "", err
	}
	if result.Text == nil {
		return "", planviewer.ErrConnectorResponseEmpty
	}
	s.summaryCache.Set(cacheKey, *result.Text)
	return *result.Text, nil
}

// RegenerateSummary bypasses the cache, calls the connector, and updates the cache entry.
func (s *Service) RegenerateSummary(ctx context.Context, planFileName, syncSource string) (string, error) {
	if s.connectorManager == nil {
		return "", dto.ErrConnectorDisabled
	}
	plan, err := s.GetPlanDetailByFileName(ctx, planFileName, syncSource)
	if err != nil {
		return "", err
	}
	result, err := s.connectorManager.Execute(ctx, connectors.ConnectorRequest{
		Role:    connectors.ConnectorRoleSummary,
		Summary: &connectors.SummaryPayload{Title: plan.Title, Content: plan.Content},
	})
	if err != nil {
		if errors.Is(err, planviewer.ErrNoConnectorEnabled) {
			return "", planviewer.ErrNoSummarizerConfigured
		}
		return "", err
	}
	if result.Text == nil {
		return "", planviewer.ErrConnectorResponseEmpty
	}
	cacheKey := planFileName + ":" + syncSource
	s.summaryCache.Set(cacheKey, *result.Text)
	return *result.Text, nil
}

// SetSummaryConnector assigns a connector to the summary slot.
func (s *Service) SetSummaryConnector(ctx context.Context, connectorName string) error {
	if s.connectorManager == nil {
		return dto.ErrConnectorDisabled
	}
	return s.connectorManager.SetSummaryConnector(ctx, connectorName)
}

// ClearSummaryConnector clears the summary connector slot.
func (s *Service) ClearSummaryConnector(ctx context.Context) error {
	if s.connectorManager == nil {
		return dto.ErrConnectorDisabled
	}
	return s.db.ClearConnectorForRole(ctx, dto.ConnectorRoleSummary)
}
