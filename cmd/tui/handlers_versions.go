package tui

import (
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/commands"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) handleRequestVersionsScreen(msg messages.RequestVersionsScreenMsg) (tea.Model, tea.Cmd) {
	return a, commands.LoadVersionsForNavigationCmd(a.ctx, a.service, msg.PlanName)
}

func (a *App) handleVersionsNavigation(msg messages.VersionsNavigationResultMsg) (tea.Model, tea.Cmd) {
	if len(msg.Versions) == 0 {
		a.statusBar.SetError("No versions found")
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	return a, a.pushVersionsScreenWithData(msg.PlanName, msg.Versions)
}

func (a *App) handleRestoreVersion(msg messages.RestoreVersionMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Restoring version...")
	return a, commands.RestoreVersionCmd(a.ctx, a.service, msg.PlanName, msg.VersionNumber)
}

func (a *App) handleRestoreResult(msg messages.RestoreResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Restore failed: " + msg.Error.Error())
		return a, nil
	}
	a.statusBar.SetSuccess("Version restored successfully")
	a.popScreen()
	return a, tea.Batch(
		commands.SyncPlansCmd(a.ctx, a.service),
		commands.LoadPlansCmd(a.ctx, a.service),
	)
}
