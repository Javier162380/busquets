package screens

import (
	"testing"
	"time"

	claudeviewer "github.com/Javier162380/claude-plan-viewer/services/claude-viewer"
	"github.com/stretchr/testify/require"
)

func TestSettingToValues(t *testing.T) {
	s := NewSettingsScreen(80, 24)

	t.Run("boolean setting maps to BooleanValue", func(t *testing.T) {
		v := s.settingToValues(claudeviewer.Setting{BoolValue: new(true)})
		require.NotNil(t, v.BooleanValue)
		require.True(t, *v.BooleanValue)
	})

	t.Run("string setting maps to StringValue", func(t *testing.T) {
		v := s.settingToValues(claudeviewer.Setting{StringValue: new("dark")})
		require.NotNil(t, v.StringValue)
		require.Equal(t, "dark", *v.StringValue)
	})

	t.Run("number setting maps to NumberValue", func(t *testing.T) {
		v := s.settingToValues(claudeviewer.Setting{NumberValue: new(200.0)})
		require.NotNil(t, v.NumberValue)
		require.Equal(t, 200.0, *v.NumberValue)
	})

	t.Run("date setting maps to DateTimeValue", func(t *testing.T) {
		now := time.Now()
		v := s.settingToValues(claudeviewer.Setting{DateValue: &now})
		require.NotNil(t, v.DateTimeValue)
		require.Equal(t, now, *v.DateTimeValue)
	})
}

func TestSettingsScreenIsInputMode(t *testing.T) {
	t.Run("false by default", func(t *testing.T) {
		s := NewSettingsScreen(80, 24)
		require.False(t, s.IsInputMode())
	})

	t.Run("true when editing is set", func(t *testing.T) {
		s := NewSettingsScreen(80, 24)
		s.editing = true
		require.True(t, s.IsInputMode())
	})
}

func TestSettingsScreenShortHelp(t *testing.T) {
	t.Run("returns navigation hint in normal mode", func(t *testing.T) {
		s := NewSettingsScreen(80, 24)
		require.Contains(t, s.ShortHelp(), "navigate")
	})

	t.Run("returns edit hint in input mode", func(t *testing.T) {
		s := NewSettingsScreen(80, 24)
		s.editing = true
		require.Contains(t, s.ShortHelp(), "save")
	})
}
