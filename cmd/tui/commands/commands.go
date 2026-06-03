// Package commands represents a file with all the possible commands used by the TUI.
package commands

import (
	"context"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/sync/errgroup"
)

// Command builders.

// LoadPlansCmd loads all plans from the service.
func LoadPlansCmd(ctx context.Context, svc claudeviewer.UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		plans, err := svc.ListAllPlansWithReadingTime(ctx)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}
		return messages.PlansLoadedMsg{Plans: plans}
	}
}

// SearchPlansCmd searches plans by query.
func SearchPlansCmd(ctx context.Context, svc claudeviewer.UnifiedService, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		plans, err := svc.SearchPlansWithReadingTime(ctx, query)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}
		return messages.PlansLoadedMsg{Plans: plans, IsFiltered: true}
	}
}

// LoadPlanDetailCmd loads a specific plan's details.
func LoadPlanDetailCmd(ctx context.Context, svc claudeviewer.UnifiedService, fileName, syncSource string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		detail, err := svc.GetPlanDetailByFileName(ctx, fileName, syncSource)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}
		return messages.PlanDetailLoadedMsg{Detail: detail}
	}
}

// LoadVersionsCmd loads version history for a plan.
func LoadVersionsCmd(ctx context.Context, svc claudeviewer.UnifiedService, planName, syncSource string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		versions, err := svc.GetPlanVersionHistory(ctx, planName, syncSource, 0, 100)
		if err != nil {
			return messages.VersionErrorMsg{Error: err}
		}
		return messages.VersionsLoadedMsg{Versions: versions}
	}
}

// LoadVersionsForNavigationCmd checks versions before navigating to versions screen.
func LoadVersionsForNavigationCmd(ctx context.Context, svc claudeviewer.UnifiedService, planName, syncSource string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		versions, err := svc.GetPlanVersionHistory(ctx, planName, syncSource, 0, 100)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}
		return messages.VersionsNavigationResultMsg{
			PlanName:   planName,
			SyncSource: syncSource,
			Versions:   versions,
		}
	}
}

// SavePlanCmd saves a plan and syncs to source directory.
func SavePlanCmd(ctx context.Context, svc claudeviewer.UnifiedService, fileName, syncSource, content string, lastModified time.Time) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		req := claudeviewer.UpdatePlanRequest{
			FileName:         fileName,
			SyncSource:       syncSource,
			NewContent:       content,
			LastModifiedTime: lastModified,
			Force:            true,
		}

		result, err := svc.UpdatePlan(ctx, req)
		if err != nil {
			return messages.SaveResultMsg{Error: err}
		}
		return messages.SaveResultMsg{Result: result}
	}
}

// SyncPlansCmd syncs plans from the source directory.
func SyncPlansCmd(ctx context.Context, svc claudeviewer.UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		count, err := svc.SyncPlans(ctx)
		if err != nil {
			return messages.SyncResultMsg{Error: err}
		}
		return messages.SyncResultMsg{Count: count}
	}
}

// RsyncPlansCmd syncs plans from the viewer directory back to the source directory.
func RsyncPlansCmd(ctx context.Context, svc claudeviewer.UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		count, err := svc.RSyncPlans(ctx)
		if err != nil {
			return messages.RSyncResultMsg{Error: err}
		}
		return messages.RSyncResultMsg{Count: count}
	}
}

// CreateTagCmd creates a new tag via the service.
func CreateTagCmd(ctx context.Context, svc claudeviewer.UnifiedService, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		_, err := svc.CreateTag(ctx, name, nil, nil)
		return messages.CreateTagResultMsg{Error: err}
	}
}

// DumpPlansCmd writes all plans from the database back to the source plans directory.
func DumpPlansCmd(ctx context.Context, svc claudeviewer.UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		count, err := svc.DumpPlans(ctx)
		if err != nil {
			return messages.DumpResultMsg{Error: err}
		}
		return messages.DumpResultMsg{Count: count}
	}
}

// SearchVersionsCmd searches a plan over its different versions.
func SearchVersionsCmd(ctx context.Context, svc claudeviewer.UnifiedService, currentPlanName, syncSource, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		planVersions, err := svc.SearchVersions(ctx, currentPlanName, syncSource, query)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}

		return messages.VersionsLoadedMsg{Versions: planVersions}
	}
}

// RestoreVersionCmd restores a plan to a previous version.
func RestoreVersionCmd(ctx context.Context, svc claudeviewer.UnifiedService, planName, syncSource string, versionNumber int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.RestorePlanVersion(ctx, planName, syncSource, versionNumber)
		if err != nil {
			return messages.RestoreResultMsg{Error: err}
		}
		return messages.RestoreResultMsg{
			Success:  true,
			PlanName: planName,
		}
	}
}

// LoadSettingsCmd loads settings by name.
func LoadSettingsCmd(ctx context.Context, svc claudeviewer.UnifiedService, settingNames []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		settings := make(map[string]claudeviewer.Setting)
		for _, name := range settingNames {
			setting, exists, err := svc.GetSetting(ctx, name)
			if err == nil && exists {
				settings[name] = setting
			}
		}
		return messages.SettingsLoadedMsg{Settings: settings}
	}
}

// SetSettingCmd updates a setting.
func SetSettingCmd(ctx context.Context, svc claudeviewer.UnifiedService, name string, values claudeviewer.SettingValues) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.SetSetting(ctx, name, values)
		if err != nil {
			return messages.SettingUpdateResultMsg{Error: err}
		}
		return messages.SettingUpdateResultMsg{
			Success:     true,
			SettingName: name,
		}
	}
}

// SendToConnectorCmd sends a plan to the enabled connector.
func SendToConnectorCmd(ctx context.Context, svc claudeviewer.UnifiedService, planFileName, syncSource string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err := svc.SendToConnector(ctx, planFileName, syncSource)
		if err != nil {
			return messages.SendToConnectorResultMsg{Success: false, Error: err}
		}
		return messages.SendToConnectorResultMsg{Success: true}
	}
}

// LoadConnectorsCmd loads all available connectors with their status.
func LoadConnectorsCmd(ctx context.Context, svc claudeviewer.UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		connectors, err := svc.ListConnectors(ctx)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}

		statuses := make([]messages.ConnectorStatus, len(connectors))
		for i, c := range connectors {
			statuses[i] = messages.ConnectorStatus{
				Name:         c.Name,
				DisplayName:  c.DisplayName,
				IsTransmit:   c.IsTransmit(),
				IsSummarizer: c.IsSummarizer(),
				Configured:   c.Configured,
			}
		}
		return messages.ConnectorsLoadedMsg{Connectors: statuses}
	}
}

// LoadConnectorSettingsCmd loads settings for a specific connector.
func LoadConnectorSettingsCmd(ctx context.Context, svc claudeviewer.UnifiedService, connectorName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		settings, err := svc.GetConnectorSettings(ctx, connectorName)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}

		values := make([]messages.ConnectorSettingValue, len(settings))
		for i, s := range settings {
			values[i] = messages.ConnectorSettingValue{
				Key:         s.Key,
				DisplayName: s.DisplayName,
				Description: s.Description,
				Value:       s.Value,
				Required:    s.Required,
				Sensitive:   s.Sensitive,
			}
		}
		return messages.ConnectorSettingsLoadedMsg{
			ConnectorName: connectorName,
			Settings:      values,
		}
	}
}

// EnableConnectorCmd enables a specific connector.
func EnableConnectorCmd(ctx context.Context, svc claudeviewer.UnifiedService, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.EnableConnector(ctx, name)
		if err != nil {
			return messages.ConnectorUpdateResultMsg{Success: false, Error: err}
		}
		return messages.ConnectorUpdateResultMsg{Success: true}
	}
}

// DisableConnectorCmd clears the transmit connector slot.
func DisableConnectorCmd(ctx context.Context, svc claudeviewer.UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.DisableConnector(ctx)
		if err != nil {
			return messages.ConnectorUpdateResultMsg{Success: false, Error: err}
		}
		return messages.ConnectorUpdateResultMsg{Success: true}
	}
}

// ClearSummaryConnectorCmd clears the summary connector slot.
func ClearSummaryConnectorCmd(ctx context.Context, svc claudeviewer.UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.ClearSummaryConnector(ctx)
		if err != nil {
			return messages.ConnectorUpdateResultMsg{Success: false, Error: err}
		}
		return messages.ConnectorUpdateResultMsg{Success: true}
	}
}

// SaveConnectorSettingCmd saves a connector setting.
func SaveConnectorSettingCmd(ctx context.Context, svc claudeviewer.UnifiedService, connectorName, key, value string, isSecret bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.ConfigureConnector(ctx, connectorName, key, value, isSecret)
		if err != nil {
			return messages.ConnectorUpdateResultMsg{Success: false, Error: err}
		}
		return messages.ConnectorUpdateResultMsg{Success: true}
	}
}

// ValidateConnectorCmd validates a connector's settings.
func ValidateConnectorCmd(ctx context.Context, svc claudeviewer.UnifiedService, connectorName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		err := svc.ValidateConnector(ctx, connectorName)
		if err != nil {
			return messages.ValidateConnectorResultMsg{Success: false, Error: err}
		}
		return messages.ValidateConnectorResultMsg{Success: true}
	}
}

// GenerateTLDRCmd generates a TLDR summary for the given plan using the configured summarizer.
func GenerateTLDRCmd(ctx context.Context, svc claudeviewer.UnifiedService, fileName, syncSource string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		plan, err := svc.GetPlanDetailByFileName(ctx, fileName, syncSource)
		if err != nil {
			return messages.TLDRGeneratedMsg{Err: err}
		}

		summary, err := svc.GenerateSummary(ctx, fileName, syncSource)
		return messages.TLDRGeneratedMsg{Summary: summary, PlanTitle: plan.Title, Err: err}
	}
}

// RegenerateTLDRCmd force-regenerates a TLDR summary, bypassing the in-memory cache.
func RegenerateTLDRCmd(ctx context.Context, svc claudeviewer.UnifiedService, fileName, syncSource string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
		defer cancel()

		plan, err := svc.GetPlanDetailByFileName(ctx, fileName, syncSource)
		if err != nil {
			return messages.TLDRGeneratedMsg{Err: err}
		}

		summary, err := svc.RegenerateSummary(ctx, fileName, syncSource)
		return messages.TLDRGeneratedMsg{Summary: summary, PlanTitle: plan.Title, Err: err}
	}
}

// SetSummaryConnectorCmd sets a connector as the active summarizer.
func SetSummaryConnectorCmd(ctx context.Context, svc claudeviewer.UnifiedService, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.SetSummaryConnector(ctx, name)
		if err != nil {
			return messages.ConnectorUpdateResultMsg{Success: false, Error: err}
		}
		return messages.ConnectorUpdateResultMsg{Success: true}
	}
}

// WatchChannelListenerCmd listens to the service's watch result channel
// and converts results into WatchResultMsg for the TUI to handle.
func WatchChannelListenerCmd(ctx context.Context, svc claudeviewer.UnifiedService) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-ctx.Done():
			return nil
		case result := <-svc.GetWatchResultChannel():
			return messages.WatchResultMsg{
				Count: result.Count,
				Error: result.Error,
			}
		}
	}
}

// LoadAllTagsForPanelCmd loads all tags, the tag→plans map, and all plans for the tag panel.
// Counts and untagged count are derived from the map to avoid redundant DB round-trips.
func LoadAllTagsForPanelCmd(ctx context.Context, svc claudeviewer.UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		errGroup, eggCtx := errgroup.WithContext(ctx)
		var tags []claudeviewer.Tag
		errGroup.Go(func() error {
			var err error
			tags, err = svc.GetAllTags(eggCtx)
			return err
		})

		var allPlans []claudeviewer.PlanSummary
		errGroup.Go(func() error {
			var err error
			allPlans, err = svc.ListAllPlansWithReadingTime(eggCtx)
			return err
		})

		var tagPlanMap map[string][]claudeviewer.PlanSummary
		errGroup.Go(func() error {
			var err error
			tagPlanMap, err = svc.BuildTagPlanMap(eggCtx)
			return err
		})

		if err := errGroup.Wait(); err != nil {
			return messages.ErrorMsg{Error: err}
		}

		counts := make(map[string]int, len(tagPlanMap))
		for k, v := range tagPlanMap {
			if k != "" {
				counts[k] = len(v)
			}
		}
		untaggedCount := len(tagPlanMap[""])

		return messages.AllTagsForPanelLoadedMsg{
			Tags:          tags,
			Counts:        counts,
			UntaggedCount: untaggedCount,
			TagPlanMap:    tagPlanMap,
			AllPlans:      allPlans,
		}
	}
}

// LoadUntaggedPlansCmd loads plans with no tags assigned.
func LoadUntaggedPlansCmd(ctx context.Context, svc claudeviewer.UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		plans, err := svc.ListUntaggedPlansWithReadingTime(ctx)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}
		return messages.PlansLoadedMsg{Plans: plans, IsFiltered: true}
	}
}

// ClearStatusCmd clears the status bar after a delay.
func ClearStatusCmd(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return messages.ClearStatusMsg{}
	})
}

func ClearStatusCmdWithDefaultDuration() tea.Cmd {
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return messages.ClearStatusMsg{}
	})
}

// LoadTagsForModalCmd loads all tags and current plan tags for the tag modal.
func LoadTagsForModalCmd(ctx context.Context, svc claudeviewer.UnifiedService, fileName, syncSource string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		allTags, err := svc.GetAllTags(ctx)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}

		planTags, err := svc.GetPlanTags(ctx, fileName, syncSource)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}

		return components.TagsLoadedMsg{
			FileName:   fileName,
			SyncSource: syncSource,
			PlanTags:   planTags,
			AllTags:    allTags,
		}
	}
}

// SetPlanTagsCmd sets tags for a plan.
func SetPlanTagsCmd(ctx context.Context, svc claudeviewer.UnifiedService, fileName, syncSource string, tags []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.SetPlanTags(ctx, fileName, syncSource, tags)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}

		plans, err := svc.ListAllPlansWithReadingTime(ctx)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}
		return messages.PlansLoadedMsg{Plans: plans}
	}
}

// SearchPlansWithTagsCmd searches plans with text query and tag filters.
func SearchPlansWithTagsCmd(ctx context.Context, svc claudeviewer.UnifiedService, query string, tags []string, matchAll bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		plans, err := svc.SearchPlansWithTags(ctx, query, tags, matchAll)
		if err != nil {
			return messages.ErrorMsg{Error: err}
		}
		return messages.PlansLoadedMsg{Plans: plans, IsFiltered: true}
	}
}

// ApplyWatchSettingCmd reads the watch mode settings from the service and returns
// a WatchModeApplyMsg so the app can start or stop the watcher without blocking Update().
func ApplyWatchSettingCmd(ctx context.Context, svc claudeviewer.UnifiedService) tea.Cmd {
	return func() tea.Msg {
		enabledSetting, exists, err := svc.GetSetting(ctx, claudeviewer.SettingWatchModeEnabled)
		if err != nil || !exists {
			return messages.WatchModeApplyMsg{}
		}
		enabled := enabledSetting.IsBoolean() && enabledSetting.GetBooleanValue()

		interval := claudeviewer.DefaultWatchIntervalSeconds
		if s, exists, err := svc.GetSetting(ctx, claudeviewer.SettingWatchIntervalSeconds); err == nil && exists && s.IsNumber() {
			interval = s.GetNumberValue()
		}
		return messages.WatchModeApplyMsg{Enabled: enabled, Interval: interval}
	}
}

// DeleteTagsCmd deletes a tag.
func DeleteTagsCmd(ctx context.Context, svc claudeviewer.UnifiedService, tagID int64, fileName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.DeleteTag(ctx, tagID)
		if err != nil {
			return components.DeleteTagCmdMsg{Error: new(err.Error())}
		}

		return components.DeleteTagCmdMsg{
			FileName: fileName,
		}
	}
}
