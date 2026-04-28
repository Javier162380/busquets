package claudeviewer

import (
	"context"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// SendToConnector sends a plan to the enabled connector.
func (s *Service) SendToConnector(ctx context.Context, planFileName string) error {
	if s.connectorManager == nil {
		return dto.ErrConnectorDisabled
	}

	// Get the plan content
	plan, err := s.GetPlanDetailByFileName(ctx, planFileName)
	if err != nil {
		return err
	}

	// Send through connector manager
	result, err := s.connectorManager.Send(ctx, plan.Title, plan.Content)
	if err != nil {
		return err
	}
	if !result.Success {
		return result.Error
	}

	return nil
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

	return &ConnectorInfo{
		Name:        connector.Name(),
		DisplayName: connector.DisplayName(),
		Enabled:     true,
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
			Enabled:     status.Enabled,
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
		value, _, _ := s.connectorManager.GetConnectorSetting(ctx, connectorName, def.Key)

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
