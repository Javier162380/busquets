package mcp

import (
	"context"
	"fmt"

	"github.com/Javier162380/busquets/services/busquets"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerMemoryTools registers all memory-related MCP tools.
func (h *Handler) registerMemoryTools() error {
	mcp.AddTool(h.server, &mcp.Tool{
		Name: "get_plan_memory",
		Description: "Returns a plan's memory — the narrative record of how it evolved — " +
			"together with its timeline of versions and comments. The timeline is " +
			"computed fresh on every call, so it is present even before a memory " +
			"has been written.",
	}, h.GetPlanMemory)

	mcp.AddTool(h.server, &mcp.Tool{
		Name: "generate_memory_prompt",
		Description: "Returns the prompts for writing a plan's memory: a system prompt " +
			"plus one user prompt per timeline event still to narrate (all events when " +
			"rebuild is true). Does not call any LLM or require an API key — write one " +
			"sentence per event yourself, then persist them with save_plan_memory. " +
			"Never write dates, version numbers or line counts into a narrative; " +
			"busquets supplies those.",
	}, h.GenerateMemoryPrompt)

	mcp.AddTool(h.server, &mcp.Tool{
		Name: "save_plan_memory",
		Description: "Persists narratives written from generate_memory_prompt. Each event " +
			"is matched to the computed timeline by refId and rejected if it does not " +
			"correspond to a real version or comment. Mode 'incremental' (default) " +
			"appends to the existing memory; 'rebuild' replaces it.",
	}, h.SavePlanMemory)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "delete_plan_memory",
		Description: "Deletes a plan's memory and its document. The plan and its versions are untouched.",
	}, h.DeletePlanMemory)

	return nil
}

// GetPlanMemory handles the get_plan_memory tool.
func (h *Handler) GetPlanMemory(ctx context.Context, req *mcp.CallToolRequest, args GetPlanMemoryArgs) (*mcp.CallToolResult, PlanMemoryResult, error) {
	if args.FileName == "" {
		return nil, PlanMemoryResult{}, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, PlanMemoryResult{}, fmt.Errorf("syncSource is required")
	}

	memory, err := h.service.GetPlanMemory(ctx, args.FileName, args.SyncSource)
	if err != nil {
		return nil, PlanMemoryResult{}, fmt.Errorf("failed to get plan memory: %w", err)
	}

	staleness, err := h.service.MemoryStaleness(ctx, args.FileName, args.SyncSource)
	if err != nil {
		return nil, PlanMemoryResult{}, fmt.Errorf("failed to check memory staleness: %w", err)
	}

	toonOutput, err := FormatPlanMemory(memory, staleness)
	if err != nil {
		return nil, PlanMemoryResult{}, fmt.Errorf("failed to format memory: %w", err)
	}

	return buildMCPResult(toonOutput), planMemoryResult(memory, staleness), nil
}

// GenerateMemoryPrompt handles the generate_memory_prompt tool.
func (h *Handler) GenerateMemoryPrompt(ctx context.Context, req *mcp.CallToolRequest, args GenerateMemoryPromptArgs) (*mcp.CallToolResult, *MemoryPromptResult, error) {
	if args.FileName == "" {
		return nil, nil, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, nil, fmt.Errorf("syncSource is required")
	}

	mode := busquets.MemoryModeIncremental
	if args.Rebuild {
		mode = busquets.MemoryModeRebuild
	}

	prompts, err := h.service.BuildMemoryPrompts(ctx, args.FileName, args.SyncSource, mode)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build memory prompts: %w", err)
	}

	toonOutput, err := FormatMemoryPrompts(prompts)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to format memory prompts: %w", err)
	}

	return buildMCPResult(toonOutput), memoryPromptResult(prompts), nil
}

// SavePlanMemory handles the save_plan_memory tool.
func (h *Handler) SavePlanMemory(ctx context.Context, req *mcp.CallToolRequest, args SavePlanMemoryArgs) (*mcp.CallToolResult, PlanMemoryResult, error) {
	if args.FileName == "" {
		return nil, PlanMemoryResult{}, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, PlanMemoryResult{}, fmt.Errorf("syncSource is required")
	}
	if len(args.Events) == 0 {
		return nil, PlanMemoryResult{}, fmt.Errorf("events is required: supply at least one narrative")
	}

	mode := busquets.MemoryMode(args.Mode)
	switch mode {
	case "", busquets.MemoryModeIncremental, busquets.MemoryModeRebuild:
	default:
		return nil, PlanMemoryResult{}, fmt.Errorf("mode must be %q or %q, got %q",
			busquets.MemoryModeIncremental, busquets.MemoryModeRebuild, args.Mode)
	}

	events := make([]busquets.SavePlanMemoryEvent, len(args.Events))
	for i, event := range args.Events {
		kind := busquets.MemoryEventKind(event.EventKind)
		switch kind {
		case busquets.MemoryEventVersion, busquets.MemoryEventRestore, busquets.MemoryEventComment:
		default:
			return nil, PlanMemoryResult{}, fmt.Errorf(
				"events[%d].eventKind must be %q, %q or %q, got %q",
				i, busquets.MemoryEventVersion, busquets.MemoryEventRestore,
				busquets.MemoryEventComment, event.EventKind)
		}
		events[i] = busquets.SavePlanMemoryEvent{
			Kind:      kind,
			RefID:     event.RefID,
			Narrative: event.Narrative,
		}
	}

	memory, err := h.service.SavePlanMemory(ctx, busquets.SavePlanMemoryRequest{
		FileName:    args.FileName,
		SyncSource:  args.SyncSource,
		Mode:        mode,
		Summary:     args.Summary,
		GeneratedBy: "mcp",
		Events:      events,
	})
	if err != nil {
		return nil, PlanMemoryResult{}, fmt.Errorf("failed to save plan memory: %w", err)
	}

	staleness, err := h.service.MemoryStaleness(ctx, args.FileName, args.SyncSource)
	if err != nil {
		return nil, PlanMemoryResult{}, fmt.Errorf("failed to check memory staleness: %w", err)
	}

	toonOutput, err := FormatPlanMemory(memory, staleness)
	if err != nil {
		return nil, PlanMemoryResult{}, fmt.Errorf("failed to format memory: %w", err)
	}

	return buildMCPResult(toonOutput), planMemoryResult(memory, staleness), nil
}

// DeletePlanMemory handles the delete_plan_memory tool.
func (h *Handler) DeletePlanMemory(ctx context.Context, req *mcp.CallToolRequest, args DeletePlanMemoryArgs) (*mcp.CallToolResult, any, error) {
	if args.FileName == "" {
		return nil, nil, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, nil, fmt.Errorf("syncSource is required")
	}

	if err := h.service.DeletePlanMemory(ctx, args.FileName, args.SyncSource); err != nil {
		return nil, nil, fmt.Errorf("failed to delete plan memory: %w", err)
	}
	return buildMCPResult(fmt.Sprintf("deleted memory for %s", args.FileName)), nil, nil
}

// planMemoryResult maps a service memory onto the tool's structured output.
func planMemoryResult(memory *busquets.PlanMemory, staleness busquets.MemoryStaleness) PlanMemoryResult {
	events := make([]MemoryEventEntry, len(memory.Events))
	for i, event := range memory.Events {
		events[i] = MemoryEventEntry{
			EventKind:     string(event.Kind),
			RefID:         event.RefID,
			VersionNumber: event.VersionNumber,
			RestoredFrom:  event.RestoredFrom,
			OccurredAt:    event.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"),
			LinesAdded:    event.LinesAdded,
			LinesRemoved:  event.LinesRemoved,
			WordCount:     event.WordCount,
		}
	}

	return PlanMemoryResult{
		FileName:            memory.FileName,
		SyncSource:          memory.SyncSource,
		PlanTitle:           memory.PlanTitle,
		Exists:              memory.Exists(),
		Summary:             memory.Summary,
		Content:             memory.Content,
		CoversUpToVersion:   memory.CoversUpToVersion,
		CoversUpToCommentID: memory.CoversUpToCommentID,
		GeneratedBy:         memory.GeneratedBy,
		NewVersions:         staleness.NewVersions,
		NewComments:         staleness.NewComments,
		Events:              events,
	}
}

// memoryPromptResult maps a service prompt set onto the tool's structured output.
func memoryPromptResult(prompts *busquets.MemoryPromptSet) *MemoryPromptResult {
	events := make([]MemoryEventPrompt, len(prompts.Events))
	for i, prompt := range prompts.Events {
		events[i] = MemoryEventPrompt{
			EventKind:     string(prompt.Event.Kind),
			RefID:         prompt.Event.RefID,
			VersionNumber: prompt.Event.VersionNumber,
			OccurredAt:    prompt.Event.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"),
			LinesAdded:    prompt.Event.LinesAdded,
			LinesRemoved:  prompt.Event.LinesRemoved,
			UserPrompt:    prompt.UserPrompt,
		}
	}

	return &MemoryPromptResult{
		FileName:            prompts.FileName,
		SyncSource:          prompts.SyncSource,
		PlanTitle:           prompts.PlanTitle,
		Mode:                string(prompts.Mode),
		SummarySystemPrompt: prompts.SummarySystemPrompt,
		SummaryUserPrompt:   prompts.SummaryUserPrompt,
		EventSystemPrompt:   prompts.EventSystemPrompt,
		Events:              events,
	}
}
