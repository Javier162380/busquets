package tui

import (
	"fmt"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/commands"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
)

func (a *App) handleOpenCommentModal(msg messages.OpenCommentModalMsg) (tea.Model, tea.Cmd) {
	return a, commands.LoadCommentsCmd(a.ctx, a.service, msg.FileName, msg.SyncSource)
}

func (a *App) handleCommentsLoaded(msg messages.CommentsLoadedMsg) (tea.Model, tea.Cmd) {
	return a.delegateToPlansScreen(msg)
}

func (a *App) handleAddComment(msg messages.AddCommentMsg) (tea.Model, tea.Cmd) {
	a.statusBar.SetLoading("Saving comment...")
	return a, commands.AddCommentCmd(a.ctx, a.service, msg.FileName, msg.SyncSource, msg.Content)
}

func (a *App) handleAddCommentResult(msg messages.AddCommentResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError(fmt.Sprintf("Failed to add comment: %s", msg.Error.Error()))
		return a, commands.ClearStatusCmd(2 * time.Second)
	}
	a.statusBar.SetSuccess("Comment added")
	return a, tea.Batch(
		commands.ClearStatusCmdWithDefaultDuration(),
		commands.LoadCommentsCmd(a.ctx, a.service, msg.FileName, msg.SyncSource),
		commands.LoadPlansCmd(a.ctx, a.service),
	)
}

func (a *App) handleDeleteComment(msg messages.DeleteCommentMsg) (tea.Model, tea.Cmd) {
	return a, commands.DeleteCommentCmd(a.ctx, a.service, msg.CommentID, msg.FileName, msg.SyncSource)
}

func (a *App) handleDeleteCommentResult(msg messages.DeleteCommentResultMsg) (tea.Model, tea.Cmd) {
	if msg.Error != nil {
		a.statusBar.SetError(fmt.Sprintf("Failed to delete comment: %s", msg.Error.Error()))
		return a, commands.ClearStatusCmd(2 * time.Second)
	}
	a.statusBar.SetSuccess("Comment deleted")
	return a, tea.Batch(
		commands.ClearStatusCmd(1*time.Second),
		commands.LoadCommentsCmd(a.ctx, a.service, msg.FileName, msg.SyncSource),
		commands.LoadPlansCmd(a.ctx, a.service),
	)
}
