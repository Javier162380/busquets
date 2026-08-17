package tui

import (
	"github.com/Javier162380/busquets/cmd/tui/commands"
	"github.com/Javier162380/busquets/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) handleCopyToClipboard(msg messages.CopyToClipboardMsg) (tea.Model, tea.Cmd) {
	return a, commands.CopyToClipboardCmd(a.ctx, a.service, msg.Text, msg.Label)
}

func (a *App) handleCopyToClipboardResult(msg messages.CopyToClipboardResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Failed to copy: " + msg.Error.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	a.statusBar.SetSuccess("Copied \"" + msg.Label + "\" to clipboard")
	return a, commands.ClearStatusCmdWithDefaultDuration()
}
