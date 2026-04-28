// Package telegram provides a Telegram connector implementation.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Javier162380/claude-plan-viewer/internal/connectors"
)

const (
	telegramAPIURL  = "https://api.telegram.org/bot%s/sendMessage"
	settingBotToken = "bot_token"
	settingChatID   = "chat_id"
	maxMessageLen   = 4000
)

// Config holds Telegram-specific configuration.
type Config struct {
	BotToken string
	ChatID   string
}

// Connector implements the Telegram connector.
type Connector struct {
	config Config
	client *http.Client
}

// New creates a new Telegram connector.
func New() *Connector {
	return &Connector{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// Name returns the connector identifier.
func (c *Connector) Name() string {
	return "telegram"
}

// DisplayName returns a human-readable name.
func (c *Connector) DisplayName() string {
	return "Telegram"
}

// RequiredSettings returns the settings this connector needs.
func (c *Connector) RequiredSettings() []connectors.SettingDefinition {
	return []connectors.SettingDefinition{
		{
			Key:         settingBotToken,
			DisplayName: "Bot Token",
			Description: "Telegram Bot API token from @BotFather",
			Required:    true,
			Sensitive:   true,
		},
		{
			Key:         settingChatID,
			DisplayName: "Chat ID",
			Description: "Target chat or channel ID",
			Required:    true,
			Sensitive:   false,
		},
	}
}

// LoadConfig loads configuration from the setting getter.
func (c *Connector) LoadConfig(ctx context.Context, getter connectors.SettingGetter) error {
	token, _, err := getter.GetConnectorSetting(ctx, c.Name(), settingBotToken)
	if err != nil {
		return fmt.Errorf("failed to get bot token: %w", err)
	}

	chatID, _, err := getter.GetConnectorSetting(ctx, c.Name(), settingChatID)
	if err != nil {
		return fmt.Errorf("failed to get chat ID: %w", err)
	}

	c.config = Config{
		BotToken: token,
		ChatID:   chatID,
	}
	return nil
}

// Validate checks if the connector is properly configured.
func (c *Connector) Validate() error {
	if c.config.BotToken == "" {
		return fmt.Errorf("bot token not configured")
	}
	if c.config.ChatID == "" {
		return fmt.Errorf("chat ID not configured")
	}
	return nil
}

// Send transmits content to Telegram.
func (c *Connector) Send(ctx context.Context, title, content string) (*connectors.SendResult, error) {
	// Format message with title
	message := fmt.Sprintf("*%s*\n\n%s", escapeMarkdown(title), escapeMarkdown(content))

	// Truncate if necessary (Telegram has 4096 char limit)
	if len(message) > maxMessageLen {
		message = message[:maxMessageLen-9] + "\\.\\.\\."
	}

	payload := map[string]interface{}{
		"chat_id":    c.config.ChatID,
		"text":       message,
		"parse_mode": "MarkdownV2",
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	url := fmt.Sprintf(telegramAPIURL, c.config.BotToken)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	var result telegramResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.OK {
		return &connectors.SendResult{
			Success: false,
			Error:   fmt.Errorf("telegram API error: %s", result.Description),
		}, nil
	}

	return &connectors.SendResult{
		Success:   true,
		MessageID: fmt.Sprintf("%d", result.Result.MessageID),
	}, nil
}

type telegramResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description,omitempty"`
	Result      struct {
		MessageID int64 `json:"message_id"` //nolint:tagliatelle // Telegram API uses snake_case
	} `json:"result,omitempty"`
}

// escapeMarkdown escapes special characters for Telegram MarkdownV2.
func escapeMarkdown(text string) string {
	replacer := strings.NewReplacer(
		"_", "\\_",
		"*", "\\*",
		"[", "\\[",
		"]", "\\]",
		"(", "\\(",
		")", "\\)",
		"~", "\\~",
		"`", "\\`",
		">", "\\>",
		"#", "\\#",
		"+", "\\+",
		"-", "\\-",
		"=", "\\=",
		"|", "\\|",
		"{", "\\{",
		"}", "\\}",
		".", "\\.",
		"!", "\\!",
	)
	return replacer.Replace(text)
}
