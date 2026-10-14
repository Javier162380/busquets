package busquets

import (
	"fmt"
	"strings"
	"time"
)

// memoryTimelineMarker separates the document header from its timeline sections.
// The renderer always writes it, and an incremental save splits on it to keep
// the sections already written while replacing the header.
const memoryTimelineMarker = "## Timeline"

const memoryTimeFormat = "2006-01-02 15:04 UTC"

// renderMemoryDocument assembles the full memory markdown from a header built
// out of title and summary, and the timeline sections.
func renderMemoryDocument(planTitle, summary, sections string) string {
	var doc strings.Builder

	doc.WriteString("# Memory: ")
	doc.WriteString(planTitle)
	doc.WriteString("\n\n")

	if summary != "" {
		doc.WriteString(strings.TrimSpace(summary))
		doc.WriteString("\n\n")
	}

	doc.WriteString(memoryTimelineMarker)
	doc.WriteString("\n\n")
	doc.WriteString(strings.TrimLeft(sections, "\n"))

	return strings.TrimRight(doc.String(), "\n") + "\n"
}

// existingMemorySections returns the timeline sections of an already-rendered
// document. A document without the marker (hand-edited, or written by an older
// version) yields "", so the caller starts a fresh timeline rather than
// scattering sections into prose.
func existingMemorySections(content string) string {
	_, after, found := strings.Cut(content, memoryTimelineMarker)
	if !found {
		return ""
	}
	return strings.Trim(after, "\n")
}

// renderMemorySections renders one "### heading" block per event.
func renderMemorySections(events []MemoryEvent) string {
	var sections strings.Builder

	for _, event := range events {
		sections.WriteString("### ")
		sections.WriteString(memoryEventHeading(event))
		sections.WriteString("\n\n")

		if event.Kind == MemoryEventComment && event.Body != "" {
			for _, line := range strings.Split(strings.TrimSpace(event.Body), "\n") {
				sections.WriteString("> ")
				sections.WriteString(line)
				sections.WriteString("\n")
			}
			sections.WriteString("\n")
		}

		narrative := strings.TrimSpace(event.Narrative)
		if narrative == "" {
			narrative = "_Not yet narrated._"
		}
		sections.WriteString(narrative)
		sections.WriteString("\n\n")
	}

	return sections.String()
}

// memoryEventHeading builds an event's heading from computed facts only.
func memoryEventHeading(event MemoryEvent) string {
	when := event.OccurredAt.UTC().Format(memoryTimeFormat)

	if event.Kind == MemoryEventComment {
		return fmt.Sprintf("Comment — %s", when)
	}

	label := "v?"
	if event.VersionNumber != nil {
		label = fmt.Sprintf("v%d", *event.VersionNumber)
	}

	switch {
	case event.Kind == MemoryEventRestore && event.RestoredFrom != nil:
		return fmt.Sprintf("%s — %s · restored from v%d", label, when, *event.RestoredFrom)
	case event.LinesAdded == 0 && event.LinesRemoved == 0:
		return fmt.Sprintf("%s — %s · initial · %d words", label, when, event.WordCount)
	default:
		return fmt.Sprintf("%s — %s · +%d −%d", label, when, event.LinesAdded, event.LinesRemoved)
	}
}

// memoryEventUserPrompt builds the per-event request sent to whoever writes the
// narrative. diff is the unified diff for version events and is empty otherwise.
func memoryEventUserPrompt(planTitle string, event MemoryEvent, diff, versionContent string) string {
	var prompt strings.Builder
	fmt.Fprintf(&prompt, "Plan: %s\n\n", planTitle)

	switch {
	case event.Kind == MemoryEventComment:
		prompt.WriteString("A comment was added to the plan. Describe what it says.\n\n")
		prompt.WriteString(event.Body)
	case event.Kind == MemoryEventRestore:
		prompt.WriteString("The plan was reverted to an earlier state. Describe what that undid.\n\n")
		prompt.WriteString(diff)
	case diff == "":
		prompt.WriteString("This is the first snapshot of the plan. Describe what it sets out to do.\n\n")
		prompt.WriteString(versionContent)
	default:
		prompt.WriteString("The plan changed. Describe what changed.\n\n")
		prompt.WriteString(diff)
	}

	return prompt.String()
}

// memoryVersionLabel names a version for diff headers.
func memoryVersionLabel(versionNumber int64, at time.Time) string {
	return fmt.Sprintf("v%d (%s)", versionNumber, at.UTC().Format(memoryTimeFormat))
}
