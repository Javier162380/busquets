// Package ollama provides a local LLM connector for generating plan summaries.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Javier162380/busquets/internal/connectors"
)

const (
	settingHost  = "host"
	settingModel = "model"

	maxContentRunes = 12000
	httpTimeout     = 60 * time.Second
)

// Connector implements the Ollama local LLM connector.
type Connector struct {
	host   string
	model  string
	client *http.Client
}

// New creates a new Ollama connector.
func New() *Connector {
	return &Connector{
		client: &http.Client{Timeout: httpTimeout},
	}
}

// Name returns the connector identifier.
func (c *Connector) Name() string { return "ollama" }

// DisplayName returns a human-readable name.
func (c *Connector) DisplayName() string { return "Ollama (Local LLM)" }

// RequiredSettings returns the settings this connector needs.
func (c *Connector) RequiredSettings() []connectors.SettingDefinition {
	return []connectors.SettingDefinition{
		{
			Key:         settingHost,
			DisplayName: "Host",
			Description: "Ollama server URL (e.g. http://localhost:11434)",
			Required:    true,
			Sensitive:   false,
		},
		{
			Key:         settingModel,
			DisplayName: "Model",
			Description: "Model name to use for summarization (e.g. llama3)",
			Required:    true,
			Sensitive:   false,
		},
	}
}

// LoadConfig loads configuration from the setting getter.
func (c *Connector) LoadConfig(ctx context.Context, getter connectors.SettingGetter) error {
	host, _, err := getter.GetConnectorSetting(ctx, c.Name(), settingHost)
	if err != nil {
		return fmt.Errorf("failed to get host: %w", err)
	}

	model, _, err := getter.GetConnectorSetting(ctx, c.Name(), settingModel)
	if err != nil {
		return fmt.Errorf("failed to get model: %w", err)
	}

	c.host = host
	c.model = model
	return nil
}

// Validate checks if the connector is properly configured.
func (c *Connector) Validate() error {
	if c.host == "" {
		return fmt.Errorf("ollama host not configured")
	}
	if c.model == "" {
		return fmt.Errorf("ollama model not configured")
	}
	return nil
}

// OllamaGenerateRequest is the request body for POST /api/generate.
type OllamaGenerateRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	System string `json:"system,omitempty"`
	Stream bool   `json:"stream"`
}

// OllamaGenerateResponse is the response body from POST /api/generate (stream=false).
type OllamaGenerateResponse struct {
	Model              string `json:"model"`
	CreatedAt          string `json:"created_at"` //nolint:tagliatelle//Match OllamaAPI.
	Response           string `json:"response"`
	Done               bool   `json:"done"`
	TotalDuration      int64  `json:"total_duration"`       //nolint:gci,tagliatelle//Match OllamaAPI.
	LoadDuration       int64  `json:"load_duration"`        //nolint:tagliatelle//Match OllamaAPI.
	PromptEvalCount    int    `json:"prompt_eval_count"`    //nolint:tagliatelle//Match OllamaAPI.
	PromptEvalDuration int64  `json:"prompt_eval_duration"` //nolint:tagliatelle//Match OllamaAPI.
	EvalCount          int    `json:"eval_count"`           //nolint:tagliatelle//Match OllamaAPI.
	EvalDuration       int64  `json:"eval_duration"`        //nolint:tagliatelle//Match OllamaAPI.
}

// truncateAtSentence trims content to at most maxRunes runes, preferring to cut
// at the last sentence boundary ('. ', '! ', '? ', or newline) within the final
// 200 runes so the prompt ends at a clean point.
func truncateAtSentence(content string, maxRunes int) string {
	runes := []rune(content)
	if len(runes) <= maxRunes {
		return content
	}
	for i := maxRunes - 1; i >= maxRunes-200 && i >= 0; i-- {
		r := runes[i]
		if (r == '.' || r == '!' || r == '?') && i+1 < len(runes) {
			next := runes[i+1]
			if next == ' ' || next == '\n' {
				return string(runes[:i+1])
			}
		}
	}
	return string(runes[:maxRunes])
}

// Send generates a summary of the plan content using the Ollama API.
func (c *Connector) Send(ctx context.Context, title, content string) (*connectors.SendResult, error) {
	content = truncateAtSentence(content, maxContentRunes)

	reqBody := OllamaGenerateRequest{
		Model:  c.model,
		Prompt: fmt.Sprintf("Summarize this plan:\n\nTitle: %s\n\n%s", title, content),
		System: connectors.SummarySystemPrompt,
		Stream: false,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf("%s/api/generate", c.host)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call ollama: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	var result OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode ollama response: %w", err)
	}

	if result.Response == "" {
		return nil, fmt.Errorf("ollama returned empty response")
	}

	return &connectors.SendResult{
		Success:  true,
		Response: &result.Response,
	}, nil
}
