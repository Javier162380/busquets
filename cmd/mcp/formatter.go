package mcp

import (
	"fmt"
	"strings"

	"github.com/Javier162380/busquets/services/busquets"

	"github.com/toon-format/toon-go"
)

// planSummaryTOON represents a plan summary for TOON encoding.
type planSummaryTOON struct {
	FileName    string `toon:"file_name"`
	SyncLabel   string `toon:"sync_label"`
	Title       string `toon:"title"`
	Tags        string `toon:"tags"`
	ModifiedAt  string `toon:"modified_at"`
	ReadingTime string `toon:"reading_time"`
}

// planDetailTOON represents plan metadata for TOON encoding.
type planDetailTOON struct {
	FileName    string `toon:"file_name"`
	SyncLabel   string `toon:"sync_label"`
	Title       string `toon:"title"`
	CreatedAt   string `toon:"created_at"`
	ModifiedAt  string `toon:"modified_at"`
	Tags        string `toon:"tags"`
	ReadingTime string `toon:"reading_time"`
	FileSize    int64  `toon:"file_size"`
}

// toolInfo represents a tool for TOON encoding.
type toolInfo struct {
	Name        string `toon:"name"`
	Description string `toon:"description"`
	Parameters  string `toon:"parameters"`
}

// planSearchResponse wraps the search results.
type planSearchResponse struct {
	Plans []planSummaryTOON `toon:"plans"`
}

// planResponse wraps a single plan.
type planResponse struct {
	Plan planDetailTOON `toon:"plan"`
}

// toolsResponse wraps the tools list.
type toolsResponse struct {
	Tools []toolInfo `toon:"tools"`
}

// FormatSearchResults formats plan summaries using TOON.
func FormatSearchResults(plans []busquets.PlanSummary) (string, error) {
	if len(plans) == 0 {
		return "plans[0]{file_name,title,tags,modified_at,reading_time}:", nil
	}

	// Convert to TOON-compatible format
	toonPlans := make([]planSummaryTOON, len(plans))
	for i, plan := range plans {
		toonPlans[i] = planSummaryTOON{
			FileName:    plan.FileName,
			SyncLabel:   plan.SyncLabel,
			Title:       plan.Title,
			Tags:        formatTagNames(plan.Tags),
			ModifiedAt:  plan.ModifiedAt.UTC().Format("2006-01-02T15:04:05Z"),
			ReadingTime: formatReadingTime(plan.ReadingTime),
		}
	}

	response := planSearchResponse{Plans: toonPlans}
	encoded, err := toon.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal TOON: %w", err)
	}

	return string(encoded), nil
}

// FormatPlanDetail formats a single plan using TOON + markdown content.
func FormatPlanDetail(plan *busquets.PlanDetail) (string, error) {
	// TOON metadata
	metadata := planResponse{
		Plan: planDetailTOON{
			FileName:    plan.FileName,
			SyncLabel:   plan.SyncLabel,
			Title:       plan.Title,
			CreatedAt:   plan.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
			ModifiedAt:  plan.ModifiedAt.UTC().Format("2006-01-02T15:04:05Z"),
			Tags:        formatTagNames(plan.Tags),
			ReadingTime: formatReadingTime(plan.ReadingTime),
			FileSize:    plan.FileSize,
		},
	}

	encoded, err := toon.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal TOON: %w", err)
	}

	// Append markdown content
	var sb strings.Builder
	sb.WriteString(string(encoded))
	sb.WriteString("\n\ncontent:\n")
	sb.WriteString(plan.Content)

	return sb.String(), nil
}

// FormatToolsList formats available tools using TOON.
func FormatToolsList() (string, error) {
	response := toolsResponse{
		Tools: []toolInfo{
			{
				Name:        "search_plans",
				Description: "Search plans by text and/or tags",
				Parameters:  "query(string)|tags(array)|matchAll(bool)|limit(int64,max:50)",
			},
			{
				Name:        "get_plan",
				Description: "Retrieve full plan content",
				Parameters:  "fileName(string,required)",
			},
			{
				Name:        "list_tools",
				Description: "List all available tools",
				Parameters:  "none",
			},
		},
	}

	encoded, err := toon.Marshal(response)
	if err != nil {
		return "", fmt.Errorf("failed to marshal TOON: %w", err)
	}

	return string(encoded), nil
}

// formatTagNames extracts tag names from a Tag slice and joins them.
func formatTagNames(tags []busquets.Tag) string {
	if len(tags) == 0 {
		return ""
	}
	names := make([]string, len(tags))
	for i, tag := range tags {
		names[i] = tag.Name
	}
	return strings.Join(names, ",")
}

// formatReadingTime formats reading time in minutes to a string.
func formatReadingTime(minutes int) string {
	if minutes == 1 {
		return "1 min"
	}
	return fmt.Sprintf("%d min", minutes)
}
