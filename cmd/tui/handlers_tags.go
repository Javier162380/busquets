package tui

import (
	"fmt"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/commands"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) handleCreateTag(msg messages.CreateTagMsg) (tea.Model, tea.Cmd) {
	return a, commands.CreateTagCmd(a.ctx, a.service, msg.Name)
}

func (a *App) handleCreateTagResult(msg messages.CreateTagResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError("Failed to create tag: " + msg.Error.Error())
		return a, commands.ClearStatusCmd(2 * time.Second)
	}
	a.statusBar.SetSuccess("Tag created")
	return a, tea.Batch(
		a.reloadPlans(),
		commands.ClearStatusCmd(1*time.Second),
	)
}

func (a *App) handleSetPlanTags(msg messages.SetPlanTagsMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Saving tags...")
	return a, tea.Batch(
		commands.SetPlanTagsCmd(a.ctx, a.service, msg.FileName, msg.SyncSource, msg.Tags),
		commands.ClearStatusCmd(1*time.Second),
	)
}

func (a *App) handleDeleteTag(msg components.DeleteTagMsg) (tea.Model, tea.Cmd) {
	return a, commands.DeleteTagsCmd(a.ctx, a.service, msg.TagID, msg.CurrentPlan)
}

func (a *App) handleDeleteTagResult(msg components.DeleteTagCmdMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError(fmt.Sprintf("Failed to delete tag %s", *msg.Error))
	} else {
		a.statusBar.SetSuccess("Deleted tag")
	}
	return a, tea.Batch(
		commands.ClearStatusCmdWithDefaultDuration(),
		a.reloadPlans(),
	)
}
