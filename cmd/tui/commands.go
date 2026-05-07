package tui

import (
	"context"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/screens"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
	"golang.org/x/sync/errgroup"
)

// Command builders.

// LoadPlansCmd loads all plans from the service.
func LoadPlansCmd(ctx context.Context, svc UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		plans, err := svc.ListAllPlansWithReadingTime(ctx)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}
		return screens.PlansLoadedMsg{Plans: plans}
	}
}

// SearchPlansCmd searches plans by query.
func SearchPlansCmd(ctx context.Context, svc UnifiedService, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		plans, err := svc.SearchPlansWithReadingTime(ctx, query)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}
		return screens.PlansLoadedMsg{Plans: plans}
	}
}

// LoadPlanDetailCmd loads a specific plan's details.
func LoadPlanDetailCmd(ctx context.Context, svc UnifiedService, fileName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		detail, err := svc.GetPlanDetailByFileName(ctx, fileName)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}
		return screens.PlanDetailLoadedMsg{Detail: detail}
	}
}

// LoadVersionsCmd loads version history for a plan.
func LoadVersionsCmd(ctx context.Context, svc UnifiedService, planName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		versions, err := svc.GetPlanVersionHistory(ctx, planName, 0, 100)
		if err != nil {
			return screens.VersionErrorMsg{Error: err}
		}
		return screens.VersionsLoadedMsg{Versions: versions}
	}
}

// LoadVersionsForNavigationCmd checks versions before navigating to versions screen.
func LoadVersionsForNavigationCmd(ctx context.Context, svc UnifiedService, planName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		versions, err := svc.GetPlanVersionHistory(ctx, planName, 0, 100)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}
		return screens.VersionsNavigationResultMsg{
			PlanName: planName,
			Versions: versions,
		}
	}
}

// SavePlanCmd saves a plan and syncs to source directory.
func SavePlanCmd(ctx context.Context, svc UnifiedService, fileName, content string, lastModified time.Time) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		req := claudeviewer.UpdatePlanRequest{
			FileName:         fileName,
			NewContent:       content,
			LastModifiedTime: lastModified,
			Force:            true,
		}

		result, err := svc.UpdatePlan(ctx, req)
		if err != nil {
			return screens.SaveResultMsg{Error: err}
		}
		return screens.SaveResultMsg{Result: result}
	}
}

// SyncPlansCmd syncs plans from the source directory.
func SyncPlansCmd(ctx context.Context, svc UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		count, err := svc.SyncPlans(ctx)
		if err != nil {
			return screens.SyncResultMsg{Error: err}
		}
		return screens.SyncResultMsg{Count: count}
	}
}

// RsyncPlansCmd syncs plans from the viewer directory back to the source directory.
func RsyncPlansCmd(ctx context.Context, svc UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		count, err := svc.RSyncPlans(ctx)
		if err != nil {
			return screens.RSyncResultMsg{Error: err}
		}
		return screens.RSyncResultMsg{Count: count}
	}
}

// CreateTagCmd creates a new tag via the service.
func CreateTagCmd(ctx context.Context, svc UnifiedService, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		_, err := svc.CreateTag(ctx, name, nil, nil)
		return screens.CreateTagResultMsg{Error: err}
	}
}

// DumpPlansCmd writes all plans from the database back to the source plans directory.
func DumpPlansCmd(ctx context.Context, svc UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		count, err := svc.DumpPlans(ctx)
		if err != nil {
			return screens.DumpResultMsg{Error: err}
		}
		return screens.DumpResultMsg{Count: count}
	}
}

// SearchVersionsCmd searches a plan over it's different versions.
func SearchVersionsCmd(ctx context.Context, svc UnifiedService, currentPlanName, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		planVersions, err := svc.SearchVersions(ctx, currentPlanName, query)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}

		return screens.VersionsLoadedMsg{Versions: planVersions}
	}
}

// RestoreVersionCmd restores a plan to a previous version.
func RestoreVersionCmd(ctx context.Context, svc UnifiedService, planName string, versionNumber int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.RestorePlanVersion(ctx, planName, versionNumber)
		if err != nil {
			return screens.RestoreResultMsg{Error: err}
		}
		return screens.RestoreResultMsg{
			Success:  true,
			PlanName: planName,
		}
	}
}

// LoadSettingsCmd loads settings by name.
func LoadSettingsCmd(ctx context.Context, svc UnifiedService, settingNames []string) tea.Cmd {
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
		return screens.SettingsLoadedMsg{Settings: settings}
	}
}

// SetSettingCmd updates a setting.
func SetSettingCmd(ctx context.Context, svc UnifiedService, name string, values claudeviewer.SettingValues) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.SetSetting(ctx, name, values)
		if err != nil {
			return screens.SettingUpdateResultMsg{Error: err}
		}
		return screens.SettingUpdateResultMsg{
			Success:     true,
			SettingName: name,
		}
	}
}

// SendToConnectorCmd sends a plan to the enabled connector.
func SendToConnectorCmd(ctx context.Context, svc UnifiedService, planFileName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		err := svc.SendToConnector(ctx, planFileName)
		if err != nil {
			return screens.SendToConnectorResultMsg{Success: false, Error: err}
		}
		return screens.SendToConnectorResultMsg{Success: true}
	}
}

// LoadConnectorsCmd loads all available connectors with their status.
func LoadConnectorsCmd(ctx context.Context, svc UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		connectors, err := svc.ListConnectors(ctx)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}

		// Convert to screen types
		statuses := make([]screens.ConnectorStatus, len(connectors))
		for i, c := range connectors {
			statuses[i] = screens.ConnectorStatus{
				Name:        c.Name,
				DisplayName: c.DisplayName,
				Enabled:     c.Enabled,
				Configured:  c.Configured,
			}
		}
		return screens.ConnectorsLoadedMsg{Connectors: statuses}
	}
}

// LoadConnectorSettingsCmd loads settings for a specific connector.
func LoadConnectorSettingsCmd(ctx context.Context, svc UnifiedService, connectorName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		settings, err := svc.GetConnectorSettings(ctx, connectorName)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}

		// Convert to screen types
		values := make([]screens.ConnectorSettingValue, len(settings))
		for i, s := range settings {
			values[i] = screens.ConnectorSettingValue{
				Key:         s.Key,
				DisplayName: s.DisplayName,
				Description: s.Description,
				Value:       s.Value,
				Required:    s.Required,
				Sensitive:   s.Sensitive,
			}
		}
		return screens.ConnectorSettingsLoadedMsg{
			ConnectorName: connectorName,
			Settings:      values,
		}
	}
}

// EnableConnectorCmd enables a specific connector.
func EnableConnectorCmd(ctx context.Context, svc UnifiedService, name string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.EnableConnector(ctx, name)
		if err != nil {
			return screens.ConnectorUpdateResultMsg{Success: false, Error: err}
		}
		return screens.ConnectorUpdateResultMsg{Success: true}
	}
}

// DisableConnectorCmd disables all connectors.
func DisableConnectorCmd(ctx context.Context, svc UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.DisableConnector(ctx)
		if err != nil {
			return screens.ConnectorUpdateResultMsg{Success: false, Error: err}
		}
		return screens.ConnectorUpdateResultMsg{Success: true}
	}
}

// SaveConnectorSettingCmd saves a connector setting.
func SaveConnectorSettingCmd(ctx context.Context, svc UnifiedService, connectorName, key, value string, isSecret bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.ConfigureConnector(ctx, connectorName, key, value, isSecret)
		if err != nil {
			return screens.ConnectorUpdateResultMsg{Success: false, Error: err}
		}
		return screens.ConnectorUpdateResultMsg{Success: true}
	}
}

// ValidateConnectorCmd validates a connector's settings.
func ValidateConnectorCmd(ctx context.Context, svc UnifiedService, connectorName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()

		err := svc.ValidateConnector(ctx, connectorName)
		if err != nil {
			return screens.ValidateConnectorResultMsg{Success: false, Error: err}
		}
		return screens.ValidateConnectorResultMsg{Success: true}
	}
}

// WatchChannelListenerCmd listens to the service's watch result channel
// and converts results into WatchResultMsg for the TUI to handle.
func WatchChannelListenerCmd(ctx context.Context, svc UnifiedService) tea.Cmd {
	return func() tea.Msg {
		select {
		case <-ctx.Done():
			return nil
		case result := <-svc.GetWatchResultChannel():
			return WatchResultMsg{
				Count: result.Count,
				Error: result.Error,
			}
		}
	}
}

// LoadAllTagsForPanelCmd loads all tags, their plan counts, and the untagged count for the tag panel.
func LoadAllTagsForPanelCmd(ctx context.Context, svc UnifiedService) tea.Cmd {
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

		var counts map[string]int
		errGroup.Go(func() error {
			var err error
			counts, err = svc.GetTagPlanCounts(eggCtx)
			return err
		})

		var untaggedCount int64
		errGroup.Go(func() error {
			var err error
			untaggedCount, err = svc.GetUntaggedPlanCount(eggCtx)
			return err
		})

		if err := errGroup.Wait(); err != nil {
			return screens.ErrorMsg{Error: err}
		}
		return screens.AllTagsForPanelLoadedMsg{Tags: tags, Counts: counts, UntaggedCount: int(untaggedCount)}
	}
}

// LoadUntaggedPlansCmd loads plans with no tags assigned.
func LoadUntaggedPlansCmd(ctx context.Context, svc UnifiedService) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		plans, err := svc.ListUntaggedPlansWithReadingTime(ctx)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}
		return screens.PlansLoadedMsg{Plans: plans, IsFiltered: true}
	}
}

// ClearStatusCmd clears the status bar after a delay.
func ClearStatusCmd(delay time.Duration) tea.Cmd {
	return tea.Tick(delay, func(time.Time) tea.Msg {
		return ClearStatusMsg{}
	})
}

// LoadTagsForModalCmd loads all tags and current plan tags for the tag modal.
func LoadTagsForModalCmd(ctx context.Context, svc UnifiedService, fileName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		// Load all tags.
		allTags, err := svc.GetAllTags(ctx)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}

		// Load plan tags.
		planTags, err := svc.GetPlanTags(ctx, fileName)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}

		return components.TagsLoadedMsg{
			FileName: fileName,
			PlanTags: planTags,
			AllTags:  allTags,
		}
	}
}

// SetPlanTagsCmd sets tags for a plan.
func SetPlanTagsCmd(ctx context.Context, svc UnifiedService, fileName string, tags []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()

		err := svc.SetPlanTags(ctx, fileName, tags)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}

		// Reload plans after setting tags.
		plans, err := svc.ListAllPlansWithReadingTime(ctx)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}
		return screens.PlansLoadedMsg{Plans: plans}
	}
}

// SearchPlansWithTagsCmd searches plans with text query and tag filters.
func SearchPlansWithTagsCmd(ctx context.Context, svc UnifiedService, query string, tags []string, matchAll bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		plans, err := svc.SearchPlansWithTags(ctx, query, tags, matchAll)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}
		return screens.PlansLoadedMsg{Plans: plans, IsFiltered: true}
	}
}

// DeleteTagsCmd delete a tag command.
func DeleteTagsCmd(ctx context.Context, svc UnifiedService, tagID int64, fileName string) tea.Cmd {
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
