package tui

import (
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/commands"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) handleCopyPlanContent(msg messages.CopyPlanContentMsg) (tea.Model, tea.Cmd) {
	return a, commands.CopyPlanContentCmd(a.ctx, a.service, msg.FileName, msg.SyncSource)
}

func (a *App) handleCopyPlanContentResult(msg messages.CopyPlanContentResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Failed to copy: " + msg.Error.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	a.statusBar.SetSuccess("Copied \"" + msg.FileName + "\" to clipboard")
	return a, commands.ClearStatusCmdWithDefaultDuration()
}
