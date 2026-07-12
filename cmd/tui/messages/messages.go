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
	FileName   string
	SyncSource string
}

// SavePlanMsg requests saving a plan.
type SavePlanMsg struct {
	FileName   string
	SyncSource string
	Content    string
	Modified   time.Time
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

// DeletePlanMsg requests deleting a single plan (DB row, files, and versions).
// FilePath is the plan's stored mirror path, sent so the service deletes the
// exact file rather than re-deriving the path.
type DeletePlanMsg struct{ FileName, SyncSource, FilePath string }

// DeletePlanResultMsg is sent when a plan delete completes.
type DeletePlanResultMsg struct {
	FileName, SyncSource string
	Error                error
}

// RenamePlanFileMsg requests renaming a plan's file (source, mirror, versions, DB row).
// FilePath is the plan's stored mirror path; NewFileName is the requested new name.
type RenamePlanFileMsg struct{ FileName, SyncSource, FilePath, NewFileName string }

// RenamePlanFileResultMsg is sent when a plan file rename completes.
type RenamePlanFileResultMsg struct {
	FileName, SyncSource, NewFileName string
	Error                             error
}

// CopyPlanContentMsg requests copying a plan's raw markdown to the clipboard.
type CopyPlanContentMsg struct{ FileName, SyncSource string }

// CopyPlanContentResultMsg is sent when a clipboard copy completes.
type CopyPlanContentResultMsg struct {
	FileName string
	Error    error
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
	FileName   string
	SyncSource string
}

// LoadTagsForModalMsg requests loading tags for the modal.
type LoadTagsForModalMsg struct {
	FileName   string
	SyncSource string
}

// SavePlanTagsMsg requests saving tags for a plan.
type SavePlanTagsMsg struct {
	FileName   string
	SyncSource string
	Tags       []string
}

// SetPlanTagsMsg requests setting tags via service layer.
type SetPlanTagsMsg struct {
	FileName   string
	SyncSource string
	Tags       []string
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
	AllPlans      []claudeviewer.PlanSummary
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
	Name         string
	DisplayName  string
	IsTransmit   bool
	IsSummarizer bool
	Configured   bool
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

// DisableConnectorMsg requests clearing the transmit connector slot.
type DisableConnectorMsg struct{}

// ClearSummaryConnectorMsg requests clearing the summary connector slot.
type ClearSummaryConnectorMsg struct{}

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
	SyncSource   string
}

// SendToConnectorResultMsg is the result of sending to connector.
type SendToConnectorResultMsg struct {
	Success bool
	Error   error
}

// GenerateTLDRMsg requests generating a TLDR summary for a plan.
type GenerateTLDRMsg struct {
	FileName   string
	SyncSource string
}

// RegenerateTLDRMsg requests a fresh TLDR summary, bypassing the in-memory cache.
type RegenerateTLDRMsg struct {
	FileName   string
	SyncSource string
}

// TLDRGeneratedMsg carries the generated TLDR summary.
type TLDRGeneratedMsg struct {
	Summary   string
	PlanTitle string
	Err       error
}

// SetSummaryConnectorMsg requests setting a connector as the summarizer.
type SetSummaryConnectorMsg struct {
	ConnectorName string
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
type ThemeChangedMsg struct{ DarkMode bool }

// RenderMarkDownByDefaultMsg signals a render-markdown-by-default change.
type RenderMarkDownByDefaultMsg struct{ Enabled bool }

// DisplayModeChangedMsg signals a display mode change.
type DisplayModeChangedMsg struct{ Mode string }

// PlansSortKeyChangedMsg signals a plans sort key change.
type PlansSortKeyChangedMsg struct{ SortKey string }

// PlansSortDirChangedMsg signals a plans sort direction change.
type PlansSortDirChangedMsg struct{ SortDir string }

// WatchModeApplyMsg signals that the watcher should be started or stopped.
type WatchModeApplyMsg struct {
	Enabled  bool
	Interval float64
}

// --- Versions ---

// RequestVersionsScreenMsg requests checking versions before navigation.
type RequestVersionsScreenMsg struct {
	PlanName   string
	SyncSource string
}

// VersionsNavigationResultMsg carries the result of a version check.
type VersionsNavigationResultMsg struct {
	PlanName   string
	SyncSource string
	Versions   []claudeviewer.PlanVersionDetail
}

// LoadVersionsMsg requests loading versions for a plan.
type LoadVersionsMsg struct {
	PlanName   string
	SyncSource string
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
	SyncSource    string
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
	PlanName   string
	SyncSource string
	Query      string
}

// ClearVersionSearchMsg requests clearing the version search.
type ClearVersionSearchMsg struct{}

// PopScreenMsg requests popping the current screen.
type PopScreenMsg struct{}

// --- Help ---

// CloseHelpMsg requests closing the help screen.
type CloseHelpMsg struct{}

// --- Comments ---

// OpenCommentModalMsg requests opening the comment modal for a plan.
type OpenCommentModalMsg struct {
	FileName   string
	SyncSource string
}

// CommentsLoadedMsg carries comments fetched from the service.
type CommentsLoadedMsg struct {
	Comments   []claudeviewer.Comment
	FileName   string
	SyncSource string
}

// AddCommentMsg requests saving a new comment.
type AddCommentMsg struct {
	FileName   string
	SyncSource string
	Content    string
}

// AddCommentResultMsg carries the result of adding a comment.
type AddCommentResultMsg struct {
	FileName   string
	SyncSource string
	Error      error
}

// DeleteCommentMsg requests deleting a comment by ID.
type DeleteCommentMsg struct {
	CommentID  int64
	FileName   string
	SyncSource string
}

// DeleteCommentResultMsg carries the result of deleting a comment.
type DeleteCommentResultMsg struct {
	FileName   string
	SyncSource string
	Error      error
}
