// Package types represents shared types over the implementation.
package types

type Layout int

const (
	LayoutSplit      Layout = iota //nolint:gofumpt,gci    // Two panels side-by-side.
	LayoutFullscreen               // Single content area.
	LayoutThreePanel               // Three panels: tags or labels + list + content.
)

// Focus determines where user attention is.
type Focus int

const (
	FocusList         Focus = iota // Navigating a list.
	FocusContent                   // Viewing content.
	FocusEditor                    // Editing content.
	FocusSearch                    // Search input.
	FocusTagFilter                 // Tag filter input.
	FocusTagPanel                  // Left-side tag navigation panel.
	FocusCommentList               // Navigating the comment list in the comment modal.
	FocusCommentInput              // Composing a new comment in the comment modal.
	FocusLabelPanel                // Left-side sync-label navigation panel.
)

// ModalState tracks which overlay is currently active on a screen
// (PlansScreen and VersionsScreen both use it).
type ModalState int

const (
	ModalNone          ModalState = iota
	ModalTagManager               // tag management modal
	ModalComment                  // comment modal
	ModalTLDR                     // TLDR/summary popup
	ModalRenameFile               // rename-plan-file modal
	ModalTextInput                // generic single-field input modal (e.g. go-to-line)
	ModalContentSearch            // content search modal (viewer search)
)
