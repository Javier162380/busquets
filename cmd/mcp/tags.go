package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTagTools registers all tag-related MCP tools.
func (h *Handler) registerTagTools() error {
	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "get_all_tags",
		Description: "Lists every tag that exists in the system, across all plans.",
	}, h.GetAllTags)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "get_plan_tags",
		Description: "Lists the tags assigned to a specific plan.",
	}, h.GetPlanTags)

	mcp.AddTool(h.server, &mcp.Tool{
		Name: "set_plan_tags",
		Description: "Replaces ALL tags on a plan with the given list — not additive. " +
			"Include every tag the plan should end up with, including ones already there; " +
			"pass an empty list to clear all tags. To remove just one tag, use delete_tag instead.",
	}, h.SetPlanTags)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "delete_tag",
		Description: "Removes one tag from one plan by name (looked up via get_plan_tags). Does not affect the tag on any other plan.",
	}, h.DeleteTag)

	return nil
}

// GetAllTags handles the get_all_tags tool.
func (h *Handler) GetAllTags(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, TagsResult, error) {
	tags, err := h.service.GetAllTags(ctx)
	if err != nil {
		return nil, TagsResult{}, fmt.Errorf("failed to get tags: %w", err)
	}
	toonOutput, err := FormatTags(tags)
	if err != nil {
		return nil, TagsResult{}, fmt.Errorf("failed to format tags: %w", err)
	}
	return buildMCPResult(toonOutput), TagsResult{Tags: tags}, nil
}

// GetPlanTags handles the get_plan_tags tool.
func (h *Handler) GetPlanTags(ctx context.Context, req *mcp.CallToolRequest, args GetPlanTagsArgs) (*mcp.CallToolResult, TagsResult, error) {
	if args.FileName == "" {
		return nil, TagsResult{}, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, TagsResult{}, fmt.Errorf("syncSource is required")
	}

	tags, err := h.service.GetPlanTags(ctx, args.FileName, args.SyncSource)
	if err != nil {
		return nil, TagsResult{}, fmt.Errorf("failed to get tags: %w", err)
	}
	toonOutput, err := FormatTags(tags)
	if err != nil {
		return nil, TagsResult{}, fmt.Errorf("failed to format tags: %w", err)
	}
	return buildMCPResult(toonOutput), TagsResult{Tags: tags}, nil
}

// SetPlanTags handles the set_plan_tags tool. Delegates the set+refetch to the atomic
// service-layer SetPlanTagsAndGet — no business logic here.
func (h *Handler) SetPlanTags(ctx context.Context, req *mcp.CallToolRequest, args SetPlanTagsArgs) (*mcp.CallToolResult, TagsResult, error) {
	if args.FileName == "" {
		return nil, TagsResult{}, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, TagsResult{}, fmt.Errorf("syncSource is required")
	}

	tags, err := h.service.SetPlanTagsAndGet(ctx, args.FileName, args.SyncSource, args.Tags)
	if err != nil {
		return nil, TagsResult{}, fmt.Errorf("failed to set tags: %w", err)
	}
	toonOutput, err := FormatTags(tags)
	if err != nil {
		return nil, TagsResult{}, fmt.Errorf("failed to format tags: %w", err)
	}
	return buildMCPResult(toonOutput), TagsResult{Tags: tags}, nil
}

// DeleteTag handles the delete_tag tool. One call to the atomic RemovePlanTag service
// method — no multi-call composition here.
func (h *Handler) DeleteTag(ctx context.Context, req *mcp.CallToolRequest, args DeletePlanTagArgs) (*mcp.CallToolResult, TagsResult, error) {
	if args.FileName == "" {
		return nil, TagsResult{}, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, TagsResult{}, fmt.Errorf("syncSource is required")
	}
	if args.Tag == "" {
		return nil, TagsResult{}, fmt.Errorf("tag is required")
	}

	tags, err := h.service.RemovePlanTag(ctx, args.FileName, args.SyncSource, args.Tag)
	if err != nil {
		return nil, TagsResult{}, fmt.Errorf("failed to remove tag: %w", err)
	}
	toonOutput, err := FormatTags(tags)
	if err != nil {
		return nil, TagsResult{}, fmt.Errorf("failed to format tags: %w", err)
	}
	return buildMCPResult(toonOutput), TagsResult{Tags: tags}, nil
}
