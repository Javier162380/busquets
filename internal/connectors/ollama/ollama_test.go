package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Javier162380/busquets/internal/connectors"

	"github.com/stretchr/testify/require"
)

// settingGetter is a stub SettingGetter backed by a map.
type settingGetter map[string]string

func (s settingGetter) GetConnectorSetting(_ context.Context, _, key string) (string, bool, error) {
	value, ok := s[key]
	return value, ok, nil
}

// fakeOllama serves /api/generate and records the last request it received.
func fakeOllama(t *testing.T, reply string) (*httptest.Server, *OllamaGenerateRequest) {
	t.Helper()
	captured := &OllamaGenerateRequest{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/generate", r.URL.Path)
		require.NoError(t, json.NewDecoder(r.Body).Decode(captured))
		require.NoError(t, json.NewEncoder(w).Encode(OllamaGenerateResponse{Response: reply, Done: true}))
	}))
	t.Cleanup(server.Close)

	return server, captured
}

func configured(t *testing.T, host string, settings settingGetter) *Connector {
	t.Helper()
	settings["host"] = host
	settings["model"] = "llama3"

	connector := New()
	require.NoError(t, connector.LoadConfig(context.Background(), settings))
	require.NoError(t, connector.Validate())
	return connector
}

func TestSend(t *testing.T) {
	t.Run("keeps the TLDR prompt shape", func(t *testing.T) {
		server, captured := fakeOllama(t, "**Goal**: ship it.")
		connector := configured(t, server.URL, settingGetter{})

		result, err := connector.Send(context.Background(), "My Plan", "The body of the plan.")
		require.NoError(t, err)
		require.True(t, result.Success)
		require.Equal(t, "**Goal**: ship it.", *result.Response)
		require.False(t, result.Truncated)

		require.Equal(t, "llama3", captured.Model)
		require.Equal(t, connectors.SummarySystemPrompt, captured.System)
		require.Equal(t, "Summarize this plan:\n\nTitle: My Plan\n\nThe body of the plan.", captured.Prompt)
		require.False(t, captured.Stream)
	})

	t.Run("reports an empty response as an error", func(t *testing.T) {
		server, _ := fakeOllama(t, "")
		connector := configured(t, server.URL, settingGetter{})

		_, err := connector.Send(context.Background(), "My Plan", "body")
		require.Error(t, err)
	})
}

func TestGenerate(t *testing.T) {
	t.Run("passes an arbitrary system prompt through", func(t *testing.T) {
		server, captured := fakeOllama(t, "One sentence.")
		connector := configured(t, server.URL, settingGetter{})

		result, err := connector.Generate(context.Background(), connectors.GeneratorOpts{
			SystemPrompt: connectors.MemoryEventSystemPrompt,
			UserPrompt:   "describe this change",
		})
		require.NoError(t, err)
		require.Equal(t, "One sentence.", *result.Response)
		require.Equal(t, connectors.MemoryEventSystemPrompt, captured.System)
		require.Equal(t, "describe this change", captured.Prompt)
	})

	t.Run("caps the prompt and reports the truncation", func(t *testing.T) {
		server, captured := fakeOllama(t, "summary")
		connector := configured(t, server.URL, settingGetter{"max_content_runes": "50"})
		require.Equal(t, 50, connector.maxContentRunes)

		long := strings.Repeat("word ", 200)
		result, err := connector.Generate(context.Background(), connectors.GeneratorOpts{UserPrompt: long})
		require.NoError(t, err)
		require.True(t, result.Truncated)
		require.LessOrEqual(t, len([]rune(captured.Prompt)), 50)
	})

	t.Run("does not flag a prompt that fits", func(t *testing.T) {
		server, _ := fakeOllama(t, "summary")
		connector := configured(t, server.URL, settingGetter{"max_content_runes": "5000"})

		result, err := connector.Generate(context.Background(), connectors.GeneratorOpts{UserPrompt: "short"})
		require.NoError(t, err)
		require.False(t, result.Truncated)
	})

	t.Run("surfaces a non-200 as an error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		t.Cleanup(server.Close)
		connector := configured(t, server.URL, settingGetter{})

		_, err := connector.Generate(context.Background(), connectors.GeneratorOpts{UserPrompt: "x"})
		require.Error(t, err)
		require.Contains(t, err.Error(), "500")
	})
}

func TestLoadConfig(t *testing.T) {
	ctx := context.Background()

	t.Run("falls back to the default cap", func(t *testing.T) {
		connector := New()
		require.NoError(t, connector.LoadConfig(ctx, settingGetter{"host": "h", "model": "m"}))
		require.Equal(t, defaultMaxContentRunes, connector.maxContentRunes)
	})

	t.Run("ignores an unusable cap rather than failing", func(t *testing.T) {
		for _, value := range []string{"not-a-number", "0", "-5", ""} {
			connector := New()
			require.NoError(t, connector.LoadConfig(ctx, settingGetter{
				"host": "h", "model": "m", "max_content_runes": value,
			}))
			require.Equal(t, defaultMaxContentRunes, connector.maxContentRunes,
				"value %q should fall back to the default", value)
		}
	})

	t.Run("max_content_runes is optional", func(t *testing.T) {
		required := map[string]bool{}
		for _, def := range New().RequiredSettings() {
			required[def.Key] = def.Required
		}
		require.True(t, required["host"])
		require.True(t, required["model"])
		require.False(t, required["max_content_runes"],
			"a required cap would break every existing ollama setup")
	})
}

func TestValidate(t *testing.T) {
	require.Error(t, New().Validate())

	connector := New()
	require.NoError(t, connector.LoadConfig(context.Background(), settingGetter{"host": "h"}))
	require.Error(t, connector.Validate(), "model is still missing")
}
