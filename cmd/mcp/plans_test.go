package mcp

import (
	"strings"
	"testing"
	"time"

	"github.com/Javier162380/busquets/services/busquets"
)

func TestFormatSearchResults(t *testing.T) {
	plans := []busquets.PlanSummary{
		{
			FileName:    "test-plan.md",
			Title:       "Test Plan",
			Tags:        []busquets.Tag{{Name: "tag1"}, {Name: "tag2"}},
			ModifiedAt:  time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			ReadingTime: 5,
		},
	}

	output, err := FormatSearchResults(plans)
	if err != nil {
		t.Fatalf("FormatSearchResults failed: %v", err)
	}

	// Verify TOON structure contains expected elements
	if !strings.Contains(output, "plans") {
		t.Errorf("Missing 'plans' in output: %s", output)
	}

	if !strings.Contains(output, "test-plan.md") {
		t.Errorf("Missing file name in output: %s", output)
	}

	if !strings.Contains(output, "Test Plan") {
		t.Errorf("Missing title in output: %s", output)
	}
}

func TestFormatSearchResults_Empty(t *testing.T) {
	plans := []busquets.PlanSummary{}

	output, err := FormatSearchResults(plans)
	if err != nil {
		t.Fatalf("FormatSearchResults failed: %v", err)
	}

	expected := "plans[0]{file_name,title,tags,modified_at,reading_time}:"
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestFormatPlanDetail(t *testing.T) {
	plan := &busquets.PlanDetail{
		PlanSummary: busquets.PlanSummary{
			FileName:    "test.md",
			Title:       "Test",
			CreatedAt:   time.Date(2024, 1, 10, 10, 0, 0, 0, time.UTC),
			ModifiedAt:  time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			Tags:        []busquets.Tag{{Name: "tag1"}},
			ReadingTime: 3,
			FileSize:    1024,
		},
		Content: "# Test Content\n\nThis is a test.",
	}

	output, err := FormatPlanDetail(plan)
	if err != nil {
		t.Fatalf("FormatPlanDetail failed: %v", err)
	}

	// Verify TOON metadata section
	if !strings.Contains(output, "plan") {
		t.Errorf("Missing TOON plan header: %s", output)
	}

	if !strings.Contains(output, "test.md") {
		t.Errorf("Missing filename in output: %s", output)
	}

	// Verify content section
	if !strings.Contains(output, "content:") {
		t.Errorf("Missing content section: %s", output)
	}

	if !strings.Contains(output, "# Test Content") {
		t.Errorf("Missing markdown content: %s", output)
	}
}

func TestFormatToolsList(t *testing.T) {
	output, err := FormatToolsList()
	if err != nil {
		t.Fatalf("FormatToolsList failed: %v", err)
	}

	// Verify all tools are listed
	if !strings.Contains(output, "search_plans") {
		t.Errorf("Missing search_plans tool: %s", output)
	}

	if !strings.Contains(output, "get_plan") {
		t.Errorf("Missing get_plan tool: %s", output)
	}

	if !strings.Contains(output, "list_tools") {
		t.Errorf("Missing list_tools tool: %s", output)
	}

	if !strings.Contains(output, "tools") {
		t.Errorf("Missing tools header: %s", output)
	}
}
