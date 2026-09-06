// Package gitdiff computes git-diff-style unified diffs between two text blobs.
package gitdiff

import "github.com/pmezard/go-difflib/difflib"

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
