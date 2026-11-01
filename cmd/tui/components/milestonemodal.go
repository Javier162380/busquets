package components

import (
	"strings"

	"github.com/Javier162380/busquets/cmd/tui/messages"
	"github.com/Javier162380/busquets/cmd/tui/styles"
	"github.com/Javier162380/busquets/services/busquets"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// milestoneHints explains each milestone in the checklist.
var milestoneHints = map[busquets.VersionMilestone]string{
	busquets.MilestoneLLMGenerated:             "content written by an assistant",
	busquets.MilestoneHumanReview:              "reviewed or edited by a human",
	busquets.MilestonePlanImplementationStart:  "implementation began from this version",
	busquets.MilestonePlanImplementationFinish: "implementation complete",
	busquets.MilestonePlanAbandoned:            "dropped or superseded",
}

// MilestoneModal toggles a version's lifecycle milestones from the fixed enum. Validation stays in the service.
type MilestoneModal struct {
	active        bool
	planID        int64
	versionNumber int64
	selected      map[busquets.VersionMilestone]bool
	cursor        int
	width, height int
}

// NewMilestoneModal creates an inactive milestone modal.
func NewMilestoneModal() *MilestoneModal {
	return &MilestoneModal{selected: make(map[busquets.VersionMilestone]bool)}
}

// Open activates the modal for one version, pre-checking its current milestones.
func (m *MilestoneModal) Open(planID, versionNumber int64, current []busquets.VersionMilestone) {
	m.planID = planID
	m.versionNumber = versionNumber
	clear(m.selected)
	for _, milestone := range current {
		m.selected[milestone] = true
	}
	m.cursor = 0
	m.active = true
}

// IsActive returns whether the modal is visible.
func (m *MilestoneModal) IsActive() bool {
	return m.active
}

// SetSize updates the modal dimensions.
func (m *MilestoneModal) SetSize(width, height int) {
	m.width = width
	m.height = height
}

// Update handles keys; enter emits the full selected set, esc discards it.
func (m *MilestoneModal) Update(msg tea.Msg) tea.Cmd {
	if !m.active {
		return nil
	}
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil
	}

	switch keyMsg.String() {
	case "up":
		m.cursor = max(m.cursor-1, 0)
	case "down":
		m.cursor = min(m.cursor+1, len(busquets.VersionMilestones)-1)
	case " ", "x":
		milestone := busquets.VersionMilestones[m.cursor]
		m.selected[milestone] = !m.selected[milestone]
	case "enter":
		m.active = false
		save := messages.SetVersionMilestonesMsg{
			PlanID:        m.planID,
			VersionNumber: m.versionNumber,
			Milestones:    m.selectedInOrder(),
		}
		return func() tea.Msg { return save }
	case "esc":
		m.active = false
	}
	return nil
}

// selectedInOrder returns the checked milestones in lifecycle order.
func (m *MilestoneModal) selectedInOrder() []busquets.VersionMilestone {
	var selected []busquets.VersionMilestone
	for _, milestone := range busquets.VersionMilestones {
		if m.selected[milestone] {
			selected = append(selected, milestone)
		}
	}
	return selected
}

// View renders the checklist.
func (m *MilestoneModal) View() string {
	if !m.active {
		return ""
	}

	modalStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(styles.AccentColor).
		Padding(1, 2).
		Width(m.width - 2)
	titleStyle := lipgloss.NewStyle().Foreground(styles.AccentColor).Bold(true)
	selectedStyle := lipgloss.NewStyle().Foreground(styles.AccentColor).Bold(true)
	mutedStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)

	var body strings.Builder
	body.WriteString(titleStyle.Render("Version milestones"))
	body.WriteString("\n\n")

	for i, milestone := range busquets.VersionMilestones {
		check := "[ ]"
		if m.selected[milestone] {
			check = "[x]"
		}
		row := check + " " + string(milestone)
		if i == m.cursor {
			body.WriteString(selectedStyle.Render("❯ " + row))
		} else {
			body.WriteString("  " + row)
		}
		body.WriteString("\n")
		body.WriteString(mutedStyle.Render("      " + milestoneHints[milestone]))
		body.WriteString("\n")
	}

	body.WriteString("\n")
	body.WriteString(mutedStyle.Render("up/down: select  space/x: toggle  enter: save  esc: cancel"))

	return modalStyle.Render(body.String())
}
