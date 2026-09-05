// Package mcp contains all the MCP handling logic.
package mcp

import (
	"fmt"

	"github.com/Javier162380/busquets/services/busquets"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Handler wraps the service and handles MCP protocol concerns.
type Handler struct {
	service busquets.UnifiedService
	server  *mcp.Server
}

// NewHandler creates a new handler instance.
func NewHandler(service busquets.UnifiedService, server *mcp.Server) *Handler {
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
	if err := h.registerCommentTools(); err != nil {
		return fmt.Errorf("failed to register comment tools: %w", err)
	}
	if err := h.registerTagTools(); err != nil {
		return fmt.Errorf("failed to register tag tools: %w", err)
	}
	if err := h.registerVersionTools(); err != nil {
		return fmt.Errorf("failed to register version tools: %w", err)
	}
	if err := h.registerSyncTools(); err != nil {
		return fmt.Errorf("failed to register sync tools: %w", err)
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
