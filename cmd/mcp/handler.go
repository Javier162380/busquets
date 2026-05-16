// Package mcp contains all the MCP handling logic.
package mcp

import (
	"context"
	"fmt"

	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// planService defines the interface for plan operations needed by MCP handlers.
type planService interface {
	SearchPlansWithTags(ctx context.Context, query string, tags []string, matchAll bool) ([]claudeviewer.PlanSummary, error)
	GetPlanDetailByFileName(ctx context.Context, fileName string) (*claudeviewer.PlanDetail, error)
}

// Handler wraps the service and handles MCP protocol concerns.
type Handler struct {
	service planService
	server  *mcp.Server
}

// NewHandler creates a new handler instance.
func NewHandler(service claudeviewer.UnifiedService, server *mcp.Server) *Handler {
	return &Handler{
		service: service,
		server:  server,
	}
}

// Register registers ALL MCP concerns in one place.
func (h *Handler) Register() error {
	if err := h.registerPlanTools(); err != nil {
		return fmt.Errorf("failed to register plan tools: %w", err)
	}

	return nil
}

// buildMCPResult creates an MCP result from text content.
func buildMCPResult(textContent ...string) *mcp.CallToolResult {
	content := make([]mcp.Content, len(textContent))
	for i, text := range textContent {
		content[i] = &mcp.TextContent{Text: text}
	}
	return &mcp.CallToolResult{Content: content}
}
