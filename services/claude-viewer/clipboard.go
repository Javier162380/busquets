package claudeviewer

import (
	"context"
	"fmt"
)

// Clipboard writes text to the system clipboard using the given mode
// ("auto"/"native"/"osc52"). Implementations may let an env var override the
// mode. It is implemented by internal/clipboard and faked in tests.
type Clipboard interface {
	Write(text, mode string) error
}

// CopyPlanContent writes a plan's raw markdown content to the system clipboard.
// The clipboard mode comes from the clipboard_mode setting (overridable via the
// PLAN_VIEWER_CLIPBOARD env var). Only the content is copied — the plan is not
// modified.
func (s *Service) CopyPlanContent(ctx context.Context, fileName, syncSource string) error {
	detail, err := s.GetPlanDetailByFileName(ctx, fileName, syncSource)
	if err != nil {
		return fmt.Errorf("failed to load plan %q: %w", fileName, err)
	}
	if err := s.clipboard.Write(detail.Content, s.getClipboardMode(ctx)); err != nil {
		return fmt.Errorf("failed to copy plan to clipboard: %w", err)
	}
	return nil
}
