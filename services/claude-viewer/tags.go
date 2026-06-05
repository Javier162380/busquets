package claudeviewer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var validCharsRegex = regexp.MustCompile(`[^a-z0-9_-]+`)

const maxTagLength = 64

// ExtractTagsFromContent parses plan content for tags declared in YAML frontmatter.
func ExtractTagsFromContent(content string) []string {
	return NormalizeTags(parseFrontmatterTags(content))
}

// parseFrontmatterTags extracts the tags value from a YAML frontmatter block.
// The block must start at byte 0 and be delimited by "---" lines.
// Returns nil if the file has no frontmatter or no tags key.
//
// Supported formats:
//
//	tags: foo, bar
//	tags: [foo, bar]
//	tags:
//	  - foo
//	  - bar
func parseFrontmatterTags(content string) []string {
	if !strings.HasPrefix(content, "---\n") {
		return nil
	}
	rest := content[4:] // skip opening "---\n"
	endIdx := strings.Index(rest, "\n---")
	if endIdx == -1 {
		return nil
	}
	block := rest[:endIdx]

	lines := strings.Split(block, "\n")
	for i, line := range lines {
		key, val, found := strings.Cut(line, ":")
		if !found || !strings.EqualFold(strings.TrimSpace(key), "tags") {
			continue
		}
		value := strings.TrimSpace(val)

		// YAML list form — value is empty, items follow as "  - foo" lines.
		if value == "" {
			var items []string
			for _, next := range lines[i+1:] {
				trimmed := strings.TrimSpace(next)
				if strings.HasPrefix(trimmed, "- ") {
					items = append(items, strings.TrimPrefix(trimmed, "- "))
				} else if trimmed != "" {
					break // stop at the next non-list key
				}
			}
			return items
		}

		// Inline array form: "[foo, bar]"
		if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
			value = value[1 : len(value)-1]
		}

		// Inline comma-separated form.
		parts := strings.Split(value, ",")
		result := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.TrimSpace(p); t != "" {
				result = append(result, t)
			}
		}
		return result
	}
	return nil
}

// InjectFrontmatterTags prepends a YAML frontmatter block with the given tags to content.
// Returns content unchanged if frontmatter is already present or tags is empty.
func InjectFrontmatterTags(content string, tags []string) string {
	if len(tags) == 0 || strings.HasPrefix(content, "---\n") {
		return content
	}
	header := "---\ntags: " + strings.Join(tags, ", ") + "\n---\n\n"
	return header + content
}

// NormalizeTags validates and normalizes tag names: lowercase, strip invalid
// characters, deduplicate, drop oversized tags, and sort for deterministic output.
func NormalizeTags(tags []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(tags))

	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		normalized = validCharsRegex.ReplaceAllString(normalized, "")
		if normalized == "" || len(normalized) > maxTagLength {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	sort.Strings(result)
	return result
}

// MigratePlanTagsToFrontmatter backfills YAML frontmatter into plans that have
// tags in the database but no frontmatter block in their file content.
// It writes both the viewer-dir copy and the original source file.
// Safe to run multiple times — skips plans that already have frontmatter tags.
func (s *Service) MigratePlanTagsToFrontmatter(ctx context.Context) (int, error) {
	plans, err := s.db.ListPlansWithTags(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list tagged plans: %w", err)
	}

	migrated := 0
	for _, plan := range s.toSummaries(ctx, plans) {
		if len(plan.Tags) == 0 {
			continue
		}

		viewerPath := filepath.Join(
			s.viewerDir,
			s.viewerSubdirFor(plan.SyncSource),
			plan.FileName,
		)
		//nolint:gosec // G304: path is controlled by the application, not user input
		raw, err := os.ReadFile(viewerPath)
		if err != nil {
			s.logger.Warn("migrate-tags: cannot read viewer file",
				"file", plan.FileName, "error", err)
			continue
		}

		// Skip if the file already declares tags in frontmatter.
		if len(parseFrontmatterTags(string(raw))) > 0 {
			continue
		}

		tagNames := make([]string, len(plan.Tags))
		for i, t := range plan.Tags {
			tagNames[i] = t.Name
		}
		updated := InjectFrontmatterTags(string(raw), tagNames)

		if err := os.WriteFile(viewerPath, []byte(updated), 0o600); err != nil {
			return migrated, fmt.Errorf("failed to write viewer file %s: %w", plan.FileName, err)
		}

		//nolint:gosec // G304: path is controlled by the application, not user input
		sourcePath := filepath.Join(plan.SyncSource, plan.FileName)
		if err := os.WriteFile(sourcePath, []byte(updated), 0o600); err != nil {
			// Non-fatal: viewer copy is updated; source will catch up on next rsync.
			s.logger.Warn("migrate-tags: cannot write source file",
				"file", plan.FileName, "error", err)
		}

		migrated++
	}
	return migrated, nil
}
