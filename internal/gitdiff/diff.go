// Package gitdiff computes git-diff-style unified diffs between two text blobs.
package gitdiff

import (
	"strings"

	"github.com/pmezard/go-difflib/difflib"
)

// Diff returns a unified diff of from against to, labeled with fromLabel/toLabel
// (e.g. "Version 3") as the diff's file headers, with 3 lines of context —
// the same shape `git diff` produces.
func Diff(from, to, fromLabel, toLabel string) (string, error) {
	ud := difflib.UnifiedDiff{
		A:        difflib.SplitLines(from),
		B:        difflib.SplitLines(to),
		FromFile: fromLabel,
		ToFile:   toLabel,
		Context:  3, // unchanged lines shown around each hunk — git diff's default (-U3)
	}
	return difflib.GetUnifiedDiffString(ud)
}

// Stats returns the number of lines added and removed between from and to.
// Counts come from difflib's op-codes, not from parsing rendered diff text,
// which would miscount content lines that legitimately begin with '+' or '-'.
func Stats(from, to string) (added, removed int) {
	matcher := difflib.NewMatcher(splitLines(from), splitLines(to))
	for _, op := range matcher.GetOpCodes() {
		switch op.Tag {
		case 'r': // replaced
			removed += op.I2 - op.I1
			added += op.J2 - op.J1
		case 'd': // deleted
			removed += op.I2 - op.I1
		case 'i': // inserted
			added += op.J2 - op.J1
		case 'e': // equal, nothing to count
		}
	}
	return added, removed
}

// splitLines splits s into lines. difflib.SplitLines is not used: it appends a
// "\n" element, inventing a line that the input does not contain — harmless
// when rendering a diff, wrong when counting.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(s, "\n")
	// A trailing newline yields a final empty element that is not a line.
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}
