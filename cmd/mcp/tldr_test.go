package mcp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/Javier162380/busquets/internal/connectors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

func TestGenerateTLDRPrompt(t *testing.T) {
	ctx := context.Background()
	service, sourcePlansDir, cleanup := setupTestService(t)
	defer cleanup()

	testContent := "# Test Plan\n\nThis is a comprehensive test plan with multiple sections."
	createTestPlanFile(t, sourcePlansDir, "test-plan.md", testContent)

	_, err := service.SyncPlans(ctx)
	require.NoError(t, err)

	handler := &Handler{service: service, server: nil}

	t.Run("successful generate", func(t *testing.T) {
		args := GenerateTLDRPromptArgs{
			FileName:   "test-plan.md",
			SyncSource: sourcePlansDir,
		}

		result, prompt, err := handler.GenerateTLDRPrompt(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotNil(t, prompt)
		require.Equal(t, "test-plan.md", prompt.FileName)
		require.Equal(t, sourcePlansDir, prompt.SyncSource)
		require.Equal(t, connectors.SummarySystemPrompt, prompt.SystemPrompt)
		require.Equal(t, fmt.Sprintf("Summarize this plan:\n\nTitle: %s\n\n%s", "Test Plan", testContent), prompt.UserPrompt)
	})

	t.Run("empty filename", func(t *testing.T) {
		args := GenerateTLDRPromptArgs{SyncSource: sourcePlansDir}
		result, prompt, err := handler.GenerateTLDRPrompt(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, prompt)
		require.Equal(t, "fileName is required", err.Error())
	})

	t.Run("empty sync source", func(t *testing.T) {
		args := GenerateTLDRPromptArgs{FileName: "test-plan.md"}
		result, prompt, err := handler.GenerateTLDRPrompt(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, prompt)
		require.Equal(t, "syncSource is required", err.Error())
	})

	t.Run("plan not found", func(t *testing.T) {
		args := GenerateTLDRPromptArgs{
			FileName:   "nonexistent-plan.md",
			SyncSource: sourcePlansDir,
		}
		result, prompt, err := handler.GenerateTLDRPrompt(ctx, &mcp.CallToolRequest{}, args)
		require.Error(t, err)
		require.Nil(t, result)
		require.Nil(t, prompt)
		require.Equal(t, "failed to get plan: not found", err.Error())
	})

	t.Run("verify TOON format with user prompt", func(t *testing.T) {
		args := GenerateTLDRPromptArgs{
			FileName:   "test-plan.md",
			SyncSource: sourcePlansDir,
		}

		result, prompt, err := handler.GenerateTLDRPrompt(ctx, &mcp.CallToolRequest{}, args)
		require.NoError(t, err)
		require.NotNil(t, result)
		require.NotEmpty(t, result.Content)

		textContent, ok := result.Content[0].(*mcp.TextContent)
		require.True(t, ok, "result should be TextContent")

		firstLine, _, found := strings.Cut(textContent.Text, "\n")
		require.True(t, found)
		require.Equal(t, "tldr:", firstLine)

		_, userPrompt, found := strings.Cut(textContent.Text, "\n\nuser_prompt:\n")
		require.True(t, found)
		require.Equal(t, prompt.UserPrompt, userPrompt)
	})
}

func TestFormatTLDRPrompt(t *testing.T) {
	prompt := &TLDRPrompt{
		FileName:     "test.md",
		SyncSource:   "/some/source",
		SystemPrompt: connectors.SummarySystemPrompt,
		UserPrompt:   "Summarize this plan:\n\nTitle: Test\n\n# Test Content\n\nThis is a test.",
	}

	output, err := FormatTLDRPrompt(prompt)
	require.NoError(t, err)

	require.Contains(t, output, "tldr")
	require.Contains(t, output, "test.md")
	require.Contains(t, output, "system_prompt")
	require.Contains(t, output, "user_prompt:")
	require.Contains(t, output, "# Test Content")
}
