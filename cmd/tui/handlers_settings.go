package tui

import (
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/commands"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/screens"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) handleSettingUpdateResult(msg messages.SettingUpdateResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Failed to save setting: " + msg.Error.Error())
	} else {
		a.statusBar.SetSuccess("Setting saved")
	}

	if msg.Success && (msg.SettingName == claudeviewer.SettingWatchModeEnabled ||
		msg.SettingName == claudeviewer.SettingWatchIntervalSeconds) {
		model, cmd := a.delegateToCurrentScreen(msg)
		return model, tea.Batch(cmd, commands.ApplyWatchSettingCmd(a.ctx, a.service))
	}

	model, cmd := a.delegateToCurrentScreen(msg)
	return model, tea.Batch(cmd, commands.ClearStatusCmdWithDefaultDuration())
}

func (a *App) handleThemeChanged(msg messages.ThemeChangedMsg) (tea.Model, tea.Cmd) {
	styles.SetDarkMode(msg.DarkMode)
	a.isDarkModeEnabled = msg.DarkMode
	for i, screen := range a.stack {
		updated, _ := screen.Update(msg)
		a.stack[i] = updated
	}
	return a, nil
}

func (a *App) handleRenderMarkdownChanged(msg messages.RenderMarkDownByDefaultMsg) (tea.Model, tea.Cmd) {
	a.renderMarkDownByDefault = msg.Enabled
	for i, screen := range a.stack {
		updated, _ := screen.Update(msg)
		a.stack[i] = updated
	}
	return a, nil
}

func (a *App) handleDisplayModeChanged(msg messages.DisplayModeChangedMsg) (tea.Model, tea.Cmd) {
	a.displayMode = msg.Mode
	for _, screen := range a.stack {
		if s, ok := screen.(*screens.PlansScreen); ok {
			s.SetDisplayMode(msg.Mode)
		}
	}
	// Only the tag panel needs a fetch on a mode switch. Labels are static and
	// SetDisplayMode rebuilds the label panel synchronously from the plans the
	// screen already holds.
	if msg.Mode == claudeviewer.DisplayModeTagPlanContent {
		return a, commands.LoadAllTagsForPanelCmd(a.ctx, a.service)
	}
	return a, nil
}
