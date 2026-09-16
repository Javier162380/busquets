package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/Javier162380/busquets/services/busquets"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// resultText returns a tool result's single text block.
func resultText(t *testing.T, result *mcp.CallToolResult) string {
	t.Helper()
	require.Len(t, result.Content, 1)
	text, ok := result.Content[0].(*mcp.TextContent)
	require.True(t, ok, "expected a text content block")
	return text.Text
}

const memoryTestPlan = "# Memory Plan\n\nThe original shape of the plan.\n"

// setupMemoryHandler creates a handler with one synced plan and returns it
// alongside the plan's name and source directory.
func setupMemoryHandler(t *testing.T) (*Handler, string, string, func()) {
	t.Helper()

	service, sourcePlansDir, cleanup := setupTestService(t)
	createTestPlanFile(t, sourcePlansDir, "memory-plan.md", memoryTestPlan)

	_, err := service.SyncPlans(context.Background())
	require.NoError(t, err)

	return &Handler{service: service}, "memory-plan.md", sourcePlansDir, cleanup
}

// narrate saves a narrative for every event the prompt tool hands back.
func narrate(t *testing.T, handler *Handler, name, sourceDir, summary string, rebuild bool) PlanMemoryResult {
	t.Helper()
	ctx := context.Background()

	_, prompts, err := handler.GenerateMemoryPrompt(ctx, nil, GenerateMemoryPromptArgs{
		FileName: name, SyncSource: sourceDir, Rebuild: rebuild,
	})
	require.NoError(t, err)

	events := make([]SaveMemoryEventArg, len(prompts.Events))
	for i, prompt := range prompts.Events {
		events[i] = SaveMemoryEventArg{
			EventKind: prompt.EventKind,
			RefID:     prompt.RefID,
			Narrative: "A sentence about this change.",
		}
	}

	mode := "incremental"
	if rebuild {
		mode = "rebuild"
	}

	_, saved, err := handler.SavePlanMemory(ctx, nil, SavePlanMemoryArgs{
		FileName: name, SyncSource: sourceDir, Mode: mode, Summary: summary, Events: events,
	})
	require.NoError(t, err)
	return saved
}

func TestGetPlanMemoryHandler(t *testing.T) {
	ctx := context.Background()

	t.Run("returns the timeline before a memory exists", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		result, structured, err := handler.GetPlanMemory(ctx, nil, GetPlanMemoryArgs{
			FileName: name, SyncSource: sourceDir,
		})
		require.NoError(t, err)
		require.False(t, structured.Exists)
		require.Len(t, structured.Events, 1)
		require.Equal(t, 1, structured.NewVersions)
		require.Equal(t, busquets.NoMemoryCoverage, structured.CoversUpToVersion)

		text := resultText(t, result)
		require.Contains(t, text, "not generated yet")
	})

	t.Run("returns the document once written", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		narrate(t, handler, name, sourceDir, "A summary.", false)

		_, structured, err := handler.GetPlanMemory(ctx, nil, GetPlanMemoryArgs{
			FileName: name, SyncSource: sourceDir,
		})
		require.NoError(t, err)
		require.True(t, structured.Exists)
		require.Equal(t, "A summary.", structured.Summary)
		require.Equal(t, 0, structured.NewVersions)
		require.Contains(t, structured.Content, "# Memory: Memory Plan")
	})

	t.Run("requires fileName and syncSource", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		_, _, err := handler.GetPlanMemory(ctx, nil, GetPlanMemoryArgs{SyncSource: sourceDir})
		require.Error(t, err)

		_, _, err = handler.GetPlanMemory(ctx, nil, GetPlanMemoryArgs{FileName: name})
		require.Error(t, err)
	})

	t.Run("errors for an unknown plan", func(t *testing.T) {
		handler, _, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		_, _, err := handler.GetPlanMemory(ctx, nil, GetPlanMemoryArgs{
			FileName: "missing.md", SyncSource: sourceDir,
		})
		require.Error(t, err)
	})
}

func TestGenerateMemoryPromptHandler(t *testing.T) {
	ctx := context.Background()

	t.Run("returns both prompts and one event per un-narrated entry", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		result, structured, err := handler.GenerateMemoryPrompt(ctx, nil, GenerateMemoryPromptArgs{
			FileName: name, SyncSource: sourceDir,
		})
		require.NoError(t, err)
		require.Equal(t, "incremental", structured.Mode)
		require.NotEmpty(t, structured.SummarySystemPrompt)
		require.NotEmpty(t, structured.EventSystemPrompt)
		require.Len(t, structured.Events, 1)
		require.Equal(t, string(busquets.MemoryEventVersion), structured.Events[0].EventKind)

		text := resultText(t, result)
		require.Contains(t, text, "event_system_prompt:")
		require.Contains(t, text, "--- event event_kind=version")
	})

	t.Run("reports an up-to-date memory as having nothing to narrate", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		narrate(t, handler, name, sourceDir, "Done.", false)

		result, structured, err := handler.GenerateMemoryPrompt(ctx, nil, GenerateMemoryPromptArgs{
			FileName: name, SyncSource: sourceDir,
		})
		require.NoError(t, err)
		require.Empty(t, structured.Events)
		require.Contains(t, resultText(t, result), "already up to date")
	})

	t.Run("rebuild asks for every event again", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		narrate(t, handler, name, sourceDir, "Done.", false)

		_, structured, err := handler.GenerateMemoryPrompt(ctx, nil, GenerateMemoryPromptArgs{
			FileName: name, SyncSource: sourceDir, Rebuild: true,
		})
		require.NoError(t, err)
		require.Equal(t, "rebuild", structured.Mode)
		require.Len(t, structured.Events, 1)
	})

	t.Run("requires fileName and syncSource", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		_, _, err := handler.GenerateMemoryPrompt(ctx, nil, GenerateMemoryPromptArgs{SyncSource: sourceDir})
		require.Error(t, err)

		_, _, err = handler.GenerateMemoryPrompt(ctx, nil, GenerateMemoryPromptArgs{FileName: name})
		require.Error(t, err)
	})
}

func TestSavePlanMemoryHandler(t *testing.T) {
	ctx := context.Background()

	t.Run("persists narratives and advances the watermark", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		saved := narrate(t, handler, name, sourceDir, "It began.", false)
		require.True(t, saved.Exists)
		require.Equal(t, int64(0), saved.CoversUpToVersion)
		require.Equal(t, 0, saved.NewVersions)
		require.Contains(t, saved.Content, "A sentence about this change.")
	})

	t.Run("rejects an event that is not on the timeline", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		_, _, err := handler.SavePlanMemory(ctx, nil, SavePlanMemoryArgs{
			FileName: name, SyncSource: sourceDir,
			Events: []SaveMemoryEventArg{{EventKind: "version", RefID: 4242, Narrative: "invented"}},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "not in the computed timeline")
	})

	t.Run("rejects an unknown event kind", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		_, _, err := handler.SavePlanMemory(ctx, nil, SavePlanMemoryArgs{
			FileName: name, SyncSource: sourceDir,
			Events: []SaveMemoryEventArg{{EventKind: "wibble", RefID: 1, Narrative: "x"}},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "eventKind")
	})

	t.Run("rejects an unknown mode", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		_, _, err := handler.SavePlanMemory(ctx, nil, SavePlanMemoryArgs{
			FileName: name, SyncSource: sourceDir, Mode: "sideways",
			Events: []SaveMemoryEventArg{{EventKind: "version", RefID: 1, Narrative: "x"}},
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "mode must be")
	})

	t.Run("requires at least one event", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		_, _, err := handler.SavePlanMemory(ctx, nil, SavePlanMemoryArgs{
			FileName: name, SyncSource: sourceDir,
		})
		require.Error(t, err)
		require.Contains(t, err.Error(), "events is required")
	})

	t.Run("rebuild replaces the timeline rather than appending", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		narrate(t, handler, name, sourceDir, "First.", false)

		_, _, err := handler.AddPlanComment(ctx, nil, AddPlanCommentArgs{
			FileName: name, SyncSource: sourceDir, Comment: "a thought",
		})
		require.NoError(t, err)

		rebuilt := narrate(t, handler, name, sourceDir, "Rewritten.", true)
		require.Equal(t, 2, strings.Count(rebuilt.Content, "### "),
			"rebuild re-narrates every event exactly once")
		require.Equal(t, "Rewritten.", rebuilt.Summary)
		require.Equal(t, 0, rebuilt.NewVersions)
		require.Equal(t, 0, rebuilt.NewComments)
	})

	t.Run("a second pass appends rather than replacing", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		first := narrate(t, handler, name, sourceDir, "First.", false)
		require.Equal(t, 1, strings.Count(first.Content, "### "))

		_, _, err := handler.AddPlanComment(ctx, nil, AddPlanCommentArgs{
			FileName: name, SyncSource: sourceDir, Comment: "a thought",
		})
		require.NoError(t, err)

		second := narrate(t, handler, name, sourceDir, "", false)
		require.Equal(t, 2, strings.Count(second.Content, "### "))
		require.Equal(t, "First.", second.Summary)
	})
}

func TestDeletePlanMemoryHandler(t *testing.T) {
	ctx := context.Background()

	t.Run("deletes the memory but keeps the timeline", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		narrate(t, handler, name, sourceDir, "Gone soon.", false)

		_, _, err := handler.DeletePlanMemory(ctx, nil, DeletePlanMemoryArgs{
			FileName: name, SyncSource: sourceDir,
		})
		require.NoError(t, err)

		_, structured, err := handler.GetPlanMemory(ctx, nil, GetPlanMemoryArgs{
			FileName: name, SyncSource: sourceDir,
		})
		require.NoError(t, err)
		require.False(t, structured.Exists)
		require.Len(t, structured.Events, 1)
	})

	t.Run("errors when there is no memory", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		_, _, err := handler.DeletePlanMemory(ctx, nil, DeletePlanMemoryArgs{
			FileName: name, SyncSource: sourceDir,
		})
		require.Error(t, err)
	})

	t.Run("requires fileName and syncSource", func(t *testing.T) {
		handler, name, sourceDir, cleanup := setupMemoryHandler(t)
		defer cleanup()

		_, _, err := handler.DeletePlanMemory(ctx, nil, DeletePlanMemoryArgs{SyncSource: sourceDir})
		require.Error(t, err)

		_, _, err = handler.DeletePlanMemory(ctx, nil, DeletePlanMemoryArgs{FileName: name})
		require.Error(t, err)
	})
}
