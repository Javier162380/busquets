package screens

import (
	"fmt"
	"strings"
	"time"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/components"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/content"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"
	"github.com/Javier162380/claude-plan-viewer/cmd/tui/types"
	"github.com/Javier162380/claude-plan-viewer/services/claude-viewer/dto"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// JobsScreen handles background job management.
type JobsScreen struct {
	// Components
	list   *components.List
	viewer *components.Viewer

	// State
	layout  types.Layout
	focus   types.Focus
	jobs    []JobWithPlan
	current *JobDetail

	// Dimensions
	width  int
	height int

	// Styles
	borderStyle lipgloss.Style

	// Theme
	isDarkModeEnabled bool
}

// JobWithPlan represents a job with its associated plan info.
type JobWithPlan struct {
	ID            string
	Name          string
	PlanName      string
	Status        dto.JobStatus
	AgentProvider string
	LastRunAt     *time.Time
	ScheduledAt   *time.Time
}

// JobDetail represents detailed job information with execution history.
type JobDetail struct {
	Job        dto.BackgroundJob
	PlanName   string
	Executions []dto.ExecutionWithJob
	Scheduled  *dto.ScheduledJob
}

// NewJobsScreen creates a new jobs screen.
func NewJobsScreen(width, height int, isDarkModeEnabled bool) *JobsScreen {
	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	return &JobsScreen{
		list:   components.NewList(nil, panelWidth, contentHeight),
		viewer: components.NewViewer(panelWidth, contentHeight),
		layout: types.LayoutSplit,
		focus:  types.FocusList,
		width:  width,
		height: height,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
		isDarkModeEnabled: isDarkModeEnabled,
	}
}

// Init initializes the screen.
func (s *JobsScreen) Init() tea.Cmd {
	return nil // Jobs loaded via JobsLoadedMsg from App
}

// Update handles messages.
func (s *JobsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case JobsLoadedMsg:
		s.jobs = convertJobsToDisplay(msg.Jobs)
		s.updateListItems()
		if len(s.jobs) > 0 {
			return s, s.loadJobDetail(s.jobs[0].ID)
		}
		return s, nil

	case JobDetailLoadedMsg:
		s.current = msg.Detail
		s.updateViewer()
		return s, nil

	case JobResultMsg:
		// Job execution completed - reload job detail
		if s.current != nil && s.current.Job.ID == msg.JobID {
			return s, s.loadJobDetail(msg.JobID)
		}
		// Also reload list to update status
		return s, func() tea.Msg {
			return RequestJobsReloadMsg{}
		}

	case RequestJobsReloadMsg:
		return s, func() tea.Msg {
			return LoadJobsMsg{}
		}
	}

	return s, nil
}

// handleKey processes keyboard input.
func (s *JobsScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	switch msg.String() {
	case "esc":
		return s, func() tea.Msg { return PopScreenMsg{} }

	case "j", "down":
		// Move down
		currentIdx := s.list.SelectedIndex()
		if currentIdx < len(s.jobs)-1 {
			s.list.Select(currentIdx + 1)
			return s, s.loadJobDetail(s.jobs[currentIdx+1].ID)
		}

	case "k", "up":
		// Move up
		currentIdx := s.list.SelectedIndex()
		if currentIdx > 0 {
			s.list.Select(currentIdx - 1)
			return s, s.loadJobDetail(s.jobs[currentIdx-1].ID)
		}

	case "t":
		// Trigger job manually
		if s.current != nil {
			return s, func() tea.Msg {
				return TriggerJobMsg{JobID: s.current.Job.ID}
			}
		}

	case "s":
		// Schedule job (prompt for time)
		if s.current != nil {
			// TODO: Open schedule modal
			return s, func() tea.Msg {
				return ScheduleJobPromptMsg{JobID: s.current.Job.ID}
			}
		}

	case "x":
		// Cancel scheduled job
		if s.current != nil && s.current.Scheduled != nil && !s.current.Scheduled.Cancelled {
			return s, func() tea.Msg {
				return CancelScheduledJobMsg{JobID: s.current.Job.ID}
			}
		}

	case "c":
		// Cancel running execution
		if s.current != nil && len(s.current.Executions) > 0 {
			latest := s.current.Executions[0]
			if latest.Execution.Status == dto.ExecutionStatusRunning {
				return s, func() tea.Msg {
					return CancelExecutionMsg{ExecutionID: latest.Execution.ID}
				}
			}
		}

	case "d":
		// Delete job (with confirmation)
		if s.current != nil {
			return s, func() tea.Msg {
				return DeleteJobPromptMsg{JobID: s.current.Job.ID, JobName: s.current.Job.Name}
			}
		}

	case "n":
		// Create new job
		return s, func() tea.Msg {
			return CreateJobPromptMsg{}
		}

	case "r":
		// Reload jobs
		return s, func() tea.Msg {
			return LoadJobsMsg{}
		}
	}

	return s, nil
}

// updateListItems updates the job list display.
func (s *JobsScreen) updateListItems() {
	items := make([]components.ListItem, len(s.jobs))
	for i, job := range s.jobs {
		status := formatJobStatus(job.Status)

		title := fmt.Sprintf("%s | %s", status, job.Name)

		desc := job.PlanName
		if job.ScheduledAt != nil {
			desc += fmt.Sprintf(" [scheduled: %s]", job.ScheduledAt.Format("15:04"))
		} else if job.LastRunAt != nil {
			desc += fmt.Sprintf(" (last: %s)", formatRelativeTime(*job.LastRunAt))
		}

		items[i] = components.NewListItem(title, desc, job)
	}
	s.list.SetItems(items)
}

// updateViewer updates the detail viewer content.
func (s *JobsScreen) updateViewer() {
	if s.current == nil {
		s.viewer.SetContent(nil)
		return
	}

	var b strings.Builder

	// Job details
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Job Details"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("ID:       %s\n", s.current.Job.ID))
	b.WriteString(fmt.Sprintf("Name:     %s\n", s.current.Job.Name))
	b.WriteString(fmt.Sprintf("Plan:     %s\n", s.current.PlanName))
	b.WriteString(fmt.Sprintf("Provider: %s\n", s.current.Job.AgentProvider))
	b.WriteString(fmt.Sprintf("Status:   %s\n", formatJobStatus(s.current.Job.Status)))

	if s.current.Job.Description != nil {
		b.WriteString(fmt.Sprintf("Desc:     %s\n", *s.current.Job.Description))
	}

	if s.current.Job.LastRunAt != nil {
		b.WriteString(fmt.Sprintf("Last Run: %s\n", s.current.Job.LastRunAt.Format("2006-01-02 15:04:05")))
	}

	// Scheduled execution
	if s.current.Scheduled != nil {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Bold(true).Render("Scheduled Execution"))
		b.WriteString("\n\n")
		b.WriteString(fmt.Sprintf("Time:      %s\n", s.current.Scheduled.ScheduledAt.Format("2006-01-02 15:04:05")))
		b.WriteString(fmt.Sprintf("Cancelled: %v\n", s.current.Scheduled.Cancelled))
	}

	// Execution history
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Bold(true).Render("Execution History"))
	b.WriteString("\n\n")

	if len(s.current.Executions) == 0 {
		b.WriteString("No executions yet\n")
	} else {
		for i, exec := range s.current.Executions {
			if i >= 10 {
				break // Limit to 10 most recent
			}

			status := formatExecutionStatus(exec.Execution.Status)
			startedAt := "N/A"
			if exec.Execution.StartedAt != nil {
				startedAt = exec.Execution.StartedAt.Format("15:04:05")
			}

			duration := ""
			if exec.Execution.CompletedAt != nil && exec.Execution.StartedAt != nil {
				dur := exec.Execution.CompletedAt.Sub(*exec.Execution.StartedAt)
				duration = fmt.Sprintf(" (%s)", dur.Round(time.Second))
			}

			b.WriteString(fmt.Sprintf("#%d %s %s %s%s\n",
				exec.Execution.ExecutionNumber,
				status,
				startedAt,
				exec.Execution.TriggeredBy,
				duration))

			if exec.Execution.ErrorMessage != nil && *exec.Execution.ErrorMessage != "" {
				errMsg := *exec.Execution.ErrorMessage
				if len(errMsg) > 100 {
					errMsg = errMsg[:100] + "..."
				}
				b.WriteString(fmt.Sprintf("    Error: %s\n", errMsg))
			}
		}
	}

	// Keybindings help
	b.WriteString("\n")
	b.WriteString(lipgloss.NewStyle().Faint(true).Render("Keys: t=trigger s=schedule x=cancel-sched c=cancel-run d=delete n=new r=reload"))

	title := "Job Details"
	if s.current != nil {
		title = s.current.Job.Name
	}

	s.viewer.SetContent(content.NewTextContent(title, b.String(), s.isDarkModeEnabled, (s.width-3)/2))
}

// loadJobDetail loads detailed job information.
func (s *JobsScreen) loadJobDetail(jobID string) tea.Cmd {
	return func() tea.Msg {
		return LoadJobDetailMsg{JobID: jobID}
	}
}

// View renders the screen.
func (s *JobsScreen) View() string {
	leftPanel := s.borderStyle.Render(s.list.View())
	rightPanel := s.borderStyle.Render(s.viewer.View())

	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		leftPanel,
		rightPanel,
	)
}

// SetSize updates screen dimensions.
func (s *JobsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height

	panelWidth := (width - 3) / 2
	contentHeight := height - 4

	s.list.SetSize(panelWidth, contentHeight)
	s.viewer.SetSize(panelWidth, contentHeight)
}

// IsInputMode returns whether the screen is in input mode.
func (s *JobsScreen) IsInputMode() bool {
	return false
}

// ShortHelp returns key binding help text.
func (s *JobsScreen) ShortHelp() string {
	return "↑/↓: navigate | t: trigger | s: schedule | x: cancel-sched | c: cancel-run | d: delete | n: new | r: reload | esc: back"
}

// Helper functions

func convertJobsToDisplay(jobs []dto.JobWithPlan) []JobWithPlan {
	result := make([]JobWithPlan, len(jobs))
	for i, job := range jobs {
		result[i] = JobWithPlan{
			ID:            job.Job.ID,
			Name:          job.Job.Name,
			PlanName:      job.PlanName,
			Status:        job.Job.Status,
			AgentProvider: job.Job.AgentProvider,
			LastRunAt:     job.Job.LastRunAt,
		}
	}
	return result
}

func formatJobStatus(status dto.JobStatus) string {
	switch status {
	case dto.JobStatusPending:
		return "⏸ PENDING"
	case dto.JobStatusRunning:
		return "▶ RUNNING"
	case dto.JobStatusCompleted:
		return "✓ COMPLETED"
	case dto.JobStatusFailed:
		return "✗ FAILED"
	case dto.JobStatusCancelled:
		return "⊘ CANCELLED"
	case dto.JobStatusPaused:
		return "⏸ PAUSED"
	default:
		return string(status)
	}
}

func formatExecutionStatus(status dto.ExecutionStatus) string {
	switch status {
	case dto.ExecutionStatusPending:
		return "⏸"
	case dto.ExecutionStatusRunning:
		return "▶"
	case dto.ExecutionStatusCompleted:
		return "✓"
	case dto.ExecutionStatusFailed:
		return "✗"
	case dto.ExecutionStatusCancelled:
		return "⊘"
	default:
		return string(status)
	}
}

func formatRelativeTime(t time.Time) string {
	dur := time.Since(t)

	if dur < time.Minute {
		return "just now"
	} else if dur < time.Hour {
		mins := int(dur.Minutes())
		return fmt.Sprintf("%dm ago", mins)
	} else if dur < 24*time.Hour {
		hours := int(dur.Hours())
		return fmt.Sprintf("%dh ago", hours)
	} else {
		days := int(dur.Hours() / 24)
		return fmt.Sprintf("%dd ago", days)
	}
}

// Messages

// JobsLoadedMsg is sent when jobs are loaded.
type JobsLoadedMsg struct {
	Jobs []dto.JobWithPlan
}

// LoadJobsMsg requests jobs to be loaded.
type LoadJobsMsg struct{}

// JobDetailLoadedMsg is sent when job details are loaded.
type JobDetailLoadedMsg struct {
	Detail *JobDetail
}

// LoadJobDetailMsg requests job details to be loaded.
type LoadJobDetailMsg struct {
	JobID string
}

// TriggerJobMsg requests a job to be triggered.
type TriggerJobMsg struct {
	JobID string
}

// ScheduleJobPromptMsg requests schedule prompt.
type ScheduleJobPromptMsg struct {
	JobID string
}

// CancelScheduledJobMsg requests scheduled job cancellation.
type CancelScheduledJobMsg struct {
	JobID string
}

// CancelExecutionMsg requests execution cancellation.
type CancelExecutionMsg struct {
	ExecutionID string
}

// DeleteJobPromptMsg requests delete confirmation.
type DeleteJobPromptMsg struct {
	JobID   string
	JobName string
}

// CreateJobPromptMsg requests job creation prompt.
type CreateJobPromptMsg struct{}

// JobResultMsg is sent when a job execution completes.
type JobResultMsg struct {
	JobID   string
	Success bool
}

// RequestJobsReloadMsg requests jobs to be reloaded.
type RequestJobsReloadMsg struct{}

// RequestJobsScreenMsg requests the jobs screen to be shown.
type RequestJobsScreenMsg struct{}
