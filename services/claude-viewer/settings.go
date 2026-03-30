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

// Setting type constants.
const (
	SettingTypeNumber   = "number"
	SettingTypeBoolean  = "boolean"
	SettingTypeString   = "string"
	SettingTypeDatetime = "datetime"
)

// Known setting name constants.
const (
	SettingReadingSpeedWPM = "reading_speed_wpm"
	SettingDarkModeEnabled = "dark_mode_enabled"
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

// SetSetting stores or updates a setting. Type is inferred from which field in values is set.
func (s *Service) SetSetting(ctx context.Context, varName string, values SettingValues) error {
	// Infer type from values.
	var varType string
	switch {
	case values.NumberValue != nil:
		varType = SettingTypeNumber
		if err := validateNumberValue(*values.NumberValue); err != nil {
			return err
		}
	case values.BooleanValue != nil:
		varType = SettingTypeBoolean
	case values.StringValue != nil:
		varType = SettingTypeString
	case values.DateTimeValue != nil:
		varType = SettingTypeDatetime
	default:
		return fmt.Errorf("no value provided in SettingValues")
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
	setting, exists, err := s.GetSetting(ctx, SettingReadingSpeedWPM)
	if err != nil || !exists || !setting.IsNumber() {
		return DefaultReadingSpeedWPM
	}
	return int(setting.GetNumberValue())
}

// IsDarkModeEnabled returns whether dark mode is enabled.
// Returns true (default) if not set or on error.
func (s *Service) IsDarkModeEnabled(ctx context.Context) bool {
	setting, exists, err := s.GetSetting(ctx, SettingDarkModeEnabled)
	if err != nil || !exists || !setting.IsBoolean() {
		return true // Default to dark mode
	}
	return setting.GetBooleanValue()
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
