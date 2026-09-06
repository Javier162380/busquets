package mcp

import (
	"context"
	"fmt"

	"github.com/Javier162380/busquets/services/busquets"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerVersionTools registers all version-related MCP tools.
func (h *Handler) registerVersionTools() error {
	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "get_plan_version_history",
		Description: "Lists version history for a plan, most recent first. Limit default=20, max=50.",
	}, h.GetPlanVersionHistory)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "get_plan_version",
		Description: "Retrieves a specific version of a plan by version number, including its content.",
	}, h.GetPlanVersion)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "restore_plan_version",
		Description: "Restores a plan's content to a previous version (the restore itself is recorded as a new version).",
	}, h.RestorePlanVersion)

	mcp.AddTool(h.server, &mcp.Tool{
		Name:        "diff_plan_versions",
		Description: "Returns a unified (git-diff-style) diff of fromVersion's content against toVersion's, plus both versions' metadata.",
	}, h.DiffPlanVersions)

	return nil
}

// GetPlanVersionHistory handles the get_plan_version_history tool. Returns
// PlanVersionHistoryResult (not a bare slice) so it has a real output schema.
func (h *Handler) GetPlanVersionHistory(ctx context.Context, req *mcp.CallToolRequest, args GetPlanVersionHistoryArgs) (*mcp.CallToolResult, PlanVersionHistoryResult, error) {
	if args.FileName == "" {
		return nil, PlanVersionHistoryResult{}, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, PlanVersionHistoryResult{}, fmt.Errorf("syncSource is required")
	}

	limit := args.Limit
	if limit <= 0 {
		limit = 20
	} else if limit > 50 {
		limit = 50
	}

	versions, err := h.service.GetPlanVersionHistory(ctx, args.FileName, args.SyncSource, args.Offset, limit)
	if err != nil {
		return nil, PlanVersionHistoryResult{}, fmt.Errorf("failed to get version history: %w", err)
	}
	toonOutput, err := FormatPlanVersionHistory(versions)
	if err != nil {
		return nil, PlanVersionHistoryResult{}, fmt.Errorf("failed to format version history: %w", err)
	}
	return buildMCPResult(toonOutput), PlanVersionHistoryResult{Versions: versions}, nil
}

// GetPlanVersion handles the get_plan_version tool.
func (h *Handler) GetPlanVersion(ctx context.Context, req *mcp.CallToolRequest, args GetPlanVersionArgs) (*mcp.CallToolResult, *busquets.PlanVersionDetail, error) {
	if args.FileName == "" {
		return nil, nil, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, nil, fmt.Errorf("syncSource is required")
	}
	if args.VersionNumber <= 0 {
		return nil, nil, fmt.Errorf("versionNumber is required")
	}

	version, err := h.service.GetPlanVersion(ctx, args.FileName, args.SyncSource, args.VersionNumber)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get version: %w", err)
	}
	toonOutput, err := FormatPlanVersion(version)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to format version: %w", err)
	}
	return buildMCPResult(toonOutput), version, nil
}

// RestorePlanVersion handles the restore_plan_version tool. One call to the atomic
// RestorePlanVersionByFileName service method — the plan-lookup composition lives in the
// service layer, not here.
func (h *Handler) RestorePlanVersion(ctx context.Context, req *mcp.CallToolRequest, args RestorePlanVersionArgs) (*mcp.CallToolResult, any, error) {
	if args.FileName == "" {
		return nil, nil, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, nil, fmt.Errorf("syncSource is required")
	}
	if args.VersionNumber <= 0 {
		return nil, nil, fmt.Errorf("versionNumber is required")
	}

	if err := h.service.RestorePlanVersionByFileName(ctx, args.FileName, args.SyncSource, args.VersionNumber); err != nil {
		return nil, nil, fmt.Errorf("failed to restore version: %w", err)
	}

	return buildMCPResult(fmt.Sprintf("restored %s to version %d", args.FileName, args.VersionNumber)), nil, nil
}

// DiffPlanVersions handles the diff_plan_versions tool.
func (h *Handler) DiffPlanVersions(ctx context.Context, req *mcp.CallToolRequest, args DiffPlanVersionsArgs) (*mcp.CallToolResult, DiffResult, error) {
	if args.FileName == "" {
		return nil, DiffResult{}, fmt.Errorf("fileName is required")
	}
	if args.SyncSource == "" {
		return nil, DiffResult{}, fmt.Errorf("syncSource is required")
	}
	if args.FromVersion < 0 {
		return nil, DiffResult{}, fmt.Errorf("fromVersion is required")
	}
	if args.ToVersion < 0 {
		return nil, DiffResult{}, fmt.Errorf("toVersion is required")
	}

	vd, err := h.service.DiffPlanVersions(ctx, args.FileName, args.SyncSource, args.FromVersion, args.ToVersion)
	if err != nil {
		return nil, DiffResult{}, fmt.Errorf("failed to diff versions: %w", err)
	}

	toonOutput, err := FormatVersionDiff(vd)
	if err != nil {
		return nil, DiffResult{}, fmt.Errorf("failed to format diff: %w", err)
	}

	return buildMCPResult(toonOutput), DiffResult{
		Diff:          vd.Diff,
		FromVersion:   vd.From.VersionNumber,
		FromCreatedAt: vd.From.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		ToVersion:     vd.To.VersionNumber,
		ToCreatedAt:   vd.To.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}, nil
}
