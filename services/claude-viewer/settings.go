package claudeviewer

import (
	"context"
	"fmt"
	"time"

	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"
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
	SettingReadingSpeedWPM         = "reading_speed_wpm"
	SettingDarkModeEnabled         = "dark_mode_enabled"
	SettingRenderMarkdownByDefault = "render_markdown_by_default"
	SettingWatchModeEnabled        = "watch_mode_enabled"
	SettingWatchIntervalSeconds    = "watch_interval_seconds"
	SettingDefaultDisplayMode      = "default_display_mode"
	SettingSummaryConnector        = "summary_connector_name"
)

// Display mode values for SettingDefaultDisplayMode.
const (
	DisplayModePlanContent    = "plan_content"     //nolint:gci    // two-panel: list + content (default)
	DisplayModeTagPlanContent = "tag_plan_content" // three-panel: tags + list + content
)

// DefaultWatchIntervalSeconds is the default interval for the watch mode background sync.
const DefaultWatchIntervalSeconds = 5.0

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
	domainSetting, err := s.db.GetSettingByName(ctx, variableName)
	if err != nil {
		if dto.IsNotFound(err) {
			return Setting{}, false, dto.ErrNotFound
		}
		return Setting{}, false, err
	}

	setting := Setting{
		SettingName: domainSetting.VariableName,
		StringValue: domainSetting.StringValue,
		NumberValue: domainSetting.NumberValue,
		BoolValue:   domainSetting.BooleanValue,
		DateValue:   domainSetting.DatetimeValue,
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
		return dto.ErrNoValue
	}

	params := dto.UpsertSettingParams{
		VariableName:  varName,
		VariableType:  varType,
		StringValue:   values.StringValue,
		NumberValue:   values.NumberValue,
		BooleanValue:  values.BooleanValue,
		DatetimeValue: values.DateTimeValue,
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

	return time.Time{}, dto.ErrInvalidDateFormat
}

// validateNumberValue validates numeric values with optional bounds.
func validateNumberValue(value float64) error {
	// Add specific validation for reading speed WPM if needed
	// For now, just ensure it's a reasonable positive number
	if value <= 0 {
		return dto.ErrInvalidNumber
	}
	return nil
}
