package claudeviewer

import (
	"regexp"
	"strings"
)

// ExtractTagsFromContent parses plan content for inline tags.
// Supports multiple formats:
// - #tag (hashtags)
// - Tags: tag1, tag2, tag3
// - tags: tag1, tag2
func ExtractTagsFromContent(content string) []string {
	tags := make(map[string]struct{})

	// Extract hashtags (#tag)
	hashtagRegex := regexp.MustCompile(`#(\w+)`)
	matches := hashtagRegex.FindAllStringSubmatch(content, -1)
	for _, match := range matches {
		if len(match) > 1 {
			tags[match[1]] = struct{}{}
		}
	}

	// Extract from "Tags:" or "tags:" metadata line
	// Match lines like: "Tags: tag1, tag2, tag3" or "tags: foo, bar"
	tagsLineRegex := regexp.MustCompile(`(?i)^tags?:\s*(.+)$`)
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
// Rules:
// - Convert to lowercase
// - Trim whitespace
// - Remove duplicates
// - Remove empty strings
// - Remove invalid characters (keep only alphanumeric, dash, underscore)
func NormalizeTags(tags []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(tags))

	// Regex to keep only alphanumeric, dash, and underscore
	validCharsRegex := regexp.MustCompile(`[^a-z0-9_-]+`)

	for _, tag := range tags {
		// Convert to lowercase and trim
		normalized := strings.ToLower(strings.TrimSpace(tag))

		// Remove invalid characters
		normalized = validCharsRegex.ReplaceAllString(normalized, "")

		// Skip empty strings or already seen tags
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
