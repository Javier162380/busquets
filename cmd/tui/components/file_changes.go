package components

import (
	"fmt"
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"

	"github.com/charmbracelet/lipgloss"
)

// FileChangesBar displays a compact list of changed files.
type FileChangesBar struct {
	changes []claudeviewer.SessionFileChangeInfo
	width   int
}

// NewFileChangesBar creates a new file changes bar component.
func NewFileChangesBar(width int) *FileChangesBar {
	return &FileChangesBar{
		changes: []claudeviewer.SessionFileChangeInfo{},
		width:   width,
	}
}

// SetChanges sets the file changes to display.
func (f *FileChangesBar) SetChanges(changes []claudeviewer.SessionFileChangeInfo) {
	f.changes = changes
}

// SetSize updates the bar width.
func (f *FileChangesBar) SetSize(width int) {
	f.width = width
}

// View renders the file changes bar.
func (f *FileChangesBar) View() string {
	if len(f.changes) == 0 {
		return styles.MetaStyle.Render("No file changes detected")
	}

	// Group changes by type
	modified := []string{}
	created := []string{}
	deleted := []string{}

	for _, change := range f.changes {
		fileName := change.FilePath
		// Extract just the filename if it's a path
		if idx := strings.LastIndex(fileName, "/"); idx != -1 {
			fileName = fileName[idx+1:]
		}

		switch change.ChangeType {
		case "modified":
			modified = append(modified, fileName)
		case "created":
			created = append(created, fileName)
		case "deleted":
			deleted = append(deleted, fileName)
		}
	}

	var parts []string

	if len(modified) > 0 {
		modifiedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("11")) // Yellow
		parts = append(parts, modifiedStyle.Render(fmt.Sprintf("M:%d", len(modified))))
	}

	if len(created) > 0 {
		createdStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("10")) // Green
		parts = append(parts, createdStyle.Render(fmt.Sprintf("A:%d", len(created))))
	}

	if len(deleted) > 0 {
		deletedStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("9")) // Red
		parts = append(parts, deletedStyle.Render(fmt.Sprintf("D:%d", len(deleted))))
	}

	prefix := styles.MetaStyle.Render("Files changed: ")
	summary := strings.Join(parts, " ")

	// Show first few filenames if space permits
	fileList := ""
	allFiles := append(append(modified, created...), deleted...)
	if len(allFiles) > 0 && f.width > 40 {
		maxFiles := 3
		if len(allFiles) < maxFiles {
			maxFiles = len(allFiles)
		}
		fileList = " (" + strings.Join(allFiles[:maxFiles], ", ")
		if len(allFiles) > maxFiles {
			fileList += fmt.Sprintf(", +%d more", len(allFiles)-maxFiles)
		}
		fileList += ")"
	}

	return prefix + summary + styles.MetaStyle.Render(fileList)
}

// Count returns the total number of file changes.
func (f *FileChangesBar) Count() int {
	return len(f.changes)
}
