package claudeviewer

import "github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

// IsNotFound reports whether err is a not-found error.
func IsNotFound(err error) bool { return dto.IsNotFound(err) }
