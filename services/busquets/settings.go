package busquets

import (
	"context"
	"fmt"
	"time"

	planviewer "github.com/Javier162380/busquets"
	"github.com/Javier162380/busquets/internal/clipboard"
	"github.com/Javier162380/busquets/services/busquets/dto"

	"charm.land/glamour/v2/styles"
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
	SettingReadingSpeedWPM         = "reading_speed_wpm" //nolint:gci//No need
	SettingDarkModeEnabled         = "dark_mode_enabled"
	SettingRenderMarkdownByDefault = "render_markdown_by_default"
	SettingWatchModeEnabled        = "watch_mode_enabled"
	SettingWatchIntervalSeconds    = "watch_interval_seconds"
	SettingDefaultDisplayMode      = "default_display_mode"
	SettingPlansSortKey            = "plans_sort_key"
	SettingPlansSortDir            = "plans_sort_dir"
	SettingSearchOver              = "search_over"
	SettingClipboardMode           = "clipboard_mode"
	SettingMarkdownTheme           = "markdown_theme"
	SettingsScreenOrientation      = "screen_orientation"
)

// Clipboard mode values for SettingClipboardMode. These mirror the modes in
// internal/clipboard: native clipboard, OSC52 escape sequence, or auto (native
// with an OSC52 fallback).
const (
	ClipboardModeAuto    = clipboard.ModeAuto
	ClipboardModeNative  = clipboard.ModeNative
	ClipboardModeOSC52   = clipboard.ModeOSC52
	DefaultClipboardMode = ClipboardModeAuto
)

// SearchField re-exports the root type so callers only need one import.
type SearchField = planviewer.SearchField

// Search field values for SettingSearchOver.
const (
	SearchOverAll      = planviewer.SearchOverAll
	SearchOverPlanName = planviewer.SearchOverPlanName
	SearchOverContent  = planviewer.SearchOverContent
	DefaultSearchOver  = planviewer.DefaultSearchOver
)

// Display mode values for SettingDefaultDisplayMode.
const (
	DisplayModePlanContent      = "plan_content"       //nolint:gci    // two-panel: list + content (default)
	DisplayModeTagPlanContent   = "tag_plan_content"   // three-panel: tags + list + content
	DisplayModeLabelPlanContent = "label_plan_content" // three-panel: sync labels + list + content
)

// Sort key values for SettingPlansSortKey.
const (
	SortKeyUpdatedAt    = "updated_at"
	SortKeyCreatedAt    = "created_at"
	SortKeyReadingTime  = "reading_time"
	SortKeySize         = "size"
	DefaultPlansSortKey = SortKeyUpdatedAt
)

// Sort direction values for SettingPlansSortDir.
const (
	SortDirDesc    = "desc"
	SortDirAsc     = "asc"
	DefaultSortDir = SortDirDesc
)

const (
	MarkdownThemeDark       = styles.DarkStyle
	MarkdownThemeLight      = styles.LightStyle
	MarkdownThemeDracula    = styles.DraculaStyle
	MarkdownThemeTokyoNight = styles.TokyoNightStyle
	MarkdownThemePinkStyle  = styles.PinkStyle
	MarkdownThemeASCII      = styles.AsciiStyle
	DefaultMarkdownTheme    = MarkdownThemeTokyoNight
)

// Orientaton values for ScreenOrientation.
const (
	ScreenOrientationHorizontal = "horizontal"
	ScreenOrientationVertical   = "vertical"
	DefaultScreenOrientation    = ScreenOrientationHorizontal
)

// sortKeyToColumn maps a sort key setting value to the corresponding DB column name.
func sortKeyToColumn(key string) string {
	switch key {
	case SortKeyCreatedAt:
		return "created_at"
	case SortKeyReadingTime:
		return "reading_time"
	case SortKeySize:
		return "file_size"
	default:
		return "modified_at"
	}
}

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

	return toSetting(domainSetting), true, nil
}

// ListSettings returns every stored setting among names, keyed by name — one
// query instead of one GetSetting call per name. A name with no stored row is
// simply absent from the result, same as GetSetting's exists=false case.
func (s *Service) ListSettings(ctx context.Context, names []string) (map[string]Setting, error) {
	rows, err := s.db.ListSettings(ctx, names)
	if err != nil {
		return nil, fmt.Errorf("failed to list settings: %w", err)
	}

	settings := make(map[string]Setting, len(rows))
	for _, row := range rows {
		settings[row.VariableName] = toSetting(row)
	}
	return settings, nil
}

// toSetting maps a dto.Setting row to the domain Setting type.
func toSetting(s dto.Setting) Setting {
	return Setting{
		SettingName: s.VariableName,
		StringValue: s.StringValue,
		NumberValue: s.NumberValue,
		BoolValue:   s.BooleanValue,
		DateValue:   s.DatetimeValue,
	}
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

// resolveSearchScope returns the active SearchField setting, defaulting to SearchOverAll.
func (s *Service) resolveSearchScope(ctx context.Context) planviewer.SearchField {
	if setting, exists, _ := s.GetSetting(ctx, SettingSearchOver); exists && setting.IsString() {
		return planviewer.SearchField(setting.GetStringValue())
	}
	return planviewer.DefaultSearchOver
}

// getClipboardMode returns the stored clipboard mode setting, or the default.
// The BUSQUETS_CLIPBOARD env var can still override this at write time
// (see internal/clipboard).
func (s *Service) getClipboardMode(ctx context.Context) string {
	if setting, exists, _ := s.GetSetting(ctx, SettingClipboardMode); exists && setting.IsString() {
		return setting.GetStringValue()
	}
	return DefaultClipboardMode
}

// getSortSettings returns the sort key and direction from DB settings, with defaults.
func (s *Service) getSortSettings(ctx context.Context) (sortKey, sortDir string) {
	sortKey = DefaultPlansSortKey
	if setting, exists, _ := s.GetSetting(ctx, SettingPlansSortKey); exists && setting.IsString() {
		sortKey = setting.GetStringValue()
	}
	sortDir = DefaultSortDir
	if setting, exists, _ := s.GetSetting(ctx, SettingPlansSortDir); exists && setting.IsString() {
		sortDir = setting.GetStringValue()
	}
	return sortKey, sortDir
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

// validateNumberValue validates numeric values with optional bounds.
func validateNumberValue(value float64) error {
	// Add specific validation for reading speed WPM if needed
	// For now, just ensure it's a reasonable positive number
	if value <= 0 {
		return dto.ErrInvalidNumber
	}
	return nil
}
