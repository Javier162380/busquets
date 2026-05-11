package tui

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/screens"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davecgh/go-spew/spew"
)

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
	RSyncPlans(ctx context.Context) (int, error)
	DumpPlans(ctx context.Context) (int, error)
	CreateTag(ctx context.Context, name string, description, color *string) (claudeviewer.Tag, error)
	RenderMarkdown(content string) (string, error)
	GetSetting(ctx context.Context, variableName string) (claudeviewer.Setting, bool, error)
	SetSetting(ctx context.Context, varName string, values claudeviewer.SettingValues) error
	SendToConnector(ctx context.Context, planFileName string) error
	ListConnectors(ctx context.Context) ([]claudeviewer.ConnectorInfo, error)
	EnableConnector(ctx context.Context, name string) error
	DisableConnector(ctx context.Context) error
	ConfigureConnector(ctx context.Context, connectorName, key, value string, isSecret bool) error
	GetConnectorSettings(ctx context.Context, connectorName string) ([]claudeviewer.ConnectorSettingInfo, error)
	ValidateConnector(ctx context.Context, connectorName string) error
	StartWatchMode(ctx context.Context, intervalSeconds float64) error
	StopWatchMode(ctx context.Context) error
	GetWatchResultChannel() <-chan claudeviewer.WatchResult
	IsWatchModeRunning() bool
	UpdateWatchInterval(intervalSeconds float64)
	GetAllTags(ctx context.Context) ([]claudeviewer.Tag, error)
	GetTagPlanCounts(ctx context.Context) (map[string]int, error)
	GetUntaggedPlanCount(ctx context.Context) (int64, error)
	ListUntaggedPlansWithReadingTime(ctx context.Context) ([]claudeviewer.PlanSummary, error)
	BuildTagPlanMap(ctx context.Context) (map[string][]claudeviewer.PlanSummary, error)
	GetPlanTags(ctx context.Context, fileName string) ([]claudeviewer.Tag, error)
	SetPlanTags(ctx context.Context, fileName string, tagNames []string) error
	SearchPlansWithTags(ctx context.Context, query string, tags []string, matchAll bool) ([]claudeviewer.PlanSummary, error)
	DeleteTag(ctx context.Context, id int64) error
}

// App is the root TUI application model.
type App struct {
	ctx context.Context

	stack []screens.Screen

	showHelp bool
	help     *screens.HelpScreen

	statusBar *components.StatusBar

	width  int
	height int

	service UnifiedService

	dump io.Writer

	isDarkModeEnabled       bool
	renderMarkDownByDefault bool
	displayMode             string
}

// New creates a new TUI application.
func New(ctx context.Context, service UnifiedService) *App {
	return &App{
		ctx:       ctx,
		service:   service,
		statusBar: components.NewStatusBar(80),
	}
}

// SetDump sets the debug dump writer.
func (a *App) SetDump(w io.Writer) {
	a.dump = w
}

// Init initializes the application.
func (a *App) Init() tea.Cmd {
	// Initialize theme from settings.
	setting, exists, _ := a.service.GetSetting(a.ctx, claudeviewer.SettingDarkModeEnabled)
	darkMode := true // default
	if exists && setting.IsBoolean() {
		darkMode = setting.GetBooleanValue()
	}
	styles.SetDarkMode(darkMode)
	a.isDarkModeEnabled = darkMode

	setting, exists, _ = a.service.GetSetting(a.ctx, claudeviewer.SettingRenderMarkdownByDefault)
	renderMarkDownByDefault := false
	if exists && setting.IsBoolean() {
		renderMarkDownByDefault = setting.GetBooleanValue()
	}
	a.renderMarkDownByDefault = renderMarkDownByDefault

	setting, exists, _ = a.service.GetSetting(a.ctx, claudeviewer.SettingDefaultDisplayMode)
	displayMode := claudeviewer.DisplayModePlanContent
	if exists && setting.IsString() {
		displayMode = setting.GetStringValue()
	}
	a.displayMode = displayMode

	// Create initial plans screen.
	plansScreen := screens.NewPlansScreen(a.width, a.height, darkMode, renderMarkDownByDefault, displayMode == claudeviewer.DisplayModePlanContent, displayMode)
	a.stack = append(a.stack, plansScreen)

	// Start watching for watch results and load initial plans.
	return tea.Batch(
		LoadPlansCmd(a.ctx, a.service),
		WatchChannelListenerCmd(a.ctx, a.service),
	)
}

// Update handles all messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.dump != nil {
		spew.Fdump(a.dump, msg)
	}

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

	case tea.KeyMsg:
		a.statusBar.ClearMessages()

		if msg.String() == "?" {
			a.showHelp = !a.showHelp
			if a.showHelp && a.help == nil {
				a.help = screens.NewHelpScreen(a.width, a.height)
			}
			return a, nil
		}

		if (msg.String() == "q" || msg.String() == "ctrl+c") && !a.showHelp && len(a.stack) > 0 && !a.stack[len(a.stack)-1].IsInputMode() {
			return a, tea.Quit
		}

		if a.showHelp {
			if msg.String() == "esc" || msg.String() == "?" {
				a.showHelp = false
			}
			return a, nil
		}

	case messages.PopScreenMsg:
		return a, a.popScreen()

	case messages.RequestVersionsScreenMsg:
		return a, LoadVersionsForNavigationCmd(a.ctx, a.service, msg.PlanName)

	case messages.VersionsNavigationResultMsg:
		if len(msg.Versions) == 0 {
			a.statusBar.SetError("No versions found")
			return a, ClearStatusCmd(500 * time.Millisecond)
		}
		return a, a.pushVersionsScreenWithData(msg.PlanName, msg.Versions)

	case messages.CloseHelpMsg:
		a.showHelp = false
		return a, nil

	case messages.PlansLoadedMsg:
		return a.delegateToCurrentScreen(msg)

	case messages.PlanDetailLoadedMsg:
		return a.delegateToCurrentScreen(msg)

	case messages.VersionsLoadedMsg:
		return a.delegateToCurrentScreen(msg)

	case messages.SaveResultMsg:
		switch {
		case msg.Error != nil:
			a.statusBar.SetError("Unable to save result")
		case msg.Result.Success:
			a.statusBar.SetSuccess("Saved result")
		case msg.Result.HasConflict:
			a.statusBar.SetError(fmt.Sprintf("Unable to save result, conflict %s", msg.Result.ConflictInfo.Message))
		}
		return a, ClearStatusCmd(500 * time.Millisecond)

	case messages.SyncResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError("Sync failed: " + msg.Error.Error())
			return a, nil
		}
		a.statusBar.SetSuccess(fmt.Sprintf("Synced %d plans", msg.Count))
		return a, tea.Batch(LoadPlansCmd(a.ctx, a.service), ClearStatusCmd(1*time.Second))

	case messages.RSyncResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError("RSync failed: " + msg.Error.Error())
			return a, nil
		}
		a.statusBar.SetSuccess(fmt.Sprintf("Rsync succeeded: %d plans sync from remote into the local directory", msg.Count))
		return a, LoadPlansCmd(a.ctx, a.service)

	case messages.DumpResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError("Dump failed: " + msg.Error.Error())
			return a, ClearStatusCmd(500 * time.Millisecond)
		}
		a.statusBar.SetSuccess(fmt.Sprintf("Dumped %d plans to source directory", msg.Count))
		return a, ClearStatusCmd(500 * time.Millisecond)

	case messages.DumpPlansMsg:
		a.statusBar.SetLoading("Dumping plans from database to source directory...")
		return a, DumpPlansCmd(a.ctx, a.service)

	case messages.DisplayModeChangedMsg:
		setting, exists, _ := a.service.GetSetting(a.ctx, claudeviewer.SettingDefaultDisplayMode)
		mode := claudeviewer.DisplayModePlanContent
		if exists && setting.IsString() {
			mode = setting.GetStringValue()
		}
		a.displayMode = mode
		for _, screen := range a.stack {
			if s, ok := screen.(*screens.PlansScreen); ok {
				s.SetDisplayMode(mode)
			}
		}
		if mode == claudeviewer.DisplayModeTagPlanContent {
			return a, LoadAllTagsForPanelCmd(a.ctx, a.service)
		}
		return a, nil

	case messages.LoadAllTagsForPanelMsg:
		return a, LoadAllTagsForPanelCmd(a.ctx, a.service)

	case messages.AllTagsForPanelLoadedMsg:
		return a.delegateToCurrentScreen(msg)

	case messages.CreateTagMsg:
		return a, CreateTagCmd(a.ctx, a.service, msg.Name)

	case messages.CreateTagResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError("Failed to create tag: " + msg.Error.Error())
			return a, ClearStatusCmd(2 * time.Second)
		}
		a.statusBar.SetSuccess("Tag created")
		return a, tea.Batch(
			LoadPlansCmd(a.ctx, a.service),
			LoadAllTagsForPanelCmd(a.ctx, a.service),
			ClearStatusCmd(1*time.Second),
		)

	case messages.ErrorMsg:
		a.statusBar.SetError(msg.Error.Error())
		return a, ClearStatusCmd(500 * time.Millisecond)

	case messages.LoadPlanDetailMsg:
		return a, LoadPlanDetailCmd(a.ctx, a.service, msg.FileName)

	case messages.SavePlanMsg:
		return a, SavePlanCmd(a.ctx, a.service, msg.FileName, msg.Content, msg.Modified)

	case messages.SyncPlansMsg:
		a.statusBar.SetLoading("Syncing plans...")
		return a, SyncPlansCmd(a.ctx, a.service)

	case messages.RSyncPlansMsg:
		a.statusBar.SetLoading("Rsyncing plans from remote directory into the LLM directory...")
		return a, RsyncPlansCmd(a.ctx, a.service)

	case messages.LoadVersionsMsg:
		return a, LoadVersionsCmd(a.ctx, a.service, msg.PlanName)

	case messages.SearchPlansMsg:
		return a, SearchPlansCmd(a.ctx, a.service, msg.Query)

	case messages.SearchVersionsMsg:
		return a, SearchVersionsCmd(a.ctx, a.service, msg.PlanName, msg.Query)

	case messages.ClearSearchMsg:
		return a, LoadPlansCmd(a.ctx, a.service)

	case messages.RestoreVersionMsg:
		a.statusBar.SetLoading("Restoring version...")
		return a, RestoreVersionCmd(a.ctx, a.service, msg.PlanName, msg.VersionNumber)

	case messages.RestoreResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError("Restore failed: " + msg.Error.Error())
			return a, nil
		}
		a.statusBar.SetSuccess("Version restored successfully")
		a.popScreen()
		return a, tea.Batch(
			SyncPlansCmd(a.ctx, a.service),
			LoadPlansCmd(a.ctx, a.service),
		)

	case messages.OpenSettingsMsg:
		return a, a.pushSettingsScreen()

	case messages.LoadSettingsMsg:
		return a, LoadSettingsCmd(a.ctx, a.service, msg.SettingNames)

	case messages.SaveSettingMsg:
		return a, SetSettingCmd(a.ctx, a.service, msg.Name, msg.Values)

	case messages.SettingsLoadedMsg:
		return a.delegateToCurrentScreen(msg)

	case messages.SettingUpdateResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError("Failed to save setting: " + msg.Error.Error())
		} else {
			a.statusBar.SetSuccess("Setting saved")
		}

		if msg.Success && msg.SettingName == claudeviewer.SettingWatchModeEnabled {
			setting, exists, _ := a.service.GetSetting(a.ctx, claudeviewer.SettingWatchModeEnabled)
			if exists && setting.IsBoolean() {
				enabled := setting.GetBooleanValue()
				if enabled {
					intervalSetting, exists, _ := a.service.GetSetting(a.ctx, claudeviewer.SettingWatchIntervalSeconds)
					intervalSeconds := 5.0
					if exists && intervalSetting.IsNumber() {
						intervalSeconds = intervalSetting.GetNumberValue()
					}
					_ = a.service.StartWatchMode(a.ctx, intervalSeconds)
					a.statusBar.SetSuccess("Watch mode enabled")
				} else {
					_ = a.service.StopWatchMode(a.ctx)
					a.statusBar.SetSuccess("Watch mode disabled")
				}
			}
		}

		if msg.Success && msg.SettingName == claudeviewer.SettingWatchIntervalSeconds {
			if a.service.IsWatchModeRunning() {
				setting, exists, _ := a.service.GetSetting(a.ctx, claudeviewer.SettingWatchIntervalSeconds)
				if exists && setting.IsNumber() {
					a.service.UpdateWatchInterval(setting.GetNumberValue())
				}
			}
		}

		return a.delegateToCurrentScreen(msg)

	case messages.WatchResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError(fmt.Sprintf("Auto-sync failed: %v", msg.Error))
		} else if msg.Count > 0 {
			a.statusBar.SetSuccess(fmt.Sprintf("Auto-synced %d plans", msg.Count))
			return a, tea.Batch(
				LoadPlansCmd(a.ctx, a.service),
				WatchChannelListenerCmd(a.ctx, a.service),
				ClearStatusCmd(1*time.Second),
			)
		}
		return a, WatchChannelListenerCmd(a.ctx, a.service)

	case messages.ClearStatusMsg:
		a.statusBar.Clear()
		return a, nil

	case messages.SendToConnectorMsg:
		a.statusBar.SetLoading("Sending to connector...")
		return a, SendToConnectorCmd(a.ctx, a.service, msg.PlanFileName)

	case messages.SendToConnectorResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError("Send failed: " + msg.Error.Error())
		} else {
			a.statusBar.SetSuccess("Sent successfully!")
		}
		return a, ClearStatusCmd(1 * time.Second)

	case messages.OpenConnectorsMsg:
		return a, a.pushConnectorsScreen()

	case messages.LoadConnectorsMsg:
		return a, LoadConnectorsCmd(a.ctx, a.service)

	case messages.ConnectorsLoadedMsg:
		return a.delegateToCurrentScreen(msg)

	case messages.LoadConnectorSettingsMsg:
		return a, LoadConnectorSettingsCmd(a.ctx, a.service, msg.ConnectorName)

	case messages.ConnectorSettingsLoadedMsg:
		return a.delegateToCurrentScreen(msg)

	case messages.EnableConnectorMsg:
		a.statusBar.SetLoading("Enabling connector...")
		return a, EnableConnectorCmd(a.ctx, a.service, msg.Name)

	case messages.DisableConnectorMsg:
		a.statusBar.SetLoading("Disabling connector...")
		return a, DisableConnectorCmd(a.ctx, a.service)

	case messages.SaveConnectorSettingMsg:
		a.statusBar.SetLoading("Saving...")
		return a, SaveConnectorSettingCmd(a.ctx, a.service, msg.ConnectorName, msg.Key, msg.Value, msg.IsSecret)

	case messages.ConnectorUpdateResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError(msg.Error.Error())
		} else {
			a.statusBar.SetSuccess("Connector updated")
		}
		return a.delegateToCurrentScreen(msg)

	case messages.ValidateConnectorMsg:
		a.statusBar.SetLoading("Validating...")
		return a, ValidateConnectorCmd(a.ctx, a.service, msg.Name)

	case messages.ValidateConnectorResultMsg:
		if msg.Error != nil {
			a.statusBar.SetError("Validation failed: " + msg.Error.Error())
		} else {
			a.statusBar.SetSuccess("Connector validated successfully!")
		}
		return a, ClearStatusCmd(1 * time.Second)

	case messages.LoadTagsForModalMsg:
		return a, LoadTagsForModalCmd(a.ctx, a.service, msg.FileName)

	case components.TagsLoadedMsg:
		return a.delegateToCurrentScreen(msg)

	case messages.SetPlanTagsMsg:
		a.statusBar.SetLoading("Saving tags...")
		return a, tea.Batch(
			SetPlanTagsCmd(a.ctx, a.service, msg.FileName, msg.Tags),
			ClearStatusCmd(1*time.Second),
		)

	case messages.SearchPlansWithTagsMsg:
		return a, SearchPlansWithTagsCmd(a.ctx, a.service, msg.Query, msg.Tags, msg.MatchAll)

	case messages.LoadUntaggedPlansMsg:
		return a, LoadUntaggedPlansCmd(a.ctx, a.service)

	case components.DeleteTagMsg:
		return a, DeleteTagsCmd(a.ctx, a.service, msg.TagID, msg.CurrentPlan)

	case components.DeleteTagCmdMsg:
		if msg.Error != nil {
			a.statusBar.SetError(fmt.Sprintf("Failed to delete tag %s", *msg.Error))
		} else {
			a.statusBar.SetSuccess("Deleted tag")
		}
		return a, tea.Batch(
			ClearStatusCmd(500*time.Millisecond), LoadTagsForModalCmd(a.ctx, a.service, msg.FileName),
		)

	case messages.ThemeChangedMsg:
		setting, exists, _ := a.service.GetSetting(a.ctx, claudeviewer.SettingDarkModeEnabled)
		darkMode := true
		if exists && setting.IsBoolean() {
			darkMode = setting.GetBooleanValue()
		}
		a.isDarkModeEnabled = darkMode
		styles.SetDarkMode(darkMode)

		for _, screen := range a.stack {
			switch s := screen.(type) {
			case *screens.PlansScreen:
				s.UpdateDarkMode(darkMode)
			case *screens.VersionsScreen:
				s.UpdateDarkMode(darkMode)
			}
		}
		return a, nil

	case messages.RenderMarkDownByDefaultMsg:
		setting, exists, _ := a.service.GetSetting(a.ctx, claudeviewer.SettingRenderMarkdownByDefault)
		renderMarkDownByDefault := false
		if exists && setting.IsBoolean() {
			renderMarkDownByDefault = setting.GetBooleanValue()
		}
		a.renderMarkDownByDefault = renderMarkDownByDefault

		for _, screen := range a.stack {
			switch s := screen.(type) {
			case *screens.PlansScreen:
				s.RenderedMarkdownByDefault(renderMarkDownByDefault)
			case *screens.VersionsScreen:
				s.RenderedMarkdownByDefault(renderMarkDownByDefault)
			}
		}
		return a, nil
	}

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

	if a.showHelp {
		return a.help.View()
	}

	var mainContent string
	if len(a.stack) > 0 {
		current := a.stack[len(a.stack)-1]
		mainContent = current.View()
		a.statusBar.SetHelp(current.ShortHelp())
	}

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
	versionsScreen := screens.NewVersionsScreenWithData(planName, versions, a.width, a.height, a.isDarkModeEnabled, a.renderMarkDownByDefault)
	a.stack = append(a.stack, versionsScreen)
	return versionsScreen.Init()
}

func (a *App) pushSettingsScreen() tea.Cmd {
	settingsScreen := screens.NewSettingsScreen(a.width, a.height)
	a.stack = append(a.stack, settingsScreen)
	return settingsScreen.Init()
}

func (a *App) pushConnectorsScreen() tea.Cmd {
	connectorsScreen := screens.NewConnectorsScreen(a.width, a.height)
	a.stack = append(a.stack, connectorsScreen)
	return connectorsScreen.Init()
}
