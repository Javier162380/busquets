package tui

import (
	"fmt"

	"github.com/Javier162380/busquets/cmd/tui/commands"
	"github.com/Javier162380/busquets/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) handleRequestVersionsScreen(msg messages.RequestVersionsScreenMsg) (tea.Model, tea.Cmd) {
	return a, commands.LoadVersionsForNavigationCmd(a.ctx, a.service, msg.PlanName, msg.SyncSource)
}

func (a *App) handleVersionsNavigation(msg messages.VersionsNavigationResultMsg) (tea.Model, tea.Cmd) {
	if len(msg.Versions) == 0 {
		a.statusBar.SetError("No versions found")
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	return a, a.pushVersionsScreenWithData(msg.PlanID, msg.PlanName, msg.Versions)
}

func (a *App) handleRestoreVersion(msg messages.RestoreVersionMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Restoring version...")
	return a, commands.RestoreVersionCmd(a.ctx, a.service, msg.PlanID, msg.PlanName, msg.VersionNumber)
}

func (a *App) handleRestoreResult(msg messages.RestoreResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Restore failed: " + msg.Error.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	a.statusBar.SetSuccess("Version restored successfully")
	a.popScreen()
	return a, tea.Batch(
		a.reloadPlans(),
		commands.ClearStatusCmdWithDefaultDuration(),
	)
}

func (a *App) handleSetVersionMilestones(msg messages.SetVersionMilestonesMsg) (tea.Model, tea.Cmd) {
	return a, commands.SetVersionMilestonesCmd(a.ctx, a.service, msg.PlanID, msg.VersionNumber, msg.Milestones)
}

// handleVersionMilestonesSaved confirms the change and lets the versions screen update the row in place.
func (a *App) handleVersionMilestonesSaved(msg messages.VersionMilestonesSavedMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetSuccess(fmt.Sprintf("Milestones saved on version %d", msg.Version.VersionNumber))
	model, cmd := a.delegateToCurrentScreen(msg)
	return model, tea.Batch(cmd, commands.ClearStatusCmdWithDefaultDuration())
}
