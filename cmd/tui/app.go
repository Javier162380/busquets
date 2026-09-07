package tui

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/Javier162380/busquets/cmd/tui/commands"
	"github.com/Javier162380/busquets/cmd/tui/components"
	"github.com/Javier162380/busquets/cmd/tui/messages"
	"github.com/Javier162380/busquets/cmd/tui/screens"
	"github.com/Javier162380/busquets/cmd/tui/styles"
	"github.com/Javier162380/busquets/services/busquets"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/davecgh/go-spew/spew"
)

// App is the root TUI application model.
type App struct {
	ctx context.Context

	stack []screens.Screen

	showHelp bool
	help     *screens.HelpScreen

	statusBar *components.StatusBar

	width  int
	height int

	service busquets.UnifiedService

	dump io.Writer

	isDarkModeEnabled       bool
	renderMarkDownByDefault bool
	displayMode             string
	markdownRenderedTheme   string
	screenOrientation       string
}

// New creates a new TUI application.
func New(ctx context.Context, service busquets.UnifiedService) *App {
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
	// Load all startup settings in one query instead of one GetSetting call each.
	names := []string{
		busquets.SettingDarkModeEnabled,
		busquets.SettingRenderMarkdownByDefault,
		busquets.SettingDefaultDisplayMode,
		busquets.SettingMarkdownTheme,
		busquets.SettingsScreenOrientation,
	}
	settings, _ := a.service.ListSettings(a.ctx, names) // missing/errored names are just absent

	darkMode := true // default
	if setting, ok := settings[busquets.SettingDarkModeEnabled]; ok && setting.IsBoolean() {
		darkMode = setting.GetBooleanValue()
	}
	styles.SetDarkMode(darkMode)
	a.isDarkModeEnabled = darkMode

	renderMarkDownByDefault := false
	if setting, ok := settings[busquets.SettingRenderMarkdownByDefault]; ok && setting.IsBoolean() {
		renderMarkDownByDefault = setting.GetBooleanValue()
	}
	a.renderMarkDownByDefault = renderMarkDownByDefault

	displayMode := busquets.DisplayModePlanContent
	if setting, ok := settings[busquets.SettingDefaultDisplayMode]; ok && setting.IsString() {
		displayMode = setting.GetStringValue()
	}
	a.displayMode = displayMode

	markdownRenderedTheme := busquets.DefaultMarkdownTheme
	if setting, ok := settings[busquets.SettingMarkdownTheme]; ok && setting.IsString() {
		markdownRenderedTheme = setting.GetStringValue()
	}
	a.markdownRenderedTheme = markdownRenderedTheme

	screenOrientation := busquets.DefaultScreenOrientation
	if setting, ok := settings[busquets.SettingsScreenOrientation]; ok && setting.IsString() {
		screenOrientation = setting.GetStringValue()
	}
	a.screenOrientation = screenOrientation

	// Create initial plans screen.
	plansScreen := screens.NewPlansScreen(a.width, a.height, darkMode, renderMarkDownByDefault, displayMode == busquets.DisplayModePlanContent, displayMode, markdownRenderedTheme, screenOrientation)
	a.stack = append(a.stack, plansScreen)

	// Start watching for watch results and load initial plans.
	return tea.Batch(
		commands.LoadPlansCmd(a.ctx, a.service),
		commands.WatchChannelListenerCmd(a.ctx, a.service),
	)
}

// Update handles all messages.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if a.dump != nil {
		spew.Fdump(a.dump, msg)
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.statusBar.SetWidth(msg.Width)
		if a.help != nil {
			a.help.SetSize(msg.Width, msg.Height)
		}
		for _, screen := range a.stack {
			screen.SetSize(msg.Width, msg.Height)
		}
		return a, nil

	case tea.KeyMsg:
		a.statusBar.ClearMessages()
		currentScreen := a.stack[len(a.stack)-1]
		if msg.String() == "?" && !currentScreen.EditorMode() {
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
			newScreen, cmd := a.help.Update(msg)
			if h, ok := newScreen.(*screens.HelpScreen); ok {
				a.help = h
			}
			return a, cmd
		}
	case messages.PopScreenMsg:
		return a, a.popScreen()
	case messages.ClearStatusMsg:
		a.statusBar.Clear()
		return a, nil
	case messages.CloseHelpMsg:
		a.showHelp = false
		return a, nil
	case messages.ErrorMsg:
		a.statusBar.SetError(msg.Error.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	case messages.VersionErrorMsg:
		a.statusBar.SetError(msg.Error.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	case messages.ContentSearchErrorMsg:
		a.statusBar.SetError(msg.Error.Error())
		return a, commands.ClearStatusCmd(2 * time.Second)
	case messages.PlansLoadedMsg, messages.AllTagsForPanelLoadedMsg:
		return a.delegateToPlansScreen(msg)
	case messages.PlanDetailLoadedMsg,
		messages.VersionsLoadedMsg,
		messages.SettingsLoadedMsg,
		messages.ConnectorsLoadedMsg,
		messages.ConnectorSettingsLoadedMsg,
		components.TagsLoadedMsg:
		return a.delegateToCurrentScreen(msg)
	case messages.LoadPlanDetailMsg:
		return a, commands.LoadPlanDetailCmd(a.ctx, a.service, msg.FileName, msg.SyncSource)
	case messages.SavePlanMsg:
		return a, commands.SavePlanCmd(a.ctx, a.service, msg.FileName, msg.SyncSource, msg.Content, msg.Modified)
	case messages.LoadVersionsMsg:
		return a, commands.LoadVersionsCmd(a.ctx, a.service, msg.PlanName, msg.SyncSource)
	case messages.SearchVersionsMsg:
		return a, commands.SearchVersionsCmd(a.ctx, a.service, msg.PlanName, msg.SyncSource, msg.Query)
	case messages.ClearSearchMsg:
		return a, commands.LoadPlansCmd(a.ctx, a.service)
	case messages.SearchPlansMsg:
		return a, commands.SearchPlansCmd(a.ctx, a.service, msg.Query)
	case messages.SearchPlansWithTagsMsg:
		return a, commands.SearchPlansWithTagsCmd(a.ctx, a.service, msg.Query, msg.Tags, msg.MatchAll)
	case messages.LoadUntaggedPlansMsg:
		return a, commands.LoadUntaggedPlansCmd(a.ctx, a.service)
	case messages.LoadTagsForModalMsg:
		return a, commands.LoadTagsForModalCmd(a.ctx, a.service, msg.FileName, msg.SyncSource)
	case messages.LoadAllTagsForPanelMsg:
		return a, commands.LoadAllTagsForPanelCmd(a.ctx, a.service)
	case messages.LoadSettingsMsg:
		return a, commands.LoadSettingsCmd(a.ctx, a.service, msg.SettingNames)
	case messages.SaveSettingMsg:
		return a, commands.SetSettingCmd(a.ctx, a.service, msg.Name, msg.Values)
	case messages.LoadConnectorsMsg:
		return a, commands.LoadConnectorsCmd(a.ctx, a.service)
	case messages.LoadConnectorSettingsMsg:
		return a, commands.LoadConnectorSettingsCmd(a.ctx, a.service, msg.ConnectorName)
	case messages.SaveResultMsg:
		return a.handleSaveResult(msg)
	case messages.SyncPlansMsg:
		return a.handleSyncPlans(msg)
	case messages.SyncResultMsg:
		return a.handleSyncResult(msg)
	case messages.RSyncPlansMsg:
		return a.handleRSyncPlans(msg)
	case messages.RSyncResultMsg:
		return a.handleRSyncResult(msg)
	case messages.DumpPlansMsg:
		return a.handleDumpPlans(msg)
	case messages.DumpResultMsg:
		return a.handleDumpResult(msg)
	case messages.DeletePlanMsg:
		return a.handleDeletePlan(msg)
	case messages.DeletePlanResultMsg:
		return a.handleDeletePlanResult(msg)
	case messages.RenamePlanFileMsg:
		return a.handleRenamePlanFile(msg)
	case messages.RenamePlanFileResultMsg:
		return a.handleRenamePlanFileResult(msg)
	case messages.CopyToClipboardMsg:
		return a.handleCopyToClipboard(msg)
	case messages.CopyToClipboardResultMsg:
		return a.handleCopyToClipboardResult(msg)
	case messages.WatchResultMsg:
		return a.handleWatchResult(msg)
	case messages.WatchModeApplyMsg:
		return a.handleWatchModeApply(msg)
	case messages.OpenSettingsMsg:
		return a, a.pushSettingsScreen()
	case messages.SettingUpdateResultMsg:
		return a.handleSettingUpdateResult(msg)
	case messages.ThemeChangedMsg:
		return a.handleThemeChanged(msg)
	case messages.RenderMarkDownByDefaultMsg:
		return a.handleRenderMarkdownChanged(msg)
	case messages.MarkdownRenderedThemeChangedMsg:
		return a.handleMarkdownRenderedThemeChanged(msg)
	case messages.DisplayModeChangedMsg:
		return a.handleDisplayModeChanged(msg)
	case messages.ScreenOrientationChangedMsg:
		return a.handleScreenOrientationChanged(msg)
	case messages.PlansSortKeyChangedMsg, messages.PlansSortDirChangedMsg:
		return a, a.reloadPlans()
	case messages.OpenConnectorsMsg:
		return a, a.pushConnectorsScreen()
	case messages.EnableConnectorMsg:
		return a.handleEnableConnector(msg)
	case messages.DisableConnectorMsg:
		return a.handleDisableConnector(msg)
	case messages.ClearSummaryConnectorMsg:
		return a.handleClearSummaryConnector(msg)
	case messages.SaveConnectorSettingMsg:
		return a.handleSaveConnectorSetting(msg)
	case messages.ConnectorUpdateResultMsg:
		return a.handleConnectorUpdateResult(msg)
	case messages.ValidateConnectorMsg:
		return a.handleValidateConnector(msg)
	case messages.ValidateConnectorResultMsg:
		return a.handleValidateConnectorResult(msg)
	case messages.SendToConnectorMsg:
		return a.handleSendToConnector(msg)
	case messages.SendToConnectorResultMsg:
		return a.handleSendToConnectorResult(msg)
	case messages.GenerateTLDRMsg:
		return a.handleGenerateTLDR(msg)
	case messages.RegenerateTLDRMsg:
		return a.handleRegenerateTLDR(msg)
	case messages.TLDRGeneratedMsg:
		return a.handleTLDRGenerated(msg)
	case messages.SetSummaryConnectorMsg:
		return a.handleSetSummaryConnector(msg)
	case messages.CreateTagMsg:
		return a.handleCreateTag(msg)
	case messages.CreateTagResultMsg:
		return a.handleCreateTagResult(msg)
	case messages.SetPlanTagsMsg:
		return a.handleSetPlanTags(msg)
	case components.DeleteTagMsg:
		return a.handleDeleteTag(msg)
	case components.DeleteTagCmdMsg:
		return a.handleDeleteTagResult(msg)
	case messages.RequestVersionsScreenMsg:
		return a.handleRequestVersionsScreen(msg)
	case messages.VersionsNavigationResultMsg:
		return a.handleVersionsNavigation(msg)
	case messages.RestoreVersionMsg:
		return a.handleRestoreVersion(msg)
	case messages.RestoreResultMsg:
		return a.handleRestoreResult(msg)
	case messages.OpenCommentModalMsg:
		return a.handleOpenCommentModal(msg)
	case messages.CommentsLoadedMsg:
		return a.handleCommentsLoaded(msg)
	case messages.AddCommentMsg:
		return a.handleAddComment(msg)
	case messages.AddCommentResultMsg:
		return a.handleAddCommentResult(msg)
	case messages.DeleteCommentMsg:
		return a.handleDeleteComment(msg)
	case messages.DeleteCommentResultMsg:
		return a.handleDeleteCommentResult(msg)
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

// delegateToPlansScreen routes a message directly to the PlansScreen wherever it sits in the
// stack. This is needed for messages like PlansLoadedMsg that must reach the plans screen even
// when another screen (e.g. settings) is on top. Falls back to the current screen if no
// PlansScreen is found.
func (a *App) delegateToPlansScreen(msg tea.Msg) (tea.Model, tea.Cmd) {
	for i, screen := range a.stack {
		if _, ok := screen.(*screens.PlansScreen); ok {
			newScreen, cmd := screen.Update(msg)
			a.stack[i] = newScreen
			return a, cmd
		}
	}
	return a.delegateToCurrentScreen(msg)
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

// reloadPlans refetches the plan list the way the active display mode needs.
// This is the only place that maps a display mode to a reload command — callers
// stay mode-agnostic.
func (a *App) reloadPlans() tea.Cmd {
	if a.displayMode == busquets.DisplayModeTagPlanContent {
		return commands.LoadAllTagsForPanelCmd(a.ctx, a.service)
	}
	return commands.LoadPlansCmd(a.ctx, a.service)
}

// Navigation helpers.
func (a *App) popScreen() tea.Cmd { //nolint:unparam // ok for now.
	if len(a.stack) > 1 {
		a.stack = a.stack[:len(a.stack)-1]
	}
	return nil
}

func (a *App) pushVersionsScreenWithData(planID int64, planName string, versions []busquets.PlanVersionDetail) tea.Cmd {
	versionsScreen := screens.NewVersionsScreenWithData(planID, planName, a.markdownRenderedTheme, versions, a.width, a.height, a.isDarkModeEnabled, a.renderMarkDownByDefault, a.screenOrientation)
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
