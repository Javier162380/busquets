package tui

import (
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/commands"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) handleEnableConnector(msg messages.EnableConnectorMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Enabling connector...")
	return a, commands.EnableConnectorCmd(a.ctx, a.service, msg.Name)
}

func (a *App) handleDisableConnector(_ messages.DisableConnectorMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Disabling connector...")
	return a, commands.DisableConnectorCmd(a.ctx, a.service)
}

func (a *App) handleSaveConnectorSetting(msg messages.SaveConnectorSettingMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Saving...")
	return a, commands.SaveConnectorSettingCmd(a.ctx, a.service, msg.ConnectorName, msg.Key, msg.Value, msg.IsSecret)
}

func (a *App) handleConnectorUpdateResult(msg messages.ConnectorUpdateResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError(msg.Error.Error())
	} else {
		a.statusBar.SetSuccess("Connector updated")
	}
	return a.delegateToCurrentScreen(msg)
}

func (a *App) handleValidateConnector(msg messages.ValidateConnectorMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Validating...")
	return a, commands.ValidateConnectorCmd(a.ctx, a.service, msg.Name)
}

func (a *App) handleValidateConnectorResult(msg messages.ValidateConnectorResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Validation failed: " + msg.Error.Error())
	} else {
		a.statusBar.SetSuccess("Connector validated successfully!")
	}
	return a, commands.ClearStatusCmd(1 * time.Second)
}

func (a *App) handleSendToConnector(msg messages.SendToConnectorMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Sending to connector...")
	return a, commands.SendToConnectorCmd(a.ctx, a.service, msg.PlanFileName)
}

func (a *App) handleSendToConnectorResult(msg messages.SendToConnectorResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Send failed: " + msg.Error.Error())
	} else {
		a.statusBar.SetSuccess("Sent successfully!")
	}
	return a, commands.ClearStatusCmd(1 * time.Second)
}
