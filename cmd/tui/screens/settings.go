package screens

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/Javier162380/busquets/cmd/tui/messages"
	"github.com/Javier162380/busquets/cmd/tui/styles"
	"github.com/Javier162380/busquets/services/busquets"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type SettingDefinition struct {
	Name          string
	Description   string
	Type          string
	Default       busquets.SettingValues
	AllowedValues []string // non-nil: cycle through these on Enter/Space (string settings)
}

var KnownSettings = []SettingDefinition{
	{
		Name:        busquets.SettingReadingSpeedWPM,
		Description: "Words per minute for reading time estimates",
		Type:        busquets.SettingTypeNumber,
		Default:     busquets.SettingValues{NumberValue: new(float64(200))},
	},
	{
		Name:        busquets.SettingDarkModeEnabled,
		Description: "Enable dark mode theme",
		Type:        busquets.SettingTypeBoolean,
		Default:     busquets.SettingValues{BooleanValue: new(true)},
	},
	{
		Name:        busquets.SettingRenderMarkdownByDefault,
		Description: "Automatically render markdown by default, using the selected theme",
		Type:        busquets.SettingTypeBoolean,
		Default:     busquets.SettingValues{BooleanValue: new(false)},
	},
	{
		Name:        busquets.SettingWatchModeEnabled,
		Description: "Automatically sync plans in the background",
		Type:        busquets.SettingTypeBoolean,
		Default:     busquets.SettingValues{BooleanValue: new(false)},
	},
	{
		Name:        busquets.SettingWatchIntervalSeconds,
		Description: "Watch mode sync interval (seconds)",
		Type:        busquets.SettingTypeNumber,
		Default:     busquets.SettingValues{NumberValue: new(busquets.DefaultWatchIntervalSeconds)},
	},
	{
		Name:          busquets.SettingDefaultDisplayMode,
		Description:   "Default plans screen layout",
		Type:          busquets.SettingTypeString,
		AllowedValues: []string{busquets.DisplayModePlanContent, busquets.DisplayModeTagPlanContent, busquets.DisplayModeLabelPlanContent},
	},
	{
		Name:          busquets.SettingPlansSortKey,
		Description:   "Sort plans by field",
		Type:          busquets.SettingTypeString,
		AllowedValues: []string{busquets.SortKeyUpdatedAt, busquets.SortKeyCreatedAt, busquets.SortKeyReadingTime, busquets.SortKeySize},
	},
	{
		Name:          busquets.SettingPlansSortDir,
		Description:   "Sort direction",
		Type:          busquets.SettingTypeString,
		AllowedValues: []string{busquets.SortDirDesc, busquets.SortDirAsc},
	},
	{
		Name:          busquets.SettingSearchOver,
		Description:   "Search over: plan name, content, or both",
		Type:          busquets.SettingTypeString,
		Default:       busquets.SettingValues{StringValue: new(string(busquets.DefaultSearchOver))},
		AllowedValues: []string{string(busquets.SearchOverAll), string(busquets.SearchOverPlanName), string(busquets.SearchOverContent)},
	},
	{
		Name:          busquets.SettingClipboardMode,
		Description:   "Clipboard: auto (native+OSC52), native, or osc52",
		Type:          busquets.SettingTypeString,
		Default:       busquets.SettingValues{StringValue: new(busquets.DefaultClipboardMode)},
		AllowedValues: []string{busquets.ClipboardModeAuto, busquets.ClipboardModeNative, busquets.ClipboardModeOSC52},
	},
	{
		Name:        busquets.SettingMarkdownTheme,
		Description: "Markdown rendering theme (only visible while markdown rendering is on)",
		Type:        busquets.SettingTypeString,
		Default:     busquets.SettingValues{StringValue: new(busquets.DefaultMarkdownTheme)},
		// MarkdownThemeNoTTYStyle is omitted: glamour renders it identically to
		// MarkdownThemeASCII, so offering both is a dead step in the cycle.
		AllowedValues: []string{busquets.MarkdownThemeDark, busquets.MarkdownThemeLight, busquets.MarkdownThemeTokyoNight, busquets.MarkdownThemeASCII, busquets.MarkdownThemeDracula, busquets.MarkdownThemePinkStyle},
	},
	{
		Name:          busquets.SettingsScreenOrientation,
		Description:   "Screen orientation, default: Horizontal",
		Type:          busquets.SettingTypeString,
		Default:       busquets.SettingValues{StringValue: new(busquets.DefaultScreenOrientation)},
		AllowedValues: []string{busquets.ScreenOrientationHorizontal, busquets.ScreenOrientationVertical},
	},
}

type SettingItem struct {
	Definition SettingDefinition
	Value      busquets.SettingValues
	Editing    bool
	EditBuffer string
}

type SettingsScreen struct {
	settings []SettingItem
	cursor   int
	editing  bool

	width  int
	height int

	borderStyle lipgloss.Style
}

func NewSettingsScreen(width, height int) *SettingsScreen {
	items := make([]SettingItem, len(KnownSettings))
	for i, def := range KnownSettings {
		items[i] = SettingItem{
			Definition: def,
			Value:      def.Default,
		}
	}

	return &SettingsScreen{
		settings: items,
		width:    width,
		height:   height,
		borderStyle: lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.BorderColor),
	}
}

func (s *SettingsScreen) Init() tea.Cmd {
	names := make([]string, len(KnownSettings))
	for i, def := range KnownSettings {
		names[i] = def.Name
	}
	return func() tea.Msg {
		return messages.LoadSettingsMsg{SettingNames: names}
	}
}

func (s *SettingsScreen) Update(msg tea.Msg) (Screen, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return s.handleKey(msg)

	case messages.SettingsLoadedMsg:
		for i := range s.settings {
			name := s.settings[i].Definition.Name
			if setting, ok := msg.Settings[name]; ok {
				s.settings[i].Value = s.settingToValues(setting)
			}
		}
		return s, nil

	case messages.SettingUpdateResultMsg:
		if msg.Error != nil {
			return s, nil
		}
		s.editing = false
		s.settings[s.cursor].Editing = false

		switch s.settings[s.cursor].Definition.Name {
		case busquets.SettingDarkModeEnabled:
			newVal := s.settings[s.cursor].Value.BooleanValue != nil && *s.settings[s.cursor].Value.BooleanValue
			return s, func() tea.Msg { return messages.ThemeChangedMsg{DarkMode: newVal} }
		case busquets.SettingRenderMarkdownByDefault:
			newVal := s.settings[s.cursor].Value.BooleanValue != nil && *s.settings[s.cursor].Value.BooleanValue
			return s, func() tea.Msg { return messages.RenderMarkDownByDefaultMsg{Enabled: newVal} }
		case busquets.SettingDefaultDisplayMode:
			newVal := ""
			if s.settings[s.cursor].Value.StringValue != nil {
				newVal = *s.settings[s.cursor].Value.StringValue
			}
			return s, func() tea.Msg { return messages.DisplayModeChangedMsg{Mode: newVal} }
		case busquets.SettingPlansSortKey:
			newVal := busquets.DefaultPlansSortKey
			if s.settings[s.cursor].Value.StringValue != nil {
				newVal = *s.settings[s.cursor].Value.StringValue
			}
			return s, func() tea.Msg { return messages.PlansSortKeyChangedMsg{SortKey: newVal} }
		case busquets.SettingPlansSortDir:
			newVal := busquets.DefaultSortDir
			if s.settings[s.cursor].Value.StringValue != nil {
				newVal = *s.settings[s.cursor].Value.StringValue
			}
			return s, func() tea.Msg { return messages.PlansSortDirChangedMsg{SortDir: newVal} }
		case busquets.SettingMarkdownTheme:
			newVal := busquets.DefaultMarkdownTheme
			if s.settings[s.cursor].Value.StringValue != nil {
				newVal = *s.settings[s.cursor].Value.StringValue
			}
			return s, func() tea.Msg { return messages.MarkdownRenderedThemeChangedMsg{Theme: newVal} }
		case busquets.SettingsScreenOrientation:
			newVal := busquets.DefaultScreenOrientation
			if s.settings[s.cursor].Value.StringValue != nil {
				newVal = *s.settings[s.cursor].Value.StringValue
			}
			return s, func() tea.Msg { return messages.ScreenOrientationChangedMsg{Orientation: newVal} }
		}

		return s, nil
	}

	return s, nil
}

func (s *SettingsScreen) settingToValues(setting busquets.Setting) busquets.SettingValues {
	var values busquets.SettingValues
	switch {
	case setting.IsNumber():
		v := setting.GetNumberValue()
		values.NumberValue = &v
	case setting.IsBoolean():
		v := setting.GetBooleanValue()
		values.BooleanValue = &v
	case setting.IsString():
		v := setting.GetStringValue()
		values.StringValue = &v
	case setting.IsDate():
		v := setting.GetDateValue()
		values.DateTimeValue = &v
	}
	return values
}

func (s *SettingsScreen) handleKey(msg tea.KeyMsg) (Screen, tea.Cmd) {
	key := msg.String()

	if s.editing {
		return s.handleEditKey(key)
	}

	switch key {
	case "esc":
		return s, func() tea.Msg {
			return messages.PopScreenMsg{}
		}

	case "j", "down":
		if s.cursor < len(s.settings)-1 {
			s.cursor++
		}
		return s, nil

	case "k", "up":
		if s.cursor > 0 {
			s.cursor--
		}
		return s, nil

	case "enter", " ":
		setting := &s.settings[s.cursor]
		switch setting.Definition.Type {
		case busquets.SettingTypeBoolean:
			current := false
			if setting.Value.BooleanValue != nil {
				current = *setting.Value.BooleanValue
			}
			newVal := !current
			setting.Value.BooleanValue = &newVal
			return s, s.saveSetting(setting.Definition.Name, setting.Value)

		case busquets.SettingTypeNumber:
			s.editing = true
			setting.Editing = true
			if setting.Value.NumberValue != nil {
				setting.EditBuffer = fmt.Sprintf("%.0f", *setting.Value.NumberValue)
			} else {
				setting.EditBuffer = ""
			}

		case busquets.SettingTypeString:
			if len(setting.Definition.AllowedValues) > 0 {
				current := ""
				if setting.Value.StringValue != nil {
					current = *setting.Value.StringValue
				}
				next := setting.Definition.AllowedValues[0]
				for i, v := range setting.Definition.AllowedValues {
					if v == current {
						next = setting.Definition.AllowedValues[(i+1)%len(setting.Definition.AllowedValues)]
						break
					}
				}
				setting.Value.StringValue = &next
				return s, s.saveSetting(setting.Definition.Name, setting.Value)
			}
		}
		return s, nil
	}

	return s, nil
}

func (s *SettingsScreen) handleEditKey(key string) (Screen, tea.Cmd) {
	setting := &s.settings[s.cursor]

	switch key {
	case "esc":
		s.editing = false
		setting.Editing = false

	case "enter":
		val, err := strconv.ParseFloat(setting.EditBuffer, 64)
		if err != nil || val <= 0 {
			s.editing = false
			setting.Editing = false
			return s, nil
		}
		setting.Value.NumberValue = &val
		return s, s.saveSetting(setting.Definition.Name, setting.Value)

	case "backspace":
		if len(setting.EditBuffer) > 0 {
			setting.EditBuffer = setting.EditBuffer[:len(setting.EditBuffer)-1]
		}

	default:
		if len(key) == 1 && key[0] >= '0' && key[0] <= '9' {
			setting.EditBuffer += key
		}
	}

	return s, nil
}

func (s *SettingsScreen) View() string {
	contentHeight := s.height - 4

	var sb strings.Builder
	sb.WriteString(styles.TitleStyle.Render("Settings") + "\n\n")

	for i, setting := range s.settings {
		sb.WriteString(s.renderSettingItem(i, &setting) + "\n\n")
	}

	sb.WriteString("\n" + styles.MutedStyle.Render("Enter/Space: edit | Esc: back"))

	return s.borderStyle.
		Width(s.width).
		Height(contentHeight).
		Padding(2, 4).
		Render(sb.String())
}

func (s *SettingsScreen) renderSettingItem(index int, setting *SettingItem) string {
	selected := index == s.cursor

	nameStyle := styles.MutedStyle
	if selected {
		nameStyle = styles.AccentStyle.Bold(true)
	}
	name := nameStyle.Render(setting.Definition.Name)

	var value string
	switch setting.Definition.Type {
	case busquets.SettingTypeBoolean:
		if setting.Value.BooleanValue != nil && *setting.Value.BooleanValue {
			value = styles.SuccessStyle.Render("[ON]")
		} else {
			value = styles.InactiveStyle.Render("[OFF]")
		}
	case busquets.SettingTypeNumber:
		if setting.Editing {
			value = styles.AccentStyle.Render(fmt.Sprintf("> %s_", setting.EditBuffer))
		} else if setting.Value.NumberValue != nil {
			value = lipgloss.NewStyle().Foreground(styles.ForegroundColor).Render(fmt.Sprintf("%.0f", *setting.Value.NumberValue))
		}
	case busquets.SettingTypeString:
		if setting.Value.StringValue != nil {
			value = lipgloss.NewStyle().Foreground(styles.ForegroundColor).Render("[" + *setting.Value.StringValue + "]")
		} else if len(setting.Definition.AllowedValues) > 0 {
			value = lipgloss.NewStyle().Foreground(styles.ForegroundColor).Render("[" + setting.Definition.AllowedValues[0] + "]")
		}
	}

	desc := styles.MutedStyle.Render(setting.Definition.Description)

	cursor := "  "
	if selected {
		cursor = styles.AccentStyle.Render("> ")
	}

	return fmt.Sprintf("%s%s: %s\n   %s", cursor, name, value, desc)
}

func (s *SettingsScreen) SetSize(width, height int) {
	s.width = width
	s.height = height
}

func (s *SettingsScreen) ShortHelp() string {
	if s.editing {
		return "0-9: type | backspace: delete | enter: save | esc: cancel"
	}
	return "down/up: navigate | enter/space: edit | esc: back"
}

// IsInputMode returns true when capturing text input.
func (s *SettingsScreen) IsInputMode() bool { return s.editing }

// EditorMode ....
func (s *SettingsScreen) EditorMode() bool { return false }

func (s *SettingsScreen) saveSetting(name string, values busquets.SettingValues) tea.Cmd {
	return func() tea.Msg {
		return messages.SaveSettingMsg{
			Name:   name,
			Values: values,
		}
	}
}
