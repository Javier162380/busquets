package tui

import (
	"context"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/screens"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
)

// Service interface for the commands package.
// This is a subset of UnifiedService that commands need.
type Service interface {
	ListAllPlansWithReadingTime(ctx context.Context) ([]claudeviewer.PlanSummary, error)
	SearchPlansWithReadingTime(ctx context.Context, query string) ([]claudeviewer.PlanSummary, error)
	GetPlanDetailByFileName(ctx context.Context, fileName string) (*claudeviewer.PlanDetail, error)
	UpdatePlan(ctx context.Context, req claudeviewer.UpdatePlanRequest) (*claudeviewer.UpdatePlanResult, error)
	GetPlanVersionHistory(ctx context.Context, planName string, offset, limit int64) ([]claudeviewer.PlanVersionDetail, error)
	SearchVersions(ctx context.Context, planName, query string) ([]claudeviewer.PlanVersionDetail, error)
	SyncPlans(ctx context.Context) (int, error)
	RenderMarkdown(content string) (string, error)
	RestorePlanVersion(ctx context.Context, planName string, versionNumber int64) error
	GetSetting(ctx context.Context, variableName string) (claudeviewer.Setting, bool, error)
	SetSetting(ctx context.Context, varName string, values claudeviewer.SettingValues) error
	SendToConnector(ctx context.Context, planFileName string) error
}

// Command builders.

// LoadPlansCmd loads all plans from the service.
func LoadPlansCmd(svc Service) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		plans, err := svc.ListAllPlansWithReadingTime(ctx)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}
		return screens.PlansLoadedMsg{Plans: plans}
	}
}

// SearchPlansCmd searches plans by query.
func SearchPlansCmd(svc Service, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		plans, err := svc.SearchPlansWithReadingTime(ctx, query)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}
		return screens.PlansLoadedMsg{Plans: plans}
	}
}

// LoadPlanDetailCmd loads a specific plan's details.
func LoadPlanDetailCmd(svc Service, fileName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()

		detail, err := svc.GetPlanDetailByFileName(ctx, fileName)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}
		return screens.PlanDetailLoadedMsg{Detail: detail}
	}
}

// LoadVersionsCmd loads version history for a plan.
func LoadVersionsCmd(svc Service, planName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		versions, err := svc.GetPlanVersionHistory(ctx, planName, 0, 100)
		if err != nil {
			return screens.VersionErrorMsg{Error: err}
		}
		return screens.VersionsLoadedMsg{Versions: versions}
	}
}

// LoadVersionsForNavigationCmd checks versions before navigating to versions screen.
func LoadVersionsForNavigationCmd(svc Service, planName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
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
func SavePlanCmd(svc Service, fileName, content string, lastModified time.Time) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
func SyncPlansCmd(svc Service) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		count, err := svc.SyncPlans(ctx)
		if err != nil {
			return screens.SyncResultMsg{Error: err}
		}
		return screens.SyncResultMsg{Count: count}
	}
}

// SearchVersionsCmd searches a plan over it's different versions.
func SearchVersionsCmd(svc Service, currentPlanName, query string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		planVersions, err := svc.SearchVersions(ctx, currentPlanName, query)
		if err != nil {
			return screens.ErrorMsg{Error: err}
		}

		return screens.VersionsLoadedMsg{Versions: planVersions}
	}
}

// RestoreVersionCmd restores a plan to a previous version.
func RestoreVersionCmd(svc Service, planName string, versionNumber int64) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
func LoadSettingsCmd(svc Service, settingNames []string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
func SetSettingCmd(svc Service, name string, values claudeviewer.SettingValues) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
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
func SendToConnectorCmd(svc Service, planFileName string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		err := svc.SendToConnector(ctx, planFileName)
		if err != nil {
			return screens.SendToConnectorResultMsg{Success: false, Error: err}
		}
		return screens.SendToConnectorResultMsg{Success: true}
	}
}
