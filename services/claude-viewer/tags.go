package claudeviewer

import (
	"regexp"
	"sort"
	"strings"
)

var validCharsRegex = regexp.MustCompile(`[^a-z0-9_-]+`)

const maxTagLength = 64

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
