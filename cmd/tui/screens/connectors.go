package screens

import (
	"fmt"
	"strings"

	"github.com/Javier162380/claude-plan-viewer/cmd/tui/styles"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ConnectorStatus represents a connector's current state.
type ConnectorStatus struct {
	Name        string
	DisplayName string
	Enabled     bool
	Configured  bool
}

// ConnectorSettingValue holds a setting's definition and current value.
type ConnectorSettingValue struct {
	Key         string
	DisplayName string
	Description string
	Value       string
	Required    bool
	Sensitive   bool
}

// ConnectorsScreen handles connector configuration.
type ConnectorsScreen struct {
	// State
	connectors      []ConnectorStatus
	settings        []ConnectorSettingValue
	selectedConn    int
	selectedSetting int
	currentConnName string

	// Focus: left panel (connectors) or right panel (settings)
	focusRight bool
	editing    bool

	// Components
	input textinput.Model

	// Dimensions
	width       int
	height      int
	borderStyle lipgloss.Style
}

// NewConnectorsScreen creates a new connectors screen.
func NewConnectorsScreen(width, height int) *ConnectorsScreen {
	ti := textinput.New()
	ti.Placeholder = "Enter value..."
	ti.CharLimit = 256

	return &ConnectorsScreen{
		width:  width,
		height: height,
		input:  ti,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
	}
}

// Init initializes the screen.
func (s *ConnectorsScreen) Init() tea.Cmd {
	return func() tea.Msg {
		return LoadConnectorsMsg{}
	}
}

// Update handles messages.
func (s *ConnectorsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case ConnectorsLoadedMsg:
		s.connectors = msg.Connectors
		// Load settings for first connector if available
		if len(s.connectors) > 0 {
			s.currentConnName = s.connectors[0].Name
			return s, func() tea.Msg {
				return LoadConnectorSettingsMsg{ConnectorName: s.currentConnName}
			}
		}
		return s, nil

	case ConnectorSettingsLoadedMsg:
		if msg.ConnectorName == s.currentConnName {
			s.settings = msg.Settings
			s.selectedSetting = 0
		}
		return s, nil

	case ConnectorUpdateResultMsg:
		if msg.Success {
			// Reload connectors to refresh status
			return s, func() tea.Msg {
				return LoadConnectorsMsg{}
			}
		}
		return s, nil
	}

	// Update text input if editing
	if s.editing {
		var cmd tea.Cmd
		s.input, cmd = s.input.Update(msg)
		return s, cmd
	}

	return s, nil
}

// handleKey processes key input.
func (s *ConnectorsScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()

	// Handle editing mode
	if s.editing {
		return s.handleEditKey(msg)
	}

	// Handle navigation
	switch key {
	case "esc":
		return s, func() tea.Msg {
			return PopScreenMsg{}
		}

	case "tab":
		// Toggle focus between panels
		if len(s.settings) > 0 {
			s.focusRight = !s.focusRight
		}
		return s, nil

	case "j", "down":
		if s.focusRight {
			if s.selectedSetting < len(s.settings)-1 {
				s.selectedSetting++
			}
		} else {
			if s.selectedConn < len(s.connectors)-1 {
				s.selectedConn++
				s.currentConnName = s.connectors[s.selectedConn].Name
				return s, func() tea.Msg {
					return LoadConnectorSettingsMsg{ConnectorName: s.currentConnName}
				}
			}
		}
		return s, nil

	case "k", "up":
		if s.focusRight {
			if s.selectedSetting > 0 {
				s.selectedSetting--
			}
		} else {
			if s.selectedConn > 0 {
				s.selectedConn--
				s.currentConnName = s.connectors[s.selectedConn].Name
				return s, func() tea.Msg {
					return LoadConnectorSettingsMsg{ConnectorName: s.currentConnName}
				}
			}
		}
		return s, nil

	case "e":
		// Edit selected setting (right panel only)
		if s.focusRight && len(s.settings) > 0 {
			setting := s.settings[s.selectedSetting]
			s.editing = true
			s.input.SetValue("")
			if !setting.Sensitive {
				s.input.SetValue(setting.Value)
			}
			if setting.Sensitive {
				s.input.EchoMode = textinput.EchoPassword
			} else {
				s.input.EchoMode = textinput.EchoNormal
			}
			// Return Focus command to start cursor blink
			return s, s.input.Focus()
		}
		return s, nil

	case "enter":
		if !s.focusRight {
			// Enable selected connector (left panel)
			if len(s.connectors) > 0 {
				conn := s.connectors[s.selectedConn]
				return s, func() tea.Msg {
					return EnableConnectorMsg{Name: conn.Name}
				}
			}
		}
		return s, nil

	case "d":
		// Disable connector (only from left panel)
		if !s.focusRight && len(s.connectors) > 0 {
			return s, func() tea.Msg {
				return DisableConnectorMsg{}
			}
		}
		return s, nil

	case "V":
		// Validate current connector settings
		if s.currentConnName != "" {
			return s, func() tea.Msg {
				return ValidateConnectorMsg{Name: s.currentConnName}
			}
		}
		return s, nil
	}

	return s, nil
}

// handleEditKey handles keys in edit mode.
func (s *ConnectorsScreen) handleEditKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()

	switch key {
	case "esc":
		s.editing = false
		s.input.Blur()
		return s, nil

	case "enter":
		// Save the setting
		if len(s.settings) > 0 {
			setting := s.settings[s.selectedSetting]
			value := s.input.Value()
			s.editing = false
			s.input.Blur()
			return s, func() tea.Msg {
				return SaveConnectorSettingMsg{
					ConnectorName: s.currentConnName,
					Key:           setting.Key,
					Value:         value,
					IsSecret:      setting.Sensitive,
				}
			}
		}
		return s, nil
	}

	// Pass the original message to text input
	var cmd tea.Cmd
	s.input, cmd = s.input.Update(msg)
	return s, cmd
}

// View renders the screen.
func (s *ConnectorsScreen) View() string {
	panelWidth := (s.width - 3) / 2
	contentHeight := s.height - 4

	leftPanel := s.renderLeftPanel(panelWidth, contentHeight)
	rightPanel := s.renderRightPanel(panelWidth, contentHeight)
	divider := s.renderDivider(contentHeight)

	return lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, divider, rightPanel)
}

// renderLeftPanel renders the connectors list.
func (s *ConnectorsScreen) renderLeftPanel(width, height int) string {
	var content strings.Builder

	title := styles.TitleStyle.Render("Connectors")
	content.WriteString(title + "\n\n")

	for i, conn := range s.connectors {
		cursor := "  "
		if i == s.selectedConn && !s.focusRight {
			cursor = "❯ "
		}

		status := "○"
		statusStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
		if conn.Enabled {
			status = "✓"
			statusStyle = lipgloss.NewStyle().Foreground(styles.SuccessColor)
		}

		name := conn.DisplayName
		if i == s.selectedConn && !s.focusRight {
			name = styles.ActiveStyle.Render(name)
		}

		line := fmt.Sprintf("%s%s %s", cursor, name, statusStyle.Render(status))
		if conn.Configured {
			line += lipgloss.NewStyle().Foreground(styles.MutedColor).Render(" (configured)")
		}
		content.WriteString(line + "\n")
	}

	if len(s.connectors) == 0 {
		content.WriteString(styles.MutedStyle.Render("No connectors available"))
	}

	borderStyle := s.borderStyle
	if !s.focusRight {
		borderStyle = borderStyle.BorderForeground(styles.AccentColor)
	}

	return borderStyle.
		Width(width).
		Height(height).
		Render(content.String())
}

// renderRightPanel renders the settings form.
func (s *ConnectorsScreen) renderRightPanel(width, height int) string {
	var content strings.Builder

	if s.currentConnName == "" {
		content.WriteString(styles.MutedStyle.Render("Select a connector"))
	} else {
		// Find connector display name
		displayName := s.currentConnName
		for _, c := range s.connectors {
			if c.Name == s.currentConnName {
				displayName = c.DisplayName
				break
			}
		}

		title := styles.TitleStyle.Render(displayName + " Settings")
		content.WriteString(title + "\n\n")

		if len(s.settings) == 0 {
			content.WriteString(styles.MutedStyle.Render("No settings required"))
		} else {
			for i, setting := range s.settings {
				cursor := "  "
				if i == s.selectedSetting && s.focusRight {
					cursor = "❯ "
				}

				label := setting.DisplayName
				if setting.Required {
					label += " *"
				}

				value := setting.Value
				if setting.Sensitive && value != "" {
					value = "••••••••"
				}
				if value == "" {
					value = "(not set)"
				}

				if i == s.selectedSetting && s.focusRight {
					label = styles.ActiveStyle.Render(label)
				}

				content.WriteString(fmt.Sprintf("%s%s\n", cursor, label)) //nolint:staticcheck // convenient.

				if i == s.selectedSetting && s.editing {
					content.WriteString("   " + s.input.View() + "\n")
				} else {
					valueStyle := lipgloss.NewStyle().Foreground(styles.MutedColor)
					content.WriteString("   " + valueStyle.Render(value) + "\n")
				}

				if setting.Description != "" {
					descStyle := lipgloss.NewStyle().Foreground(styles.MutedColor).Italic(true)
					content.WriteString("   " + descStyle.Render(setting.Description) + "\n")
				}
				content.WriteString("\n")
			}
		}
	}

	borderStyle := s.borderStyle
	if s.focusRight {
		borderStyle = borderStyle.BorderForeground(styles.AccentColor)
	}

	return borderStyle.
		Width(width).
		Height(height).
		Render(content.String())
}

// renderDivider renders a vertical divider.
func (s *ConnectorsScreen) renderDivider(height int) string {
	divider := ""
	for i := 0; i < height; i++ {
		divider += "│"
		if i < height-1 {
			divider += "\n"
		}
	}
	return divider
}

// SetSize updates screen dimensions.
func (s *ConnectorsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

// ShortHelp returns key binding help.
func (s *ConnectorsScreen) ShortHelp() string {
	if s.editing {
		return "enter: save | esc: cancel"
	}
	if s.focusRight {
		return "j/k: navigate | e: edit | V: validate | tab: switch | esc: back"
	}
	return "j/k: navigate | enter: enable | d: disable | V: validate | tab: switch | esc: back"
}

// IsInputMode returns true when capturing text input.
func (s *ConnectorsScreen) IsInputMode() bool {
	return s.editing
}

// Message types for connectors screen.

// OpenConnectorsMsg requests opening the connectors screen.
type OpenConnectorsMsg struct{}

// LoadConnectorsMsg requests loading connectors list.
type LoadConnectorsMsg struct{}

// ConnectorsLoadedMsg carries loaded connectors.
type ConnectorsLoadedMsg struct {
	Connectors []ConnectorStatus
}

// LoadConnectorSettingsMsg requests loading a connector's settings.
type LoadConnectorSettingsMsg struct {
	ConnectorName string
}

// ConnectorSettingsLoadedMsg carries connector settings with values.
type ConnectorSettingsLoadedMsg struct {
	ConnectorName string
	Settings      []ConnectorSettingValue
}

// EnableConnectorMsg requests enabling a connector.
type EnableConnectorMsg struct {
	Name string
}

// DisableConnectorMsg requests disabling all connectors.
type DisableConnectorMsg struct{}

// SaveConnectorSettingMsg requests saving a connector setting.
type SaveConnectorSettingMsg struct {
	ConnectorName string
	Key           string
	Value         string
	IsSecret      bool
}

// ConnectorUpdateResultMsg carries result of connector operations.
type ConnectorUpdateResultMsg struct {
	Success bool
	Error   error
}

// ValidateConnectorMsg requests validating a connector's settings.
type ValidateConnectorMsg struct {
	Name string
}

// ValidateConnectorResultMsg carries the validation result.
type ValidateConnectorResultMsg struct {
	Success bool
	Error   error
}
