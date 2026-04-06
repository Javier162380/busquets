// Package secrets provides a store for sensitive connector configuration.
package secrets

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
)

// Store provides access to secrets (connector tokens, API keys, etc.).
type Store interface {
	Get(ctx context.Context, connectorName, key string) (string, bool, error)
	Set(ctx context.Context, connectorName, key, value string) error
	Delete(ctx context.Context, connectorName, key string) error
	List(ctx context.Context, connectorName string) (map[string]string, error)
}

// DBStore implements Store using the connector_settings table.
type DBStore struct {
	db dto.Repository
}

// NewDBStore creates a new DBStore.
// The db parameter can be any type implementing dto.Repository.
func NewDBStore(db dto.Repository) *DBStore {
	return &DBStore{db: db}
}

// Get retrieves a secret value for a connector.
func (s *DBStore) Get(ctx context.Context, connectorName, key string) (string, bool, error) {
	setting, err := s.db.GetConnectorSetting(ctx, connectorName, key)
	if errors.Is(err, sql.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return setting.SettingValue, true, nil
}

// Set stores a secret value for a connector.
func (s *DBStore) Set(ctx context.Context, connectorName, key, value string) error {
	return s.db.UpsertConnectorSetting(ctx, dto.UpsertConnectorSettingParams{
		ConnectorName: connectorName,
		SettingKey:    key,
		SettingValue:  value,
		IsSecret:      true,
	})
}

// Delete removes a secret for a connector.
func (s *DBStore) Delete(ctx context.Context, connectorName, key string) error {
	return s.db.DeleteConnectorSetting(ctx, connectorName, key)
}

// List returns all secrets for a connector.
func (s *DBStore) List(ctx context.Context, connectorName string) (map[string]string, error) {
	settings, err := s.db.ListConnectorSettings(ctx, connectorName)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string, len(settings))
	for _, setting := range settings {
		result[setting.SettingKey] = setting.SettingValue
	}
	return result, nil
}
