package components

import (
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// TagModalFocus represents which part of the modal has focus.
type TagModalFocus int

const (
	TagModalFocusTagList TagModalFocus = iota
	TagModalFocusInput
)

// TagModal is a component for managing tags on a plan.
type TagModal struct {
	isActive     bool
	planFileName string
	allTags      []dto.Tag
	planTags     []dto.Tag
	selectedTags map[string]bool // Track which tags are selected
	input        textinput.Model
	selectedIdx  int
	focus        TagModalFocus
	width        int
	height       int
}

// SavePlanTagsMsg is sent when the modal is closed to save tags.
type SavePlanTagsMsg struct {
	FileName string
	Tags     []string
}

// NewTagModal creates a new tag modal.
func NewTagModal() *TagModal {
	ti := textinput.New()
	ti.Placeholder = "Enter new tag name..."
	ti.CharLimit = 50

	return &TagModal{
		input:        ti,
		selectedTags: make(map[string]bool),
		width:        60,
		height:       20,
	}
}

// Open opens the modal with the given plan and tags.
func (m *TagModal) Open(fileName string, planTags, allTags []dto.Tag) {
	m.isActive = true
	m.planFileName = fileName
	m.planTags = planTags
	m.allTags = allTags
	m.selectedIdx = 0
	m.focus = TagModalFocusTagList
	m.input.Reset()

	// Initialize selected tags from plan tags
	m.selectedTags = make(map[string]bool)
	for _, tag := range planTags {
		m.selectedTags[tag.Name] = true
	}
}

// Close closes the modal and returns the selected tag names.
func (m *TagModal) Close() []string {
	m.isActive = false
	tags := make([]string, 0, len(m.selectedTags))
	for tag, selected := range m.selectedTags {
		if selected {
			tags = append(tags, tag)
		}
	}
	return tags
}

// IsActive returns whether the modal is active.
func (m *TagModal) IsActive() bool {
	return m.isActive
}

// ToggleTag toggles a tag on or off.
func (m *TagModal) ToggleTag(tagName string) {
	m.selectedTags[tagName] = !m.selectedTags[tagName]
}

// AddNewTag adds a new tag to the selected tags.
func (m *TagModal) AddNewTag(tagName string) {
	if tagName != "" {
		m.selectedTags[tagName] = true
		m.input.Reset()
	}
}

// Update handles messages for the tag modal.
func (m *TagModal) Update(msg tea.Msg) tea.Cmd {
	if !m.isActive {
		return nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// Close and save
			tags := m.Close()
			return func() tea.Msg {
				return SavePlanTagsMsg{
					FileName: m.planFileName,
					Tags:     tags,
				}
			}

		case "tab":
			// Switch focus
			if m.focus == TagModalFocusTagList {
				m.focus = TagModalFocusInput
				m.input.Focus()
			} else {
				m.focus = TagModalFocusTagList
				m.input.Blur()
			}

		case "up":
			if m.focus == TagModalFocusTagList && m.selectedIdx > 0 {
				m.selectedIdx--
			}

		case "down":
			if m.focus == TagModalFocusTagList && m.selectedIdx < len(m.allTags)-1 {
				m.selectedIdx++
			}

		case " ":
			// Toggle tag
			if m.focus == TagModalFocusTagList && len(m.allTags) > 0 {
				tagName := m.allTags[m.selectedIdx].Name
				m.ToggleTag(tagName)
			}

		case "enter":
			// Add new tag from input
			if m.focus == TagModalFocusInput {
				m.AddNewTag(m.input.Value())
			}
		case "d":
			if m.focus == TagModalFocusTagList {
				tagID := m.allTags[m.selectedIdx].ID
				return func() tea.Msg {
					return DeleteTagMsg{
						TagID: tagID, CurrentPlan: m.planFileName,
					}
				}
			}
		}

		// Update input if focused
		if m.focus == TagModalFocusInput {
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return cmd
		}
	default:
		return nil
	}

	return nil
}

// View renders the tag modal.
func (m *TagModal) View() string {
	if !m.isActive {
		return ""
	}

	// Styles
	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("162")).
		Padding(1, 2).
		Width(m.width).
		Height(m.height)

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("162")).
		MarginBottom(1)

	sectionStyle := lipgloss.NewStyle().
		MarginTop(1).
		MarginBottom(1)

	selectedTagStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("35")).
		Bold(true).
		Padding(0, 1)

	helpStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		MarginTop(1)

	// Build content
	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render("Manage Tags: " + m.planFileName))
	content.WriteString("\n\n")

	// Current tags section
	content.WriteString(sectionStyle.Render("Current Tags:"))
	content.WriteString("\n")
	if len(m.selectedTags) == 0 {
		content.WriteString("  (no tags)\n")
	} else {
		currentTags := make([]string, 0)
		for tag, selected := range m.selectedTags {
			if selected {
				currentTags = append(currentTags, selectedTagStyle.Render("● "+tag))
			}
		}
		content.WriteString("  " + strings.Join(currentTags, "  ") + "\n")
	}

	// Available tags section
	content.WriteString("\n")
	content.WriteString(sectionStyle.Render("Available Tags:"))
	content.WriteString("\n")
	if len(m.allTags) == 0 {
		content.WriteString("  (no tags available)\n")
	} else {
		for i, tag := range m.allTags {
			prefix := "  "
			if m.focus == TagModalFocusTagList && i == m.selectedIdx {
				prefix = "❯ "
			}

			checkbox := "☐"
			if m.selectedTags[tag.Name] {
				checkbox = "☑"
			}

			line := prefix + checkbox + " " + tag.Name
			if m.focus == TagModalFocusTagList && i == m.selectedIdx {
				content.WriteString(titleStyle.Render(line) + "\n")
			} else {
				content.WriteString(line + "\n")
			}
		}
	}

	// Add tag input
	content.WriteString("\n")
	content.WriteString(sectionStyle.Render("Add New Tag:"))
	content.WriteString("\n")
	if m.focus == TagModalFocusInput {
		content.WriteString("  " + m.input.View() + "\n")
	} else {
		content.WriteString("  " + styles.InactiveStyle.Render(m.input.View()) + "\n")
	}

	// Help text
	content.WriteString("\n")
	helpText := "[Space] Toggle | [Enter] Add | [Tab] Switch | [ESC] Save & Close"
	content.WriteString(helpStyle.Render(helpText))

	return modalStyle.Render(content.String())
}

// DeleteTagMsg delete tags for plans searching.
type DeleteTagMsg struct {
	TagID       int64
	CurrentPlan string
}

type DeleteTagCmdMsg struct {
	Error    *string
	FileName string
}
