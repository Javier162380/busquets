package claudeviewer

import (
	"context"
	"fmt"
)

// SendToConnector sends a plan to the enabled connector.
func (s *Service) SendToConnector(ctx context.Context, planFileName string) error {
	if s.connectorManager == nil {
		return fmt.Errorf("connector manager not initialized")
	}

	// Get the plan content
	plan, err := s.GetPlanDetailByFileName(ctx, planFileName)
	if err != nil {
		return fmt.Errorf("failed to get plan: %w", err)
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
		return fmt.Errorf("connector manager not initialized")
	}
	return s.connectorManager.EnableConnector(ctx, name)
}

// DisableConnector disables all connectors.
func (s *Service) DisableConnector(ctx context.Context) error {
	if s.connectorManager == nil {
		return fmt.Errorf("connector manager not initialized")
	}
	return s.connectorManager.DisableConnector(ctx)
}

// ConfigureConnector sets a configuration value for a connector.
func (s *Service) ConfigureConnector(ctx context.Context, connectorName, key, value string, isSecret bool) error {
	if s.connectorManager == nil {
		return fmt.Errorf("connector manager not initialized")
	}
	return s.connectorManager.SetConnectorSetting(ctx, connectorName, key, value, isSecret)
}
