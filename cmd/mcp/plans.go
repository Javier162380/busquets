package mcp

import (
	"context"
	"fmt"

	"github.com/Javier162380/busquets/services/busquets"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerPlanTools registers all plan-related MCP tools.
func (h *Handler) registerPlanTools() error {
	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "search_plans",
		Description: "Search LLM/AI-assistant plan files by text content and/or tags. Returns plan summaries with metadata in TOON format. Use empty query to list all plans. Tag filtering supports AND (matchAll=true) or OR (matchAll=false) logic. Limit parameter: default=20, max=50.",
	}, h.SearchPlansHandler)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "get_plan",
		Description: "Retrieve the full content and metadata of a specific plan file by filename. Returns metadata in TOON format followed by markdown content.",
	}, h.GetPlanHandler)

	return nil
}

// SearchPlansHandler handles the search_plans tool.
func (h *Handler) SearchPlansHandler(ctx context.Context, req *mcp.CallToolRequest, args SearchPlansArgs) (*mcp.CallToolResult, []busquets.PlanSummary, error) {
	// Validate and set defaults
	limit := args.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 50 {
		limit = 50
	}

	// Call service layer
	plans, err := h.service.SearchPlansWithTags(ctx, args.Query, args.Tags, args.MatchAll) // TODO: optimize this call please.
	if err != nil {
		return nil, nil, fmt.Errorf("failed to search plans: %w", err)
	}

	// Limit results
	if int64(len(plans)) > limit {
		plans = plans[:limit]
	}

	// Format response using TOON
	toonOutput, err := FormatSearchResults(plans)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to format results: %w", err)
	}

	return buildMCPResult(toonOutput), plans, nil
}

// GetPlanHandler handles the get_plan tool.
func (h *Handler) GetPlanHandler(ctx context.Context, req *mcp.CallToolRequest, args GetPlanArgs) (*mcp.CallToolResult, *busquets.PlanDetail, error) {
	if args.FileName == "" {
		return nil, nil, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, nil, fmt.Errorf("syncSource is required")
	}

	plan, err := h.service.GetPlanDetailByFileName(ctx, args.FileName, args.SyncSource)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get plan: %w", err)
	}

	// Format response using TOON
	toonOutput, err := FormatPlanDetail(plan)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to format plan: %w", err)
	}

	return buildMCPResult(toonOutput), plan, nil
}
