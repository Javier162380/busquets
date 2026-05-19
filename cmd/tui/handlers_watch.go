package tui

import (
	"fmt"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/commands"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) handleWatchResult(msg messages.WatchResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError(fmt.Sprintf("Auto-sync failed: %v", msg.Error))
	} else if msg.Count > 0 {
		a.statusBar.SetSuccess(fmt.Sprintf("Auto-synced %d plans", msg.Count))
		return a, tea.Batch(
			commands.LoadPlansCmd(a.ctx, a.service, a.plansSortKey, a.plansSortDir),
			commands.WatchChannelListenerCmd(a.ctx, a.service),
			commands.ClearStatusCmd(1*time.Second),
		)
	}
	return a, commands.WatchChannelListenerCmd(a.ctx, a.service)
}

func (a *App) handleWatchModeApply(msg messages.WatchModeApplyMsg) (tea.Model, tea.Cmd) {
	if msg.Enabled {
		_ = a.service.StartWatchMode(a.ctx, msg.Interval)
		a.statusBar.SetSuccess("Watch mode enabled")
	} else {
		_ = a.service.StopWatchMode(a.ctx)
		a.statusBar.SetSuccess("Watch mode disabled")
	}
	return a, commands.ClearStatusCmdWithDefaultDuration()
}
