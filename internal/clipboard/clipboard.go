// Package clipboard writes text to the system clipboard, preferring the native
// clipboard and falling back to an OSC52 terminal escape sequence so copy still
// works over SSH/tmux/screen where no native clipboard tool is available.
//
// The OSC52 fallback is adapted from k9s (github.com/derailed/k9s,
// internal/view/clipboard.go).
package clipboard

import (
	"encoding/base64"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/atotto/clipboard"
)

// Clipboard mode values. Mode selects how a write reaches the clipboard.
const (
	ModeAuto   = "auto"   // native, falling back to OSC52
	ModeNative = "native" // native clipboard tool only
	ModeOSC52  = "osc52"  // OSC52 terminal escape sequence only
)

const (
	// clipboardModeEnv overrides the caller-supplied mode for the current session.
	clipboardModeEnv = "BUSQUETS_CLIPBOARD"
	// osc52MaxEnv caps the base64-encoded OSC52 payload size.
	osc52MaxEnv = "BUSQUETS_OSC52_MAX"
	termEnv     = "TERM"
	tmuxEnv     = "TMUX"

	dumbTerm         = "dumb"
	screenTermPrefix = "screen"

	// defaultOSC52MaxEncodedLen is the conservative default payload ceiling many
	// terminals accept for an OSC52 sequence.
	defaultOSC52MaxEncodedLen = 74994
)

// Clipboard writes text to the system clipboard using the given mode
// ("auto"/"native"/"osc52"). It is implemented by SystemClipboard and faked in
// tests, mirroring the injectable-dependency style of nowprovider.NowProvider.
type Clipboard interface {
	Write(text, mode string) error
}

//go:generate mockgen -package clipboard_test -destination ./test/clipboard_stub.go . Clipboard

// SystemClipboard writes to the native clipboard, falling back to OSC52. It is
// safe for concurrent use.
type SystemClipboard struct{}

// Write copies text to the clipboard using mode ("auto"/"native"/"osc52"). An
// empty text is a no-op. The BUSQUETS_CLIPBOARD env var, when set to a valid
// mode, overrides the mode argument.
func (SystemClipboard) Write(text, mode string) error {
	if text == "" {
		return nil
	}

	switch effectiveMode(mode) {
	case ModeNative:
		return clipboard.WriteAll(text)
	case ModeOSC52:
		return writeOSC52(text)
	default: // ModeAuto: try native, fall back to OSC52.
		if err := clipboard.WriteAll(text); err == nil {
			return nil
		}
		return writeOSC52(text)
	}
}

// effectiveMode resolves the mode actually used: a valid BUSQUETS_CLIPBOARD
// env value wins, otherwise the caller-supplied setting, otherwise auto.
func effectiveMode(setting string) string {
	if m := normalizeMode(os.Getenv(clipboardModeEnv)); m != "" {
		return m
	}
	if m := normalizeMode(setting); m != "" {
		return m
	}
	return ModeAuto
}

// normalizeMode returns the canonical mode for s, or "" if s is not a known mode.
func normalizeMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case ModeAuto:
		return ModeAuto
	case ModeNative:
		return ModeNative
	case ModeOSC52:
		return ModeOSC52
	default:
		return ""
	}
}

func canTryOSC52() bool {
	if !isTTY(os.Stdout) {
		return false
	}
	return termValue() != dumbTerm
}

func termValue() string {
	return strings.ToLower(strings.TrimSpace(os.Getenv(termEnv)))
}

func isTTY(f *os.File) bool {
	if f == nil {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func writeOSC52(text string) error {
	if !canTryOSC52() {
		return fmt.Errorf("osc52 clipboard unavailable: stdout is not a tty or TERM=dumb")
	}

	encoded := base64.StdEncoding.EncodeToString([]byte(text))
	maxLen := osc52MaxEncodedLen()
	if len(encoded) > maxLen {
		return fmt.Errorf("osc52 payload exceeds encoded size limit (%d > %d)", len(encoded), maxLen)
	}

	term := termValue()
	seq := osc52Sequence(encoded, os.Getenv(tmuxEnv) != "", strings.HasPrefix(term, screenTermPrefix))
	_, err := os.Stdout.WriteString(seq)

	return err
}

func osc52MaxEncodedLen() int {
	v := strings.TrimSpace(os.Getenv(osc52MaxEnv))
	if v == "" {
		return defaultOSC52MaxEncodedLen
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return defaultOSC52MaxEncodedLen
	}
	return n
}

func osc52Sequence(encoded string, tmux, screen bool) string {
	switch {
	case tmux:
		// tmux DCS passthrough — requires allow-passthrough in tmux.conf.
		return "\033Ptmux;\033\033]52;c;" + encoded + "\a\033\\"
	case screen:
		// GNU screen DCS passthrough.
		return "\033P\033]52;c;" + encoded + "\a\033\\"
	default:
		return "\033]52;c;" + encoded + "\a"
	}
}
