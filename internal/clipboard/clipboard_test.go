package clipboard

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEffectiveMode(t *testing.T) {
	uu := map[string]struct {
		env     string
		setting string
		want    string
	}{
		"defaults to auto":     {env: "", setting: "", want: ModeAuto},
		"setting used":         {env: "", setting: "native", want: ModeNative},
		"setting osc52":        {env: "", setting: "osc52", want: ModeOSC52},
		"invalid setting":      {env: "", setting: "toast", want: ModeAuto},
		"env overrides":        {env: "osc52", setting: "native", want: ModeOSC52},
		"env case-insensitive": {env: "NATIVE", setting: "osc52", want: ModeNative},
		"invalid env falls":    {env: "toast", setting: "osc52", want: ModeOSC52},
	}

	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			t.Setenv(clipboardModeEnv, u.env)
			assert.Equal(t, u.want, effectiveMode(u.setting))
		})
	}
}

func TestOSC52MaxEncodedLen(t *testing.T) {
	uu := map[string]struct {
		env string
		e   int
	}{
		"empty":    {env: "", e: defaultOSC52MaxEncodedLen},
		"valid":    {env: "100", e: 100},
		"zero":     {env: "0", e: defaultOSC52MaxEncodedLen},
		"negative": {env: "-1", e: defaultOSC52MaxEncodedLen},
		"invalid":  {env: "abc", e: defaultOSC52MaxEncodedLen},
	}

	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			t.Setenv(osc52MaxEnv, u.env)
			assert.Equal(t, u.e, osc52MaxEncodedLen())
		})
	}
}

func TestOSC52Sequence(t *testing.T) {
	const encoded = "SGVsbG8="

	uu := map[string]struct {
		tmux, screen bool
		e            string
	}{
		"plain":  {e: "\033]52;c;SGVsbG8=\a"},
		"tmux":   {tmux: true, e: "\033Ptmux;\033\033]52;c;SGVsbG8=\a\033\\"},
		"screen": {screen: true, e: "\033P\033]52;c;SGVsbG8=\a\033\\"},
	}

	for k := range uu {
		u := uu[k]
		t.Run(k, func(t *testing.T) {
			assert.Equal(t, u.e, osc52Sequence(encoded, u.tmux, u.screen))
		})
	}
}

func TestWriteEmptyIsNoOp(t *testing.T) {
	require.NoError(t, Writer{}.Write("", ModeNative))
	require.NoError(t, Writer{}.Write("", ModeOSC52))
}

func TestWriteOSC52Unavailable(t *testing.T) {
	// go test's stdout is not a TTY, so writeOSC52 must report unavailable.
	t.Setenv(termEnv, dumbTerm)

	err := writeOSC52("hello")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "osc52 clipboard unavailable")
}
