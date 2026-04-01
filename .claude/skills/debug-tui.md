# Debugging TUI Sessions

This skill explains how to debug the BubbleTea TUI application by capturing all messages to a log file.

## Quick Start

```bash
# Run TUI in debug mode
make tui-debug

# Or manually
DEBUG=1 ./bin/plan-viewer tui
```

Debug messages are written to: `~/.claude-viewer/tui-debug.log`

## How It Works

When `DEBUG=1` is set, the TUI dumps every `tea.Msg` to the log file using `go-spew`.

**Session markers**: Each session appends to the log with a separator:
```
========== SESSION START: 2026-04-01 10:30:45 ==========
```

This captures:

- **Window events**: `tea.WindowSizeMsg` with terminal dimensions
- **Key presses**: `tea.KeyMsg` with key name, runes, and modifiers
- **Mouse events**: `tea.MouseMsg` with coordinates and button state
- **Custom messages**: All app-specific messages like `PlansLoadedMsg`, `PlanDetailLoadedMsg`, etc.

## Analyzing the Log

### View in real-time
```bash
tail -f ~/.claude-viewer/tui-debug.log
```

### Search for specific messages
```bash
# Find all key presses
grep "tea.KeyMsg" ~/.claude-viewer/tui-debug.log

# Find errors
grep "ErrorMsg" ~/.claude-viewer/tui-debug.log

# Find plan loads
grep "PlansLoadedMsg" ~/.claude-viewer/tui-debug.log
```

### Compare sessions
```bash
# List all session starts
grep "SESSION START" ~/.claude-viewer/tui-debug.log

# Count sessions
grep -c "SESSION START" ~/.claude-viewer/tui-debug.log

# View messages between two sessions
sed -n '/SESSION START: 2026-04-01 10:30/,/SESSION START/p' ~/.claude-viewer/tui-debug.log
```

### Common Message Types

| Message | Description |
|---------|-------------|
| `tea.WindowSizeMsg` | Terminal resized, contains Width/Height |
| `tea.KeyMsg` | Key pressed, contains Type/Runes/Alt |
| `PlansLoadedMsg` | Plans list fetched from database |
| `PlanDetailLoadedMsg` | Single plan content loaded |
| `LoadPlanDetailMsg` | Request to load a plan (internal) |
| `VersionsLoadedMsg` | Version history loaded |
| `ErrorMsg` | An error occurred |
| `SyncResultMsg` | Sync operation completed |

## Example Log Output

```
(tea.WindowSizeMsg) {
 Width: (int) 188,
 Height: (int) 48
}
(screens.PlansLoadedMsg) {
 Plans: ([]claudeviewer.PlanSummary) (len=42) {
  ...
 }
}
(tea.KeyMsg) {
 Type: (tea.KeyType) 0,
 Runes: ([]int32) (len=1) {
  (int32) 106  // 'j' key
 },
 Alt: (bool) false
}
```

## Implementation Details

The debug feature is implemented in:

- **`cmd/tui/app.go`**: `App.dump` field and `SetDump()` method
- **`cmd/tui/main.go`**: `StartWithOptions(service, debug)` opens log file in append mode
- **`cmd/main.go`**: Reads `DEBUG` env var and passes to `StartWithOptions`

Key code in `main.go`:
```go
// Open in append mode to preserve previous sessions
f, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)

// Write session separator
sessionStart := fmt.Sprintf("\n\n========== SESSION START: %s ==========\n\n",
    time.Now().Format("2006-01-02 15:04:05"))
f.WriteString(sessionStart)
```

Key code in `App.Update()`:
```go
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
    if a.dump != nil {
        spew.Fdump(a.dump, msg)
    }
    // ... rest of update logic
}
```

## Troubleshooting

### Log file not created
- Ensure `~/.claude-viewer/` directory exists (created automatically on first sync/serve)
- Check write permissions

### Log file too large
- The log appends across sessions and grows quickly
- Truncate when needed: `> ~/.claude-viewer/tui-debug.log`
- Or delete and start fresh: `rm ~/.claude-viewer/tui-debug.log`

### Missing messages
- Some messages may be filtered before reaching `Update()`
- Check if the message is being sent via `tea.Cmd`
