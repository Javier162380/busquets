// Package types represents shared types over the implementation.
package types

type Layout int

const (
	LayoutSplit      Layout = iota // Two panels side-by-side.
	LayoutFullscreen               // Single content area.
)

// Focus determines where user attention is.
type Focus int

const (
	FocusList          Focus = iota // Navigating a list.
	FocusContent                    // Viewing content.
	FocusEditor                     // Editing content.
	FocusSearch                     // Search input.
	FocusTagFilter                  // Tag filter input.
	FocusMessageViewer              // Viewing session messages.
	FocusMessageInput               // Typing a message.
)
