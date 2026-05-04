package claudeviewer

import (
	"regexp"
	"strings"
)

var (
	hashtagRegex    = regexp.MustCompile(`#(\w+)`)
	tagsLineRegex   = regexp.MustCompile(`(?i)^tags?:\s*(.+)$`)
	validCharsRegex = regexp.MustCompile(`[^a-z0-9_-]+`)
)

// ExtractTagsFromContent parses plan content for inline tags.
func ExtractTagsFromContent(content string) []string {
	tags := make(map[string]struct{})

	// Extract hashtags (#tag)
	matches := hashtagRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			tags[match[1]] = struct{}{}
		}
	}

	// Extract from "Tags:" or "tags:" metadata line.
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		matches := tagsLineRegex.FindStringSubmatch(line)
		if len(matches) > 1 {
			// Split by comma and trim each tag
			tagList := strings.Split(matches[1], ",")
			for _, tag := range tagList {
				tag = strings.TrimSpace(tag)
				if tag != "" {
					tags[tag] = struct{}{}
				}
			}
		}
	}

	// Convert map to slice
	result := make([]string, 0, len(tags))
	for tag := range tags {
		result = append(result, tag)
	}

	return NormalizeTags(result)
}

// NormalizeTags validates and normalizes tag names.
func NormalizeTags(tags []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(tags))

	for _, tag := range tags {
		normalized := strings.ToLower(strings.TrimSpace(tag))
		normalized = validCharsRegex.ReplaceAllString(normalized, "")
		if normalized == "" {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}

		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}

	return result
}
