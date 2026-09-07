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
	FocusDiff                      // Viewing a fullscreen version diff (VersionsScreen).
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
	ModalMetadata                 // metadata popup (paths, size, reading time, timestamps)
)

type EditorMode int

const (
	EditorModeNavigation EditorMode = iota
	EditorModeInsert
)

// Vertical-orientation split layout constants.
const (
	// VerticalHeightOverhead is how much of the terminal height goes to
	// border rows, the divider, and the 1-row status-bar safety margin every
	// render function reserves (App.View() appends a status-bar line below
	// whatever a screen renders).
	VerticalHeightOverhead = 7

	// VerticalListRatioNum / VerticalListRatioDenom: the list (or, in
	// three-panel mode, the side-panel+list row) gets this fraction of the
	// remaining height; content gets the rest.
	VerticalListRatioNum   = 3
	VerticalListRatioDenom = 10
)

// Horizontal-orientation split layout constants, used by the two-panel
// renderSplitView (list | content, side-by-side) in both PlansScreen and
// VersionsScreen.
const (
	// HorizontalPanelWidthOverhead is subtracted from the terminal width
	// before splitting it evenly between the two side-by-side panels.
	HorizontalPanelWidthOverhead = 3

	// HorizontalHeightOverhead is subtracted from the terminal height to get
	// each panel's content height.
	HorizontalHeightOverhead = 4
)
