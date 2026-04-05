// Package secrets provides a store for sensitive connector configuration.
package secrets

import (
	"context"
	"database/sql"
	"errors"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository"
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
	db repository.Querier
}

// NewDBStore creates a new DBStore.
// The db parameter can be any type implementing repository.Querier.
func NewDBStore(db repository.Querier) *DBStore {
	return &DBStore{db: db}
}

// Get retrieves a secret value for a connector.
func (s *DBStore) Get(ctx context.Context, connectorName, key string) (string, bool, error) {
	setting, err := s.db.GetConnectorSetting(ctx, repository.GetConnectorSettingParams{
		ConnectorName: connectorName,
		SettingKey:    key,
	})
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
	return s.db.UpsertConnectorSetting(ctx, repository.UpsertConnectorSettingParams{
		ConnectorName: connectorName,
		SettingKey:    key,
		SettingValue:  value,
		IsSecret:      true,
	})
}

// Delete removes a secret for a connector.
func (s *DBStore) Delete(ctx context.Context, connectorName, key string) error {
	return s.db.DeleteConnectorSetting(ctx, repository.DeleteConnectorSettingParams{
		ConnectorName: connectorName,
		SettingKey:    key,
	})
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
