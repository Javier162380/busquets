// Package mcp contains all the MCP handling logic.
package mcp

import (
	"context"
	"fmt"

	"github.com/Javier162380/busquets/internal/connectors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTLDRTools registers the TLDR-related MCP tool.
func (h *Handler) registerTLDRTools() error {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "generate_tldr_prompt",
		Description: "Fetches a plan and returns a systemPrompt (the same " +
			"Goal/Approach/Outcome template used by the ollama summarizer connector) " +
			"plus a userPrompt (title + content, ready to summarize). Does not call " +
			"any LLM or require an API key — write the three-bullet TLDR yourself " +
			"following systemPrompt, then optionally persist it with add_comment.",
	}, h.GenerateTLDRPrompt)

	return nil
}

// GenerateTLDRPrompt handles the generate_tldr_prompt tool.
func (h *Handler) GenerateTLDRPrompt(ctx context.Context, req *mcp.CallToolRequest, args GenerateTLDRPromptArgs) (*mcp.CallToolResult, *TLDRPrompt, error) {
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

	prompt := &TLDRPrompt{
		FileName:     plan.FileName,
		SyncSource:   plan.SyncSource,
		SystemPrompt: connectors.SummarySystemPrompt,
		// Same shape as ollama.Connector.Send's Prompt field, minus its
		// truncateAtSentence cap — that 12k-char limit exists for Ollama's small
		// local-model context window, which doesn't apply to the calling assistant.
		UserPrompt: fmt.Sprintf("Summarize this plan:\n\nTitle: %s\n\n%s", plan.Title, plan.Content),
	}

	toonOutput, err := FormatTLDRPrompt(prompt)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to format tldr prompt: %w", err)
	}
	return buildMCPResult(toonOutput), prompt, nil
}
