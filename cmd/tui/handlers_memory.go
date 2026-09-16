package tui

import (
	"fmt"

	"github.com/Javier162380/busquets/cmd/tui/commands"
	"github.com/Javier162380/busquets/cmd/tui/messages"
	"github.com/Javier162380/busquets/cmd/tui/screens"

	tea "github.com/charmbracelet/bubbletea"
)

// handleRequestMemoryScreen opens the memory screen for a plan.
//
// Unlike versions, this does not check for data first: the timeline is computed,
// so the screen is useful even when no memory has been written.
func (a *App) handleRequestMemoryScreen(msg messages.RequestMemoryScreenMsg) (tea.Model, tea.Cmd) {
	return a, commands.LoadMemoryCmd(a.ctx, a.service, msg.PlanName, msg.SyncSource)
}

func (a *App) handleMemoryLoaded(msg messages.MemoryLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		a.statusBar.SetError("Failed to load memory: " + msg.Err.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}

	// Already on the memory screen (a reload): let it update in place.
	if _, ok := a.stack[len(a.stack)-1].(*screens.MemoryScreen); ok {
		return a.delegateToCurrentScreen(msg)
	}
	return a, a.pushMemoryScreen(msg)
}

func (a *App) handleGenerateMemory(msg messages.GenerateMemoryMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Writing memory...")
	return a, tea.Batch(
		commands.GenerateMemoryCmd(a.ctx, a.service, msg.PlanName, msg.SyncSource, msg.Mode),
		commands.MemoryChannelListenerCmd(a.ctx, a.service),
	)
}

func (a *App) handleMemoryProgress(msg messages.MemoryProgressMsg) (tea.Model, tea.Cmd) {
	if msg.Done {
		// The terminal update is reported by MemoryGeneratedMsg, which carries
		// the saved memory; stop listening here.
		return a, nil
	}
	if msg.Total > 0 {
		a.statusBar.SetLoading(fmt.Sprintf("Writing memory... event %d/%d", msg.Current, msg.Total))
	}
	// Keep listening for the next update.
	return a, commands.MemoryChannelListenerCmd(a.ctx, a.service)
}

func (a *App) handleMemoryGenerated(msg messages.MemoryGeneratedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		a.statusBar.SetError("Memory failed: " + msg.Err.Error())
		// Clear the screen's in-flight flag so the key works again.
		if screen, ok := a.stack[len(a.stack)-1].(*screens.MemoryScreen); ok {
			screen.SetGenerating(false)
		}
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}

	a.statusBar.SetSuccess("Memory updated")
	model, cmd := a.delegateToCurrentScreen(msg)
	return model, tea.Batch(cmd, commands.ClearStatusCmdWithDefaultDuration())
}

func (a *App) handleDeleteMemory(msg messages.DeleteMemoryMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Deleting memory...")
	return a, tea.Batch(
		commands.DeleteMemoryCmd(a.ctx, a.service, msg.PlanName, msg.SyncSource),
		func() tea.Msg {
			return messages.RequestMemoryScreenMsg{PlanName: msg.PlanName, SyncSource: msg.SyncSource}
		},
	)
}

func (a *App) handleDeleteMemoryResult(msg messages.DeleteMemoryResultMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		a.statusBar.SetError("Delete failed: " + msg.Err.Error())
	} else {
		a.statusBar.SetSuccess("Memory deleted")
	}
	return a, commands.ClearStatusCmdWithDefaultDuration()
}

func (a *App) handleRequestMemoryDiff(msg messages.RequestMemoryDiffMsg) (tea.Model, tea.Cmd) {
	return a, commands.MemoryDiffCmd(a.ctx, a.service, msg.PlanName, msg.SyncSource, msg.FromVersion, msg.ToVersion)
}

func (a *App) handleMemoryDiffLoaded(msg messages.MemoryDiffLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		a.statusBar.SetError("Diff failed: " + msg.Err.Error())
		return a, commands.ClearStatusCmdWithDefaultDuration()
	}
	return a.delegateToCurrentScreen(msg)
}
