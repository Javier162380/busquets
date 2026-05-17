// Package ollama provides a local LLM connector for generating plan summaries.
package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/connectors"
)

const (
	settingHost  = "host"
	settingModel = "model"

	maxContentChars = 8000
	httpTimeout     = 60 * time.Second

	systemPrompt = `You are a concise technical plan summarizer. Output 3-5 clear, actionable bullet points covering goals, approach, and key outcomes.
					The response need to contain these three bullet points. Goals, Approach and Outcome. You must match always the following template.
					
					**Goal**: Here the plan goal.
					**Approach**: Here the plan approach.
					**Outcome**: Here the plan outcome.
					`
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

// Send generates a summary of the plan content using the Ollama API.
func (c *Connector) Send(ctx context.Context, title, content string) (*connectors.SendResult, error) {
	if len(content) > maxContentChars {
		content = content[:maxContentChars]
	}

	requestBody := map[string]any{
		"model":  c.model,
		"system": systemPrompt,
		"prompt": fmt.Sprintf("Summarize this plan:\n\nTitle: %s\n\n%s", title, content),
		"stream": false,
	}

	body, err := json.Marshal(requestBody)
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

	var result struct {
		Response string `json:"response"`
	}
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
