package mcp

import (
	"fmt"
	"strings"
	"time"

	"github.com/Javier162380/busquets/services/busquets"

	"github.com/toon-format/toon-go"
)

// planSummaryTOON represents a plan summary for TOON encoding.
type planSummaryTOON struct {
	FileName    string `toon:"file_name"`
	SyncSource  string `toon:"sync_source"`
	SyncLabel   string `toon:"sync_label"`
	Title       string `toon:"title"`
	Tags        string `toon:"tags"`
	ModifiedAt  string `toon:"modified_at"`
	ReadingTime string `toon:"reading_time"`
}

// planDetailTOON represents plan metadata for TOON encoding.
type planDetailTOON struct {
	FileName    string `toon:"file_name"`
	SyncSource  string `toon:"sync_source"`
	SyncLabel   string `toon:"sync_label"`
	Title       string `toon:"title"`
	CreatedAt   string `toon:"created_at"`
	ModifiedAt  string `toon:"modified_at"`
	Tags        string `toon:"tags"`
	ReadingTime string `toon:"reading_time"`
	FileSize    int64  `toon:"file_size"`
}

// planSearchResponse wraps the search results.
type planSearchResponse struct {
	Plans []planSummaryTOON `toon:"plans"`
}

// planResponse wraps a single plan.
type planResponse struct {
	Plan planDetailTOON `toon:"plan"`
}

// commentTOON represents a comment for TOON encoding. Shared by the single-comment
// (add_comment) and list (get_plan_comments) responses.
type commentTOON struct {
	ID        int64     `toon:"id"`
	PlanID    int64     `toon:"plan_id"`
	Comment   string    `toon:"comment"`
	CreatedAt time.Time `toon:"created_at"`
	UpdatedAt time.Time `toon:"updated_at"`
}

type commentResponse struct {
	Comment commentTOON `toon:"comment"`
}

type commentsResponse struct {
	Comments []commentTOON `toon:"comments"`
}

// tagTOON represents a tag for TOON encoding.
type tagTOON struct {
	ID          int64  `toon:"id"`
	Name        string `toon:"name"`
	Description string `toon:"description"`
	Color       string `toon:"color"`
}

type tagsResponse struct {
	Tags []tagTOON `toon:"tags"`
}

// planVersionTOON represents a plan version for TOON encoding.
type planVersionTOON struct {
	ID            int64  `toon:"id"`
	VersionNumber int64  `toon:"version_number"`
	WordCount     int64  `toon:"word_count"`
	CreatedAt     string `toon:"created_at"`
	ReadingTime   string `toon:"reading_time"`
	Tags          string `toon:"tags"`
}

type planVersionHistoryResponse struct {
	Versions []planVersionTOON `toon:"versions"`
}

type planVersionResponse struct {
	Version planVersionTOON `toon:"version"`
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
			SyncSource:  plan.SyncSource,
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
			SyncSource:  plan.SyncSource,
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

// FormatAddPlanComment formats a single comment using TOON + its content.
func FormatAddPlanComment(planComment busquets.Comment) (string, error) {
	wrapped := commentResponse{Comment: commentTOON{
		ID:        planComment.ID,
		PlanID:    planComment.PlanID,
		Comment:   planComment.Content,
		CreatedAt: planComment.CreatedAt,
		UpdatedAt: planComment.UpdatedAt,
	}}

	encoded, err := toon.Marshal(wrapped)
	if err != nil {
		return "", fmt.Errorf("failed to marshal TOON: %w", err)
	}

	// Append markdown content
	var sb strings.Builder
	sb.WriteString(string(encoded))
	sb.WriteString("\n\ncomment:\n")
	sb.WriteString(planComment.Content)

	return sb.String(), nil
}

// FormatPlanComments formats a list of comments using TOON.
func FormatPlanComments(comments []busquets.Comment) (string, error) {
	toonComments := make([]commentTOON, len(comments))
	for i, c := range comments {
		toonComments[i] = commentTOON{
			ID:        c.ID,
			PlanID:    c.PlanID,
			Comment:   c.Content,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		}
	}

	encoded, err := toon.Marshal(commentsResponse{Comments: toonComments})
	if err != nil {
		return "", fmt.Errorf("failed to marshal TOON: %w", err)
	}

	return string(encoded), nil
}

// FormatTags formats a list of tags using TOON.
func FormatTags(tags []busquets.Tag) (string, error) {
	toonTags := make([]tagTOON, len(tags))
	for i, t := range tags {
		toonTags[i] = tagTOON{
			ID:          t.ID,
			Name:        t.Name,
			Description: derefOrEmpty(t.Description),
			Color:       derefOrEmpty(t.Color),
		}
	}

	encoded, err := toon.Marshal(tagsResponse{Tags: toonTags})
	if err != nil {
		return "", fmt.Errorf("failed to marshal TOON: %w", err)
	}

	return string(encoded), nil
}

// FormatPlanVersionHistory formats version history (metadata only, no content) using TOON.
func FormatPlanVersionHistory(versions []busquets.PlanVersionDetail) (string, error) {
	toonVersions := make([]planVersionTOON, len(versions))
	for i, v := range versions {
		toonVersions[i] = toPlanVersionTOON(v)
	}

	encoded, err := toon.Marshal(planVersionHistoryResponse{Versions: toonVersions})
	if err != nil {
		return "", fmt.Errorf("failed to marshal TOON: %w", err)
	}

	return string(encoded), nil
}

// FormatPlanVersion formats a single version's metadata + content using TOON, mirroring
// FormatPlanDetail's shape.
func FormatPlanVersion(v *busquets.PlanVersionDetail) (string, error) {
	encoded, err := toon.Marshal(planVersionResponse{Version: toPlanVersionTOON(*v)})
	if err != nil {
		return "", fmt.Errorf("failed to marshal TOON: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(string(encoded))
	sb.WriteString("\n\ncontent:\n")
	sb.WriteString(v.Content)

	return sb.String(), nil
}

func toPlanVersionTOON(v busquets.PlanVersionDetail) planVersionTOON {
	return planVersionTOON{
		ID:            v.ID,
		VersionNumber: v.VersionNumber,
		WordCount:     v.WordCount,
		CreatedAt:     v.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		ReadingTime:   formatReadingTime(v.ReadingTime),
		Tags:          formatTagNames(v.Tags),
	}
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

// derefOrEmpty dereferences a string pointer, or returns "" if nil.
func derefOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// versionDiffTOON represents version-diff metadata for TOON encoding.
type versionDiffTOON struct {
	FromVersion   int64  `toon:"from_version"`
	FromCreatedAt string `toon:"from_created_at"`
	ToVersion     int64  `toon:"to_version"`
	ToCreatedAt   string `toon:"to_created_at"`
}

type versionDiffResponse struct {
	Diff versionDiffTOON `toon:"diff"`
}

// tldrPromptTOON represents TLDR prompt metadata for TOON encoding. SystemPrompt is
// short and constant so it fits fine as a scalar row; UserPrompt carries the whole
// plan body and is appended as raw text below instead (same convention as
// FormatPlanDetail, FormatAddPlanComment, FormatPlanVersion, and FormatVersionDiff:
// bulky content stays out of the tabular TOON block).
type tldrPromptTOON struct {
	FileName     string `toon:"file_name"`
	SyncSource   string `toon:"sync_source"`
	SystemPrompt string `toon:"system_prompt"`
}

type tldrPromptResponse struct {
	Tldr tldrPromptTOON `toon:"tldr"`
}

// FormatTLDRPrompt formats a TLDR prompt bundle (metadata + system prompt via TOON,
// user prompt appended as raw text) using TOON.
func FormatTLDRPrompt(p *TLDRPrompt) (string, error) {
	metadata := tldrPromptResponse{Tldr: tldrPromptTOON{
		FileName:     p.FileName,
		SyncSource:   p.SyncSource,
		SystemPrompt: p.SystemPrompt,
	}}

	encoded, err := toon.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal TOON: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(string(encoded))
	sb.WriteString("\n\nuser_prompt:\n")
	sb.WriteString(p.UserPrompt)

	return sb.String(), nil
}

// FormatVersionDiff formats diff metadata using TOON + the raw diff text.
func FormatVersionDiff(vd busquets.VersionDiff) (string, error) {
	metadata := versionDiffResponse{Diff: versionDiffTOON{
		FromVersion:   vd.From.VersionNumber,
		FromCreatedAt: vd.From.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
		ToVersion:     vd.To.VersionNumber,
		ToCreatedAt:   vd.To.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	}}
	encoded, err := toon.Marshal(metadata)
	if err != nil {
		return "", fmt.Errorf("failed to marshal TOON: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(string(encoded))
	sb.WriteString("\n\ndiff:\n")
	sb.WriteString(vd.Diff)

	return sb.String(), nil
}

// memoryEventTOON represents a timeline entry for TOON encoding.
type memoryEventTOON struct {
	EventKind     string `toon:"event_kind"`
	RefID         int64  `toon:"ref_id"`
	VersionNumber string `toon:"version_number"`
	OccurredAt    string `toon:"occurred_at"`
	Change        string `toon:"change"`
}

type memoryResponse struct {
	Memory memoryMetaTOON    `toon:"memory"`
	Events []memoryEventTOON `toon:"events"`
}

type memoryMetaTOON struct {
	FileName            string `toon:"file_name"`
	SyncSource          string `toon:"sync_source"`
	PlanTitle           string `toon:"plan_title"`
	Exists              bool   `toon:"exists"`
	CoversUpToVersion   int64  `toon:"covers_up_to_version"`
	CoversUpToCommentID int64  `toon:"covers_up_to_comment_id"`
	GeneratedBy         string `toon:"generated_by"`
	NewVersions         int    `toon:"new_versions"`
	NewComments         int    `toon:"new_comments"`
}

// memoryEventChange renders an event's computed change stats for display.
func memoryEventChange(event busquets.MemoryEvent) string {
	switch {
	case event.Kind == busquets.MemoryEventComment:
		return "comment"
	case event.Kind == busquets.MemoryEventRestore && event.RestoredFrom != nil:
		return fmt.Sprintf("restored from v%d", *event.RestoredFrom)
	case event.LinesAdded == 0 && event.LinesRemoved == 0:
		return fmt.Sprintf("initial (%d words)", event.WordCount)
	default:
		return fmt.Sprintf("+%d -%d", event.LinesAdded, event.LinesRemoved)
	}
}

func memoryEventsTOON(events []busquets.MemoryEvent) []memoryEventTOON {
	out := make([]memoryEventTOON, len(events))
	for i, event := range events {
		versionNumber := ""
		if event.VersionNumber != nil {
			versionNumber = fmt.Sprintf("%d", *event.VersionNumber)
		}
		out[i] = memoryEventTOON{
			EventKind:     string(event.Kind),
			RefID:         event.RefID,
			VersionNumber: versionNumber,
			OccurredAt:    event.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"),
			Change:        memoryEventChange(event),
		}
	}
	return out
}

// FormatPlanMemory formats a memory as TOON metadata plus its markdown document.
func FormatPlanMemory(memory *busquets.PlanMemory, staleness busquets.MemoryStaleness) (string, error) {
	encoded, err := toon.Marshal(memoryResponse{
		Memory: memoryMetaTOON{
			FileName:            memory.FileName,
			SyncSource:          memory.SyncSource,
			PlanTitle:           memory.PlanTitle,
			Exists:              memory.Exists(),
			CoversUpToVersion:   memory.CoversUpToVersion,
			CoversUpToCommentID: memory.CoversUpToCommentID,
			GeneratedBy:         memory.GeneratedBy,
			NewVersions:         staleness.NewVersions,
			NewComments:         staleness.NewComments,
		},
		Events: memoryEventsTOON(memory.Events),
	})
	if err != nil {
		return "", fmt.Errorf("failed to marshal TOON: %w", err)
	}

	var sb strings.Builder
	sb.WriteString(string(encoded))
	if memory.Exists() {
		sb.WriteString("\n\nmemory:\n")
		sb.WriteString(memory.Content)
	} else {
		sb.WriteString("\n\nmemory:\n(not generated yet — call generate_memory_prompt, then save_plan_memory)")
	}
	return sb.String(), nil
}

// FormatMemoryPrompts formats the prompt set. The prompts themselves are emitted
// verbatim rather than through TOON — they are instructions to follow, and the
// per-event ones carry diffs whose whitespace must survive intact.
func FormatMemoryPrompts(prompts *busquets.MemoryPromptSet) (string, error) {
	var sb strings.Builder

	fmt.Fprintf(&sb, "plan: %s\nfile_name: %s\nsync_source: %s\nmode: %s\nevents_to_narrate: %d\n",
		prompts.PlanTitle, prompts.FileName, prompts.SyncSource, prompts.Mode, len(prompts.Events))

	sb.WriteString("\nsummary_system_prompt:\n")
	sb.WriteString(prompts.SummarySystemPrompt)
	sb.WriteString("\n\nsummary_user_prompt:\n")
	sb.WriteString(prompts.SummaryUserPrompt)
	sb.WriteString("\n\nevent_system_prompt:\n")
	sb.WriteString(prompts.EventSystemPrompt)

	if len(prompts.Events) == 0 {
		sb.WriteString("\n\nevents: none — this memory is already up to date.\n")
		return sb.String(), nil
	}

	for _, prompt := range prompts.Events {
		versionNumber := ""
		if prompt.Event.VersionNumber != nil {
			versionNumber = fmt.Sprintf(" version_number=%d", *prompt.Event.VersionNumber)
		}
		fmt.Fprintf(&sb, "\n\n--- event event_kind=%s ref_id=%d%s occurred_at=%s ---\n",
			prompt.Event.Kind, prompt.Event.RefID, versionNumber,
			prompt.Event.OccurredAt.UTC().Format("2006-01-02T15:04:05Z"))
		sb.WriteString(prompt.UserPrompt)
	}
	sb.WriteString("\n")

	return sb.String(), nil
}
