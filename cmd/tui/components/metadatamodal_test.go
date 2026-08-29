package components

import (
	"testing"

	"github.com/Javier162380/busquets/cmd/tui/messages"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/require"
)

func TestMetadataModal(t *testing.T) {
	rows := []MetadataRow{
		{Label: "Source Path", Value: "/src/plan.md", Copyable: true},
		{Label: "Destination Path", Value: "/dst/plan.md", Copyable: true},
		{Label: "Size in Bytes", Value: "4213"},
	}

	tests := []struct {
		name            string
		keys            []string
		wantSelectedIdx int
		wantActive      bool
		wantCopyMsg     *messages.CopyToClipboardMsg
	}{
		{
			name:            "down moves selection to the next row",
			keys:            []string{"down"},
			wantSelectedIdx: 1,
			wantActive:      true,
		},
		{
			name:            "down clamps at the last row",
			keys:            []string{"down", "down", "down", "down"},
			wantSelectedIdx: 2,
			wantActive:      true,
		},
		{
			name:            "up clamps at the first row",
			keys:            []string{"up"},
			wantSelectedIdx: 0,
			wantActive:      true,
		},
		{
			name:            "c on a copyable row emits CopyToClipboardMsg",
			keys:            []string{"c"},
			wantSelectedIdx: 0,
			wantActive:      true,
			wantCopyMsg:     &messages.CopyToClipboardMsg{Text: "/src/plan.md", Label: "Source Path"},
		},
		{
			name:            "c on the second copyable row copies that row",
			keys:            []string{"down", "c"},
			wantSelectedIdx: 1,
			wantActive:      true,
			wantCopyMsg:     &messages.CopyToClipboardMsg{Text: "/dst/plan.md", Label: "Destination Path"},
		},
		{
			name:            "c on a non-copyable row is a no-op",
			keys:            []string{"down", "down", "c"},
			wantSelectedIdx: 2,
			wantActive:      true,
			wantCopyMsg:     nil,
		},
		{
			name:            "esc closes without emitting",
			keys:            []string{"esc"},
			wantSelectedIdx: 0,
			wantActive:      false,
			wantCopyMsg:     nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewMetadataModal()
			m.SetSize(60, 20)
			m.Open("Plan Metadata", rows)

			var lastCmd tea.Cmd
			for _, key := range tt.keys {
				var keyMsg tea.KeyMsg
				if key == "esc" {
					keyMsg = tea.KeyMsg{Type: tea.KeyEsc}
				} else {
					keyMsg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
					switch key {
					case "up":
						keyMsg = tea.KeyMsg{Type: tea.KeyUp}
					case "down":
						keyMsg = tea.KeyMsg{Type: tea.KeyDown}
					}
				}
				lastCmd = m.Update(keyMsg)
			}

			require.Equal(t, tt.wantSelectedIdx, m.selectedIdx)
			require.Equal(t, tt.wantActive, m.IsActive())

			if tt.wantCopyMsg == nil {
				require.Nil(t, lastCmd)
				return
			}

			require.NotNil(t, lastCmd)
			msg, ok := lastCmd().(messages.CopyToClipboardMsg)
			require.True(t, ok)
			require.Equal(t, *tt.wantCopyMsg, msg)
		})
	}

	t.Run("IsActive is false before Open", func(t *testing.T) {
		m := NewMetadataModal()
		require.False(t, m.IsActive())
	})

	t.Run("down is a no-op with a single row", func(t *testing.T) {
		m := NewMetadataModal()
		m.SetSize(60, 20)
		m.Open("Version Metadata", []MetadataRow{{Label: "Source Path", Value: "/v1.md", Copyable: true}})
		m.Update(tea.KeyMsg{Type: tea.KeyDown})
		require.Equal(t, 0, m.selectedIdx)
		require.Equal(t, true, m.IsActive())
	})
}
