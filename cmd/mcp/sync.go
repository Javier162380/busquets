package mcp

import (
	"context"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// SyncResult wraps a sync count so it's a valid MCP output schema (a bare int isn't).
type SyncResult struct {
	Count int `json:"count"`
}

// registerSyncTools registers all sync-related MCP tools.
func (h *Handler) registerSyncTools() error {
	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "sync_plans",
		Description: "Forces an immediate sync of plans from configured source directories into the database.",
	}, h.SyncPlans)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "rsync_plans",
		Description: "Forces an immediate bidirectional sync, writing database changes back to source directories.",
	}, h.RSyncPlans)

	return nil
}

// SyncPlans handles the sync_plans tool.
func (h *Handler) SyncPlans(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, SyncResult, error) {
	count, err := h.service.SyncPlans(ctx)
	if err != nil {
		return nil, SyncResult{}, fmt.Errorf("failed to sync plans: %w", err)
	}
	return buildMCPResult(fmt.Sprintf("synced %d plans", count)), SyncResult{Count: count}, nil
}

// RSyncPlans handles the rsync_plans tool.
func (h *Handler) RSyncPlans(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, SyncResult, error) {
	count, err := h.service.RSyncPlans(ctx)
	if err != nil {
		return nil, SyncResult{}, fmt.Errorf("failed to rsync plans: %w", err)
	}
	return buildMCPResult(fmt.Sprintf("rsynced %d plans", count)), SyncResult{Count: count}, nil
}
