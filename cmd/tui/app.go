package tui

import (
	"context"
	"fmt"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/screens"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
)

// Ensure UnifiedService satisfies the Service interface used by commands.
var _ Service = (UnifiedService)(nil)

// UnifiedService is the interface the TUI expects from the service.
type UnifiedService interface {
	ListAllPlansWithReadingTime(ctx context.Context) ([]claudeviewer.PlanSummary, error)
	GetPlanDetailByFileName(ctx context.Context, fileName string) (*claudeviewer.PlanDetail, error)
	UpdatePlan(ctx context.Context, req claudeviewer.UpdatePlanRequest) (*claudeviewer.UpdatePlanResult, error)
	SearchPlansWithReadingTime(ctx context.Context, query string) ([]claudeviewer.PlanSummary, error)
	SavePlanVersion(ctx context.Context, planName, content string) error
	GetPlanVersionHistory(ctx context.Context, planName string, offset, limit int64) ([]claudeviewer.PlanVersionDetail, error)
	GetPlanVersion(ctx context.Context, planName string, versionNumber int64) (*claudeviewer.PlanVersionDetail, error)
	SearchVersions(ctx context.Context, planName, query string) ([]claudeviewer.PlanVersionDetail, error)
	RestorePlanVersion(ctx context.Context, planName string, versionNumber int64) error
	SyncPlans(ctx context.Context) (int, error)
	RenderMarkdown(content string) (string, error)
	GetSetting(ctx context.Context, variableName string) (claudeviewer.Setting, bool, error)
	SetSetting(ctx context.Context, varName string, values claudeviewer.SettingValues) error
}

// App is the root TUI application model.
type App struct {
	// Navigation stack.
	stack []screens.Screen

	// Help overlay (renders on top when visible).
	showHelp bool
	help     *screens.HelpScreen

	// Status bar.
	statusBar *components.StatusBar

	// Dimensions.
	width  int
	height int

	// Service.
	service UnifiedService
}

// New creates a new TUI application.
func New(service UnifiedService) *App {
	return &App{
		service:   service,
		statusBar: components.NewStatusBar(80),
	}
}

// Init initializes the application.
func (a *App) Init() tea.Cmd {
	// Initialize theme from settings.
	setting, exists, _ := a.service.GetSetting(context.Background(), claudeviewer.SettingDarkModeEnabled)
	darkMode := true // default
	if exists && setting.IsBoolean() {
		darkMode = setting.GetBooleanValue()
	}
	styles.SetDarkMode(darkMode)

	// Create initial plans screen.
	plansScreen := screens.NewPlansScreen(a.width, a.height)
	a.stack = append(a.stack, plansScreen)

	// Load initial plans.
	return LoadPlansCmd(a.service)
}

// Update handles all messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	// Window resize - broadcast to all screens.
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.statusBar.SetWidth(msg.Width)

		// Update help screen size.
		if a.help != nil {
			a.help.SetSize(msg.Width, msg.Height)
		}

		// Broadcast to all screens in stack.
		for _, screen := range a.stack {
			screen.SetSize(msg.Width, msg.Height)
		}
		return a, nil

	// Global key handling.
	case tea.KeyMsg:
		// Clear any error/success messages on new key press.
		a.statusBar.ClearMessages()

		// Help toggle (always available).
		if msg.String() == "?" {
			a.showHelp = !a.showHelp
			if a.showHelp && a.help == nil {
				a.help = screens.NewHelpScreen(a.width, a.height)
			}
			return a, nil
		}

		// Quit (only from root screen).
		if (msg.String() == "q" || msg.String() == "ctrl+c") && len(a.stack) == 1 && !a.showHelp {
			return a, tea.Quit
		}

		// If help is shown, handle help keys.
		if a.showHelp {
			if msg.String() == "esc" || msg.String() == "?" {
				a.showHelp = false
			}
			return a, nil
		}

	// Screen navigation messages.
	case screens.PopScreenMsg:
		return a, a.popScreen()

	case screens.RequestVersionsScreenMsg:
		// Check if versions exist before navigating.
		return a, LoadVersionsForNavigationCmd(a.service, msg.PlanName)

	case screens.VersionsNavigationResultMsg:
		// Check if versions exist.
		if len(msg.Versions) == 0 {
			// No versions - show message in App's status bar and stay on current screen.
			a.statusBar.SetError("No versions found")
			return a, nil
		}
		// Versions exist - push versions screen with data.
		return a, a.pushVersionsScreenWithData(msg.PlanName, msg.Versions)

	case screens.CloseHelpMsg:
		a.showHelp = false
		return a, nil

	// Service result messages - handle and delegate.
	case screens.PlansLoadedMsg:
		// Delegate to current screen.
		return a.delegateToCurrentScreen(msg)

	case screens.PlanDetailLoadedMsg:
		return a.delegateToCurrentScreen(msg)

	case screens.VersionsLoadedMsg:
		return a.delegateToCurrentScreen(msg)

	case screens.SaveResultMsg:
		return a.delegateToCurrentScreen(msg)

	case screens.SyncResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError("Sync failed: " + msg.Error.Error())
			return a, nil
		}
		// Show success message and reload plans.
		a.statusBar.SetSuccess(fmt.Sprintf("Synced %d plans", msg.Count))
		return a, LoadPlansCmd(a.service)

	case screens.ErrorMsg:
		// Show error in App's status bar (which is rendered).
		a.statusBar.SetError(msg.Error.Error())
		return a, nil

	// Internal command messages from screens.
	case screens.LoadPlanDetailMsg:
		return a, LoadPlanDetailCmd(a.service, msg.FileName)

	case screens.SavePlanMsg:
		return a, SavePlanCmd(a.service, msg.FileName, msg.Content, msg.Modified)

	case screens.SyncPlansMsg:
		a.statusBar.SetLoading("Syncing plans...")
		return a, SyncPlansCmd(a.service)

	case screens.LoadVersionsMsg:
		return a, LoadVersionsCmd(a.service, msg.PlanName)

	case screens.SearchPlansMsg:
		return a, SearchPlansCmd(a.service, msg.Query)

	case screens.SearchVersionsMsg:
		return a, SearchVersionsCmd(a.service, msg.PlanName, msg.Query)

	case screens.ClearSearchMsg:
		return a, LoadPlansCmd(a.service)

	case screens.RestoreVersionMsg:
		a.statusBar.SetLoading("Restoring version...")
		return a, RestoreVersionCmd(a.service, msg.PlanName, msg.VersionNumber)

	case screens.RestoreResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError("Restore failed: " + msg.Error.Error())
			return a, nil
		}
		a.statusBar.SetSuccess("Version restored successfully")
		// Pop versions screen to go back to plans.
		a.popScreen()
		// Sync and reload plans to show restored content.
		return a, tea.Batch(
			SyncPlansCmd(a.service),
			LoadPlansCmd(a.service),
		)

	// Settings messages.
	case screens.OpenSettingsMsg:
		return a, a.pushSettingsScreen()

	case screens.LoadSettingsMsg:
		return a, LoadSettingsCmd(a.service, msg.SettingNames)

	case screens.SaveSettingMsg:
		return a, SetSettingCmd(a.service, msg.Name, msg.Values)

	case screens.SettingsLoadedMsg:
		return a.delegateToCurrentScreen(msg)

	case screens.SettingUpdateResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError("Failed to save setting: " + msg.Error.Error())
		} else {
			a.statusBar.SetSuccess("Setting saved")
		}
		return a.delegateToCurrentScreen(msg)

	case screens.ThemeChangedMsg:
		// Get current dark mode setting and apply theme.
		setting, exists, _ := a.service.GetSetting(context.Background(), claudeviewer.SettingDarkModeEnabled)
		darkMode := true // default
		if exists && setting.IsBoolean() {
			darkMode = setting.GetBooleanValue()
		}
		styles.SetDarkMode(darkMode)
		return a, nil
	}

	// Delegate other messages to current screen.
	return a.delegateToCurrentScreen(msg)
}

// delegateToCurrentScreen passes a message to the current screen.
func (a *App) delegateToCurrentScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	if len(a.stack) == 0 {
		return a, nil
	}

	current := a.stack[len(a.stack)-1]
	newScreen, cmd := current.Update(msg)
	a.stack[len(a.stack)-1] = newScreen

	return a, cmd
}

// View renders the application.
func (a *App) View() string {
	if a.width == 0 || a.height == 0 {
		return "Loading..."
	}

	// If help is shown, render help overlay.
	if a.showHelp {
		return a.help.View()
	}

	// Get current screen view.
	var mainContent string
	if len(a.stack) > 0 {
		current := a.stack[len(a.stack)-1]
		mainContent = current.View()

		// Update status bar with screen's help.
		a.statusBar.SetHelp(current.ShortHelp())
	}

	// Render status bar (footer).
	status := a.statusBar.View()

	return fmt.Sprintf("%s\n%s", mainContent, status)
}

// Navigation helpers.
func (a *App) popScreen() tea.Cmd { //nolint:unparam // ok for now.
	if len(a.stack) > 1 {
		a.stack = a.stack[:len(a.stack)-1]
	}
	return nil
}

func (a *App) pushVersionsScreenWithData(planName string, versions []claudeviewer.PlanVersionDetail) tea.Cmd {
	versionsScreen := screens.NewVersionsScreenWithData(planName, versions, a.width, a.height)
	a.stack = append(a.stack, versionsScreen)
	return versionsScreen.Init()
}

func (a *App) pushSettingsScreen() tea.Cmd {
	settingsScreen := screens.NewSettingsScreen(a.width, a.height)
	a.stack = append(a.stack, settingsScreen)
	return settingsScreen.Init()
}
