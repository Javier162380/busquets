package tui

import (
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/commands"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) handleEnableConnector(msg messages.EnableConnectorMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Enabling connector...")
	return a, tea.Batch(
		commands.EnableConnectorCmd(a.ctx, a.service, msg.Name),
		commands.ClearStatusCmdWithDefaultDuration(),
	)
}

func (a *App) handleDisableConnector(_ messages.DisableConnectorMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Clearing transmit connector...")
	return a, tea.Batch(
		commands.DisableConnectorCmd(a.ctx, a.service),
		commands.ClearStatusCmdWithDefaultDuration(),
	)
}

func (a *App) handleClearSummaryConnector(_ messages.ClearSummaryConnectorMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Clearing summary connector...")
	return a, tea.Batch(
		commands.ClearSummaryConnectorCmd(a.ctx, a.service),
		commands.ClearStatusCmdWithDefaultDuration(),
	)
}

func (a *App) handleSaveConnectorSetting(msg messages.SaveConnectorSettingMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Saving...")
	return a, tea.Batch(
		commands.SaveConnectorSettingCmd(a.ctx, a.service, msg.ConnectorName, msg.Key, msg.Value, msg.IsSecret),
		commands.ClearStatusCmdWithDefaultDuration(),
	)
}

func (a *App) handleConnectorUpdateResult(msg messages.ConnectorUpdateResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError(msg.Error.Error())
	} else {
		a.statusBar.SetSuccess("Connector updated")
	}
	model, cmd := a.delegateToCurrentScreen(msg) //nolint:gci//no need.
	return model, tea.Batch(cmd, commands.ClearStatusCmdWithDefaultDuration())
}

func (a *App) handleValidateConnector(msg messages.ValidateConnectorMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Validating...")
	return a, tea.Batch(
		commands.ValidateConnectorCmd(a.ctx, a.service, msg.Name),
		commands.ClearStatusCmdWithDefaultDuration(),
	)
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
	return a, tea.Batch(
		commands.SendToConnectorCmd(a.ctx, a.service, msg.PlanFileName),
		commands.ClearStatusCmdWithDefaultDuration(),
	)
}

func (a *App) handleSendToConnectorResult(msg messages.SendToConnectorResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Send failed: " + msg.Error.Error())
	} else {
		a.statusBar.SetSuccess("Sent successfully!")
	}
	return a, commands.ClearStatusCmd(1 * time.Second)
}

func (a *App) handleGenerateTLDR(msg messages.GenerateTLDRMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Generating summary...")
	return a, commands.GenerateTLDRCmd(a.ctx, a.service, msg.FileName)
}

func (a *App) handleRegenerateTLDR(msg messages.RegenerateTLDRMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Regenerating summary...")
	return a, commands.RegenerateTLDRCmd(a.ctx, a.service, msg.FileName)
}

func (a *App) handleTLDRGenerated(msg messages.TLDRGeneratedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		a.statusBar.SetError("Summary failed: " + msg.Err.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	model, cmd := a.delegateToCurrentScreen(msg)
	return model, tea.Batch(
		cmd,
		commands.ClearStatusCmdWithDefaultDuration(),
	)
}

func (a *App) handleSetSummaryConnector(msg messages.SetSummaryConnectorMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Setting summarizer...")
	return a, tea.Batch(
		commands.SetSummaryConnectorCmd(a.ctx, a.service, msg.ConnectorName),
		commands.ClearStatusCmdWithDefaultDuration(),
	)
}
