package components

import (
	"sort"
	"strings"
)

// ContentSearch indexes raw markdown source for case-insensitive substring
// search and maps byte-offset matches back to 1-indexed lines.
type ContentSearch struct {
	raw         string
	lower       string
	lineOffsets []int // lineOffsets[i] = byte offset where line i+1 starts

	query   string
	matches []int // byte offsets into raw, in ascending order
	current int   // index into matches, -1 if none
}

// NewContentSearch creates an empty search index.
func NewContentSearch() *ContentSearch { return &ContentSearch{current: -1} }

// SetContent rebuilds the index for new raw content and re-applies the
// current query (if any) against it, so switching plans mid-search doesn't
// leave stale matches from the previous plan's content.
func (c *ContentSearch) SetContent(raw string) {
	c.raw = raw
	c.lower = strings.ToLower(raw)
	c.rebuildLineOffsets()
	c.SetQuery(c.query, 1)
}

// rebuildLineOffsets recomputes the line-start table for c.raw. Split out of
// SetContent so SetContent reads as four named steps rather than an inlined
// loop — this can't move to NewContentSearch, though: at construction time
// (Viewer's own constructor, before any plan is loaded) there is no content
// yet to index. SetContent runs once per plan/version load for the lifetime
// of the Viewer, which is why this rebuilds rather than initializes once —
// the index has to track whatever content is currently displayed, not a
// fixed value from construction.
func (c *ContentSearch) rebuildLineOffsets() {
	c.lineOffsets = c.lineOffsets[:0] // reuse the backing array instead of reallocating on every plan switch
	c.lineOffsets = append(c.lineOffsets, 0)
	for i, r := range c.raw {
		if r == '\n' {
			c.lineOffsets = append(c.lineOffsets, i+1)
		}
	}
}

// SetQuery re-scans for query (case-insensitive substring, via repeated
// strings.Index over the cached lowercased content — no tokenizing) and
// selects the first match at or after fromLine (1-indexed), wrapping to the
// top if none follow it.
func (c *ContentSearch) SetQuery(query string, fromLine int) {
	c.query = query
	c.matches = c.matches[:0]
	c.current = -1
	if query == "" {
		return
	}

	needle := strings.ToLower(query)
	for offset := 0; ; {
		idx := strings.Index(c.lower[offset:], needle)
		if idx < 0 {
			break
		}
		c.matches = append(c.matches, offset+idx)
		offset += idx + len(needle) // advance past this match, allow overlaps to be skipped
	}
	if len(c.matches) == 0 {
		return
	}

	from := c.lineStart(fromLine)
	c.current = sort.Search(len(c.matches), func(i int) bool { return c.matches[i] >= from })
	if c.current == len(c.matches) {
		c.current = 0 // no match at/after fromLine — wrap to the first one in the document
	}
}

// HasMatches reports whether the current query has any matches.
func (c *ContentSearch) HasMatches() bool { return len(c.matches) > 0 }

// MatchCount returns the total number of matches for the current query.
func (c *ContentSearch) MatchCount() int { return len(c.matches) }

// Query returns the current search query.
func (c *ContentSearch) Query() string { return c.query }

// CurrentIndex returns the 1-indexed position of the active match among all
// matches (for a "3/12" indicator), or 0 if there is none.
func (c *ContentSearch) CurrentIndex() int {
	if c.current < 0 {
		return 0
	}
	return c.current + 1
}

// CurrentLine returns the 1-indexed line of the active match, or 0 if none.
func (c *ContentSearch) CurrentLine() int {
	if c.current < 0 {
		return 0
	}
	return c.lineForOffset(c.matches[c.current])
}

// Next advances to the next match, wrapping around, and returns its line.
func (c *ContentSearch) Next() int { return c.step(1) }

// Prev moves to the previous match, wrapping around, and returns its line.
func (c *ContentSearch) Prev() int { return c.step(-1) }

func (c *ContentSearch) step(dir int) int {
	if len(c.matches) == 0 {
		return 0
	}
	c.current = (c.current + dir + len(c.matches)) % len(c.matches)
	return c.CurrentLine()
}

// CurrentRange returns the active match's 1-indexed line and its [start,end)
// byte range relative to that line's start, or ok=false if there is none.
// Used to distinguish the active match from other matches on the same line
// when highlighting in raw mode.
func (c *ContentSearch) CurrentRange() (line, start, end int, ok bool) {
	if c.current < 0 {
		return 0, 0, 0, false
	}
	offset := c.matches[c.current]
	line = c.lineForOffset(offset)
	lo := c.lineStart(line)
	return line, offset - lo, offset - lo + len(c.query), true
}

// MatchesOnLine returns [start,end) byte ranges relative to the line's own
// start, for every match on that 1-indexed line. Used for raw-mode highlight.
func (c *ContentSearch) MatchesOnLine(line int) [][2]int {
	lo, hi := c.lineStart(line), c.lineStart(line+1)
	var ranges [][2]int
	for _, off := range c.matches {
		if off >= lo && off < hi {
			ranges = append(ranges, [2]int{off - lo, off - lo + len(c.query)})
		}
	}
	return ranges
}

// lineForOffset maps a byte offset in raw to its 1-indexed line number via
// binary search over lineOffsets.
func (c *ContentSearch) lineForOffset(offset int) int {
	return sort.Search(len(c.lineOffsets), func(i int) bool { return c.lineOffsets[i] > offset })
}

// lineStart returns the byte offset where the given 1-indexed line begins.
func (c *ContentSearch) lineStart(line int) int {
	if line < 1 {
		line = 1
	}
	if line > len(c.lineOffsets) {
		return len(c.raw)
	}
	return c.lineOffsets[line-1]
}
