// Package messages defines all TUI message types used to communicate between
// the App, screens, and background commands.
package messages

import (
	"time"

	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
)

// --- Watch / status ---

// WatchResultMsg carries the result of a background watch sync.
type WatchResultMsg struct {
	Count int
	Error error
}

// ClearStatusMsg signals to clear the status bar message.
type ClearStatusMsg struct{}

// --- Plans ---

// PlansLoadedMsg is sent when plans are loaded.
type PlansLoadedMsg struct {
	Plans      []claudeviewer.PlanSummary
	IsFiltered bool // when true, only s.plans is updated, not s.allPlans
}

// PlanDetailLoadedMsg is sent when plan detail is loaded.
type PlanDetailLoadedMsg struct {
	Detail *claudeviewer.PlanDetail
}

// LoadPlanDetailMsg requests loading a plan detail.
type LoadPlanDetailMsg struct {
	FileName string
}

// SavePlanMsg requests saving a plan.
type SavePlanMsg struct {
	FileName string
	Content  string
	Modified time.Time
}

// SyncPlansMsg requests syncing plans.
type SyncPlansMsg struct{}

// RSyncPlansMsg requests resyncing plans from the viewer directory back into the LLM directory.
type RSyncPlansMsg struct{}

// DumpPlansMsg requests dumping all plans from the database to the source plans directory.
type DumpPlansMsg struct{}

// SaveResultMsg is sent when save completes.
type SaveResultMsg struct {
	Result *claudeviewer.UpdatePlanResult
	Error  error
}

// SyncResultMsg is sent when sync completes.
type SyncResultMsg struct {
	Count int
	Error error
}

// RSyncResultMsg is sent when rsync completes.
type RSyncResultMsg struct {
	Count int
	Error error
}

// DumpResultMsg is sent when dump completes.
type DumpResultMsg struct {
	Count int
	Error error
}

// ErrorMsg is sent on error.
type ErrorMsg struct {
	Error error
}

// --- Tags ---

// CreateTagMsg requests creating a new tag.
type CreateTagMsg struct {
	Name string
}

// CreateTagResultMsg is sent when tag creation completes.
type CreateTagResultMsg struct {
	Error error
}

// OpenTagModalMsg requests opening the tag modal for a plan.
type OpenTagModalMsg struct {
	FileName string
}

// LoadTagsForModalMsg requests loading tags for the modal.
type LoadTagsForModalMsg struct {
	FileName string
}

// SavePlanTagsMsg requests saving tags for a plan.
type SavePlanTagsMsg struct {
	FileName string
	Tags     []string
}

// SetPlanTagsMsg requests setting tags via service layer.
type SetPlanTagsMsg struct {
	FileName string
	Tags     []string
}

// FilterByTagsMsg requests filtering plans by tags.
type FilterByTagsMsg struct {
	Tags     []string
	MatchAll bool
}

// LoadAllTagsForPanelMsg requests loading all tags for the tag panel.
type LoadAllTagsForPanelMsg struct{}

// LoadUntaggedPlansMsg requests loading plans with no tags.
type LoadUntaggedPlansMsg struct{}

// AllTagsForPanelLoadedMsg carries all tags, their plan counts, and the tag→plans map.
type AllTagsForPanelLoadedMsg struct {
	Tags          []claudeviewer.Tag
	Counts        map[string]int
	UntaggedCount int
	TagPlanMap    map[string][]claudeviewer.PlanSummary
}

// --- Search ---

// SearchPlansMsg requests searching plans.
type SearchPlansMsg struct {
	PlanName string
	Query    string
}

// ClearSearchMsg requests clearing search and loading all plans.
type ClearSearchMsg struct{}

// SearchPlansWithTagsMsg requests searching plans with tag filter.
type SearchPlansWithTagsMsg struct {
	Query    string
	Tags     []string
	MatchAll bool
}

// --- Connector ---

// ConnectorStatus represents a connector's current state.
type ConnectorStatus struct {
	Name        string
	DisplayName string
	Enabled     bool
	Configured  bool
}

// ConnectorSettingValue holds a setting's definition and current value.
type ConnectorSettingValue struct {
	Key         string
	DisplayName string
	Description string
	Value       string
	Required    bool
	Sensitive   bool
}

// OpenConnectorsMsg requests opening the connectors screen.
type OpenConnectorsMsg struct{}

// LoadConnectorsMsg requests loading connectors list.
type LoadConnectorsMsg struct{}

// ConnectorsLoadedMsg carries loaded connectors.
type ConnectorsLoadedMsg struct {
	Connectors []ConnectorStatus
}

// LoadConnectorSettingsMsg requests loading a connector's settings.
type LoadConnectorSettingsMsg struct {
	ConnectorName string
}

// ConnectorSettingsLoadedMsg carries connector settings with values.
type ConnectorSettingsLoadedMsg struct {
	ConnectorName string
	Settings      []ConnectorSettingValue
}

// EnableConnectorMsg requests enabling a connector.
type EnableConnectorMsg struct {
	Name string
}

// DisableConnectorMsg requests disabling all connectors.
type DisableConnectorMsg struct{}

// SaveConnectorSettingMsg requests saving a connector setting.
type SaveConnectorSettingMsg struct {
	ConnectorName string
	Key           string
	Value         string
	IsSecret      bool
}

// ConnectorUpdateResultMsg carries result of connector operations.
type ConnectorUpdateResultMsg struct {
	Success bool
	Error   error
}

// ValidateConnectorMsg requests validating a connector's settings.
type ValidateConnectorMsg struct {
	Name string
}

// ValidateConnectorResultMsg carries the validation result.
type ValidateConnectorResultMsg struct {
	Success bool
	Error   error
}

// SendToConnectorMsg requests sending current plan to connector.
type SendToConnectorMsg struct {
	PlanFileName string
}

// SendToConnectorResultMsg is the result of sending to connector.
type SendToConnectorResultMsg struct {
	Success bool
	Error   error
}

// --- Settings ---

// OpenSettingsMsg requests opening the settings screen.
type OpenSettingsMsg struct{}

// LoadSettingsMsg requests loading settings by name.
type LoadSettingsMsg struct {
	SettingNames []string
}

// SettingsLoadedMsg carries loaded settings.
type SettingsLoadedMsg struct {
	Settings map[string]claudeviewer.Setting
}

// SaveSettingMsg requests saving a setting.
type SaveSettingMsg struct {
	Name   string
	Values claudeviewer.SettingValues
}

// SettingUpdateResultMsg carries result of a setting update.
type SettingUpdateResultMsg struct {
	Success     bool
	SettingName string
	Error       error
}

// ThemeChangedMsg signals a theme change.
type ThemeChangedMsg struct{}

// RenderMarkDownByDefaultMsg signals a render-markdown-by-default change.
type RenderMarkDownByDefaultMsg struct{}

// DisplayModeChangedMsg signals a display mode change.
type DisplayModeChangedMsg struct{}

// --- Versions ---

// RequestVersionsScreenMsg requests checking versions before navigation.
type RequestVersionsScreenMsg struct {
	PlanName string
}

// VersionsNavigationResultMsg carries the result of a version check.
type VersionsNavigationResultMsg struct {
	PlanName string
	Versions []claudeviewer.PlanVersionDetail
}

// LoadVersionsMsg requests loading versions for a plan.
type LoadVersionsMsg struct {
	PlanName string
}

// VersionsLoadedMsg is sent when versions are loaded.
type VersionsLoadedMsg struct {
	Versions []claudeviewer.PlanVersionDetail
}

// VersionErrorMsg is sent on version error.
type VersionErrorMsg struct {
	Error error
}

// RestoreVersionMsg requests restoring a version.
type RestoreVersionMsg struct {
	PlanName      string
	VersionNumber int64
}

// RestoreResultMsg is sent when restore completes.
type RestoreResultMsg struct {
	Success  bool
	PlanName string
	Error    error
}

// SearchVersionsMsg requests searching within a plan's versions.
type SearchVersionsMsg struct {
	PlanName string
	Query    string
}

// ClearVersionSearchMsg requests clearing the version search.
type ClearVersionSearchMsg struct{}

// PopScreenMsg requests popping the current screen.
type PopScreenMsg struct{}

// --- Help ---

// CloseHelpMsg requests closing the help screen.
type CloseHelpMsg struct{}
