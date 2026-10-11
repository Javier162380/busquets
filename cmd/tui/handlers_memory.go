package tui

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/Javier162380/busquets/cmd/tui/commands"
	"github.com/Javier162380/busquets/cmd/tui/messages"
	"github.com/Javier162380/busquets/cmd/tui/screens"

	tea "github.com/charmbracelet/bubbletea"
)

// memoryStatusDuration is how long a memory status stays in the status bar.
// Longer than the default so a failure like "Memory failed: ..." can be read.
const memoryStatusDuration = 3 * time.Second

// handleRequestMemoryScreen opens the memory screen for a plan.
//
// Unlike versions, this does not check for data first: the timeline is computed,
// so the screen is useful even when no memory has been written.
func (a *App) handleRequestMemoryScreen(msg messages.RequestMemoryScreenMsg) (tea.Model, tea.Cmd) {
	return a, commands.LoadMemoryCmd(a.ctx, a.service, msg.PlanName, msg.SyncSource)
}

func (a *App) handleMemoryLoaded(msg messages.MemoryLoadedMsg) (tea.Model, tea.Cmd) {
	// Already on the memory screen (a reload): let it update in place.
	if _, ok := a.stack[len(a.stack)-1].(*screens.MemoryScreen); ok {
		return a.delegateToCurrentScreen(msg)
	}
	return a, a.pushMemoryScreen(msg)
}

func (a *App) handleGenerateMemory(msg messages.GenerateMemoryMsg) (tea.Model, tea.Cmd) {
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelMemoryGeneration = cancel

	cmds := []tea.Cmd{
		commands.GenerateMemoryCmd(ctx, a.service, msg.PlanName, msg.SyncSource, msg.Mode),
		commands.MemoryChannelListenerCmd(a.ctx, a.service),
	}
	a.statusBar.SetLoading("Writing memory...")
	cmds = append(cmds, commands.ClearStatusCmd(memoryStatusDuration))
	return a, tea.Batch(cmds...)
}

func (a *App) handleCancelMemoryGeneration() (tea.Model, tea.Cmd) {
	if a.cancelMemoryGeneration == nil {
		return a, nil
	}
	a.cancelMemoryGeneration()
	a.statusBar.SetLoading("Cancelling memory...")
	return a, commands.ClearStatusCmd(memoryStatusDuration)
}

func (a *App) handleMemoryProgress(msg messages.MemoryProgressMsg) (tea.Model, tea.Cmd) {
	if msg.Done {
		// The terminal update is reported by MemoryGeneratedMsg, which carries
		// the saved memory; stop listening here.
		return a, nil
	}
	cmds := []tea.Cmd{commands.MemoryChannelListenerCmd(a.ctx, a.service)}
	if msg.Total > 0 {
		a.statusBar.SetLoading(fmt.Sprintf("Writing memory... event %d/%d", msg.Current, msg.Total))
		cmds = append(cmds, commands.ClearStatusCmd(memoryStatusDuration))
	}
	// Keep listening for the next update.
	return a, tea.Batch(cmds...)
}

func (a *App) handleMemoryGenerated(msg messages.MemoryGeneratedMsg) (tea.Model, tea.Cmd) {
	if a.cancelMemoryGeneration != nil {
		a.cancelMemoryGeneration()
		a.cancelMemoryGeneration = nil
	}

	if msg.Err != nil {
		if errors.Is(msg.Err, context.Canceled) {
			a.statusBar.SetSuccess("Memory cancelled, nothing was saved")
		} else {
			a.statusBar.SetError("Memory failed: " + msg.Err.Error())
		}
		// Clear the screen's in-flight flag so the key works again.
		if screen, ok := a.stack[len(a.stack)-1].(*screens.MemoryScreen); ok {
			screen.SetGenerating(false)
		}
		return a, commands.ClearStatusCmd(memoryStatusDuration)
	}

	model, cmd := a.delegateToCurrentScreen(msg)
	cmds := []tea.Cmd{cmd}
	a.statusBar.SetSuccess("Memory updated")
	cmds = append(cmds, commands.ClearStatusCmd(memoryStatusDuration))
	return model, tea.Batch(cmds...)
}

func (a *App) handleDeleteMemory(msg messages.DeleteMemoryMsg) (tea.Model, tea.Cmd) {
	cmds := []tea.Cmd{
		commands.DeleteMemoryCmd(a.ctx, a.service, msg.PlanName, msg.SyncSource),
		func() tea.Msg {
			return messages.RequestMemoryScreenMsg{PlanName: msg.PlanName, SyncSource: msg.SyncSource}
		},
	}
	a.statusBar.SetLoading("Deleting memory...")
	cmds = append(cmds, commands.ClearStatusCmd(memoryStatusDuration))
	return a, tea.Batch(cmds...)
}

func (a *App) handleDeleteMemoryResult(messages.DeleteMemoryResultMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetSuccess("Memory deleted")
	return a, commands.ClearStatusCmd(memoryStatusDuration)
}

func (a *App) handleRequestMemoryDiff(msg messages.RequestMemoryDiffMsg) (tea.Model, tea.Cmd) {
	return a, commands.MemoryDiffCmd(a.ctx, a.service, msg.PlanName, msg.SyncSource, msg.FromVersion, msg.ToVersion)
}

func (a *App) handleMemoryDiffLoaded(msg messages.MemoryDiffLoadedMsg) (tea.Model, tea.Cmd) {
	if msg.Err != nil {
		a.statusBar.SetError("Diff failed: " + msg.Err.Error())
		return a, commands.ClearStatusCmd(memoryStatusDuration)
	}
	return a.delegateToCurrentScreen(msg)
}
