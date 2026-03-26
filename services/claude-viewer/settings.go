package claudeviewer

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/repository"
)

var (
	ErrSettingNotFound    = errors.New("setting not found")
	ErrInvalidDateFormat  = errors.New("invalid datetime format")
	ErrInvalidNumberValue = errors.New("invalid number value")
)

type Setting struct {
	SettingName string
	StringValue *string
	NumberValue *float64
	DateValue   *time.Time
	BoolValue   *bool
}

func (s *Setting) IsString() bool {
	return s.StringValue != nil
}

func (s *Setting) GetStringValue() string {
	if s.StringValue == nil {
		return ""
	}
	return *s.StringValue
}

func (s *Setting) IsNumber() bool {
	return s.NumberValue != nil
}

func (s *Setting) GetNumberValue() float64 {
	if s.NumberValue == nil {
		return 0
	}
	return *s.NumberValue
}

func (s *Setting) IsBoolean() bool {
	return s.BoolValue != nil
}

func (s *Setting) GetBooleanValue() bool {
	if s.BoolValue == nil {
		return false
	}
	return *s.BoolValue
}

func (s *Setting) IsDate() bool {
	return s.DateValue != nil
}

func (s *Setting) GetDateValue() time.Time {
	if s.DateValue == nil {
		return time.Time{}
	}

	return *s.DateValue
}

func (s *Service) GetSetting(ctx context.Context, variableName string) (Setting, bool, error) {
	repoSetting, err := s.db.GetSettingByName(ctx, variableName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Setting{}, false, ErrSettingNotFound
		}
		return Setting{}, false, fmt.Errorf("%w: %s", ErrSettingNotFound, variableName)
	}

	setting := Setting{
		SettingName: repoSetting.VariableName,
	}

	switch {
	case repoSetting.StringValue.Valid:
		setting.StringValue = &repoSetting.StringValue.String

	case repoSetting.NumberValue.Valid:
		setting.NumberValue = &repoSetting.NumberValue.Float64

	case repoSetting.BooleanValue.Valid:
		setting.BoolValue = &repoSetting.BooleanValue.Bool

	case repoSetting.DatetimeValue.Valid:
		setting.DateValue = &repoSetting.DatetimeValue.Time
	}
	return setting, true, nil
}

func (s *Service) GetStringValue(ctx context.Context, variableName string) (string, bool, error) {
	setting, err := s.db.GetSettingByName(ctx, variableName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", false, ErrSettingNotFound
		}
		return "", false, fmt.Errorf("failed to get setting: %w", err)
	}

	if !setting.StringValue.Valid {
		return "", true, nil
	}
	return setting.StringValue.String, true, nil
}

// GetNumberValue returns a numeric setting value by name.
// Returns (value, exists, error).
func (s *Service) GetNumberValue(ctx context.Context, variableName string) (float64, bool, error) {
	setting, err := s.db.GetSettingByName(ctx, variableName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, false, nil
		}
		return 0, false, fmt.Errorf("failed to get setting: %w", err)
	}

	if !setting.NumberValue.Valid {
		return 0, true, nil
	}
	return setting.NumberValue.Float64, true, nil
}

// GetBooleanValue returns a boolean setting value by name.
// Returns (value, exists, error).
func (s *Service) GetBooleanValue(ctx context.Context, variableName string) (bool, bool, error) {
	setting, err := s.db.GetSettingByName(ctx, variableName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("failed to get setting: %w", err)
	}

	if !setting.BooleanValue.Valid {
		return false, true, nil
	}
	return setting.BooleanValue.Bool, true, nil
}

// GetDateTimeValue returns a datetime setting value by name.
// Returns (value, exists, error).
func (s *Service) GetDateTimeValue(ctx context.Context, variableName string) (time.Time, bool, error) {
	setting, err := s.db.GetSettingByName(ctx, variableName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return time.Time{}, false, nil
		}
		return time.Time{}, false, fmt.Errorf("failed to get setting: %w", err)
	}

	if !setting.DatetimeValue.Valid {
		return time.Time{}, true, nil
	}
	return setting.DatetimeValue.Time, true, nil
}

// SetSetting stores or updates a setting with validation based on variable type.
func (s *Service) SetSetting(ctx context.Context, varName, varType string, values SettingValues) error {
	switch varType {
	case "datetime":
	case "number":
		if values.NumberValue != nil {
			if err := validateNumberValue(*values.NumberValue); err != nil {
				return err
			}
		}
	case "string":
	case "boolean":
	default:
		return fmt.Errorf("unknown variable type: %s", varType)
	}

	params := repository.UpsertSettingParams{
		VariableName: varName,
		VariableType: varType,
		StringValue: sql.NullString{
			String: "",
			Valid:  values.StringValue != nil,
		},
		NumberValue: sql.NullFloat64{
			Float64: 0,
			Valid:   values.NumberValue != nil,
		},
		BooleanValue: sql.NullBool{
			Bool:  false,
			Valid: values.BooleanValue != nil,
		},
		DatetimeValue: sql.NullTime{
			Time:  time.Time{},
			Valid: values.DateTimeValue != nil,
		},
	}

	if values.StringValue != nil {
		params.StringValue.String = *values.StringValue
	}
	if values.NumberValue != nil {
		params.NumberValue.Float64 = *values.NumberValue
	}
	if values.BooleanValue != nil {
		params.BooleanValue.Bool = *values.BooleanValue
	}
	if values.DateTimeValue != nil {
		params.DatetimeValue.Time = *values.DateTimeValue
	}

	if err := s.db.UpsertSetting(ctx, params); err != nil {
		return fmt.Errorf("failed to upsert setting: %w", err)
	}

	return nil
}

// GetReadingSpeedForDisplay returns the current reading speed WPM setting
// from the database, with fallback to default if not set.
func (s *Service) GetReadingSpeedForDisplay(ctx context.Context) int {
	wpm, exists, err := s.GetNumberValue(ctx, "reading_speed_wpm")
	if err != nil || !exists {
		return DefaultReadingSpeedWPM
	}
	return int(wpm)
}

// IsDarkModeEnabled returns whether dark mode is enabled.
// Returns false if not set or on error.
func (s *Service) IsDarkModeEnabled(ctx context.Context) bool {
	enabled, exists, err := s.GetBooleanValue(ctx, "dark_mode_enabled")
	if err != nil || !exists {
		return false
	}
	return enabled
}

// ValidateDateTimeValue validates ISO 8601 format datetime strings.
// Returns parsed time.Time or error.
func ValidateDateTimeValue(value string) (time.Time, error) {
	// Try ISO 8601 formats
	formats := []string{
		time.RFC3339,          // "2024-03-15T10:30:00Z"
		time.RFC3339Nano,      // "2024-03-15T10:30:00.000Z"
		"2006-01-02",          // "2024-03-15"
		"2006-01-02T15:04:05", // "2024-03-15T10:30:00"
	}

	for _, format := range formats {
		t, err := time.Parse(format, value)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("%w: expected ISO 8601 format (e.g., 2024-03-15T10:30:00Z)", ErrInvalidDateFormat)
}

// validateNumberValue validates numeric values with optional bounds.
func validateNumberValue(value float64) error {
	// Add specific validation for reading speed WPM if needed
	// For now, just ensure it's a reasonable positive number
	if value <= 0 {
		return fmt.Errorf("number value must be positive")
	}
	return nil
}
