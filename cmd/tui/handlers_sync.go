package tui

import (
	"fmt"
	"time"

	"github.com/Javier162380/busquets/cmd/tui/commands"
	"github.com/Javier162380/busquets/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) handleSaveResult(msg messages.SaveResultMsg) (tea.Model, tea.Cmd) {
	switch {
	case msg.Error != nil:
		a.statusBar.SetError("Unable to save result")
	case msg.Result.Success:
		a.statusBar.SetSuccess("Saved result")
	case msg.Result.HasConflict:
		a.statusBar.SetError(fmt.Sprintf("Unable to save result, conflict %s", msg.Result.ConflictInfo.Message))
	}
	_, delegateCmd := a.delegateToCurrentScreen(msg)
	return a, tea.Batch(delegateCmd, commands.ClearStatusCmdWithDefaultDuration())
}

func (a *App) handleStashResult(msg messages.StashResultMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetSuccess(fmt.Sprintf("Stash result for plan at %s", msg.FilePath))
	return a, commands.ClearStatusCmdWithDefaultDuration()
}

func (a *App) handleSyncPlans(_ messages.SyncPlansMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Syncing plans...")
	return a, commands.SyncPlansCmd(a.ctx, a.service)
}

func (a *App) handleSyncResult(msg messages.SyncResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Sync failed: " + msg.Error.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	a.statusBar.SetSuccess(fmt.Sprintf("Synced %d plans", msg.Count))
	return a, tea.Batch(commands.LoadPlansCmd(a.ctx, a.service), commands.ClearStatusCmd(1*time.Second))
}

func (a *App) handleRSyncPlans(_ messages.RSyncPlansMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Rsyncing plans from remote directory into the LLM directory...")
	return a, commands.RsyncPlansCmd(a.ctx, a.service)
}

func (a *App) handleRSyncResult(msg messages.RSyncResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("RSync failed: " + msg.Error.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	a.statusBar.SetSuccess(fmt.Sprintf("Rsync succeeded: %d plans sync from remote into the local directory", msg.Count))
	return a, tea.Batch(commands.LoadPlansCmd(a.ctx, a.service), commands.ClearStatusCmdWithDefaultDuration())
}

func (a *App) handleDumpPlans(_ messages.DumpPlansMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Dumping plans from database to source directory...")
	return a, commands.DumpPlansCmd(a.ctx, a.service)
}

func (a *App) handleDumpResult(msg messages.DumpResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Dump failed: " + msg.Error.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	a.statusBar.SetSuccess(fmt.Sprintf("Dumped %d plans to source directory", msg.Count))
	return a, commands.ClearStatusCmdWithDefaultDuration()
}

func (a *App) handleDeletePlan(msg messages.DeletePlanMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Deleting plan...")
	return a, tea.Batch(
		commands.DeletePlanCmd(a.ctx, a.service, msg.FileName, msg.SyncSource),
		commands.ClearStatusCmdWithDefaultDuration(),
	)
}

func (a *App) handleDeletePlanResult(msg messages.DeletePlanResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Failed to delete plan: " + msg.Error.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	a.statusBar.SetSuccess("Deleted plan")
	return a, tea.Batch(
		a.reloadPlans(),
		commands.ClearStatusCmd(1*time.Second),
	)
}

func (a *App) handleRenamePlanFile(msg messages.RenamePlanFileMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Renaming plan...")
	return a, tea.Batch(
		commands.RenamePlanFileCmd(a.ctx, a.service, msg.FileName, msg.SyncSource, msg.NewFileName),
		commands.ClearStatusCmdWithDefaultDuration(),
	)
}

func (a *App) handleRenamePlanFileResult(msg messages.RenamePlanFileResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Failed to rename plan: " + msg.Error.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	a.statusBar.SetSuccess("Renamed plan")
	return a, tea.Batch(
		a.reloadPlans(),
		commands.ClearStatusCmdWithDefaultDuration(),
	)
}
