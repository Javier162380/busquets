package tui

import (
	"fmt"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/commands"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"

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
