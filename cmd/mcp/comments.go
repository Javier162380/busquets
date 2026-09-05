package mcp

import (
	"context"
	"fmt"

	"github.com/Javier162380/busquets/services/busquets"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerCommentTools registers all comment-related MCP tools.
func (h *Handler) registerCommentTools() error {
	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "add_comment",
		Description: "Adds a comment to a plan.",
	}, h.AddPlanComment)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "get_plan_comments",
		Description: "Lists all comments on a plan, oldest first.",
	}, h.GetPlanComments)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "delete_comment",
		Description: "Deletes a comment by its ID (from get_plan_comments results).",
	}, h.DeletePlanComment)

	return nil
}

// AddPlanComment handles the add_comment tool.
func (h *Handler) AddPlanComment(ctx context.Context, req *mcp.CallToolRequest, args AddPlanCommentArgs) (*mcp.CallToolResult, *busquets.Comment, error) {
	if args.Comment == "" {
		return nil, nil, fmt.Errorf("invalid argument, comment can't be empty")
	}

	if args.FileName == "" {
		return nil, nil, fmt.Errorf("fileName is required")
	}

	if args.SyncSource == "" {
		return nil, nil, fmt.Errorf("syncSource is required")
	}

	comment, err := h.service.AddComment(ctx, args.FileName, args.SyncSource, args.Comment)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to add comment: %w", err)
	}

	toonOutput, err := FormatAddPlanComment(comment)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to format results: %w", err)
	}
	return buildMCPResult(toonOutput), &comment, nil
}

// GetPlanComments handles the get_plan_comments tool. Returns PlanCommentsResult (not a
// bare slice) so it has a real output schema.
func (h *Handler) GetPlanComments(ctx context.Context, req *mcp.CallToolRequest, args GetPlanCommentsArgs) (*mcp.CallToolResult, PlanCommentsResult, error) {
	if args.FileName == "" {
		return nil, PlanCommentsResult{}, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, PlanCommentsResult{}, fmt.Errorf("syncSource is required")
	}

	comments, err := h.service.GetPlanComments(ctx, args.FileName, args.SyncSource)
	if err != nil {
		return nil, PlanCommentsResult{}, fmt.Errorf("failed to get comments: %w", err)
	}

	toonOutput, err := FormatPlanComments(comments)
	if err != nil {
		return nil, PlanCommentsResult{}, fmt.Errorf("failed to format comments: %w", err)
	}
	return buildMCPResult(toonOutput), PlanCommentsResult{Comments: comments}, nil
}

// DeletePlanComment handles the delete_comment tool.
func (h *Handler) DeletePlanComment(ctx context.Context, req *mcp.CallToolRequest, args DeleteCommentArgs) (*mcp.CallToolResult, any, error) {
	if args.CommentID == 0 {
		return nil, nil, fmt.Errorf("commentId is required")
	}
	if err := h.service.DeleteComment(ctx, args.CommentID); err != nil {
		return nil, nil, fmt.Errorf("failed to delete comment: %w", err)
	}
	return buildMCPResult(fmt.Sprintf("deleted comment %d", args.CommentID)), nil, nil
}
