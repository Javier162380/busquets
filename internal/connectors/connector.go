// Package connectors provides interfaces and types for external channel connectors.
package connectors

import (
	"context"

	domain "github.com/Javier162380/busquets"
)

// SummarySystemPrompt is the instruction template used to generate a plan TLDR.
// Shared by generative summary connectors (e.g. ollama) and the generate_tldr_prompt
// MCP tool, which hands this same template to the calling assistant instead of
// calling a connector, so the output format is identical regardless of who writes it.
const SummarySystemPrompt = "You are a concise technical plan summarizer. " +
	"Output exactly three bullet points covering goals, approach, and key outcomes. " +
	"You must always match the following template:\n\n" +
	"**Goal**: Here the plan goal.\n" +
	"**Approach**: Here the plan approach.\n" +
	"**Outcome**: Here the plan outcome."

// MemoryEventSystemPrompt instructs a model to describe one timeline event in a
// single sentence. Dates, version numbers and change stats are computed by the
// application and must not be restated. Shared by the summary connector and the
// generate_memory_prompt MCP tool so both produce the same shape.
const MemoryEventSystemPrompt = "You are describing one change in the history of a technical plan. " +
	"Write exactly one sentence saying what changed and why it matters. " +
	"Never state dates, times, version numbers, line counts or word counts — " +
	"the application supplies those alongside your sentence. " +
	"Be specific about the substance of the change; do not pad with filler."

// SendResult contains the result of a send/generate operation.
type SendResult struct {
	Success   bool
	MessageID *string // non-nil for messaging connectors (e.g. Telegram)
	Error     error
	Response  *string // non-nil for generative connectors (e.g. Ollama, LM Studio)
	// Truncated reports that the prompt was cut to fit the model's context.
	// Silent truncation would let a summary be written from part of the input
	// with no sign of it, so callers are told and can say so in their output.
	Truncated bool
}

// GeneratorOpts is one generation request. A struct rather than positional
// arguments so knobs (temperature, token limits) can be added later without
// changing every implementation.
type GeneratorOpts struct {
	SystemPrompt string
	UserPrompt   string
}

// GenerativeConnector is implemented by connectors that can answer an arbitrary
// prompt rather than only the fixed TLDR template. Messaging connectors such as
// Telegram deliberately do not implement it.
//
//go:generate mockgen -package connectors_test -destination ./test/generative_stub.go . GenerativeConnector
type GenerativeConnector interface {
	Connector
	Generate(ctx context.Context, opts GeneratorOpts) (*SendResult, error)
}

// Connector defines the interface for external channel connectors.
//
//go:generate mockgen -package connectors_test -destination ./test/connector_stub.go . Connector
type Connector interface {
	// Name returns the unique identifier for this connector.
	Name() string

	// DisplayName returns a human-readable name for UI display.
	DisplayName() string

	// Send transmits content to the external channel.
	Send(ctx context.Context, title, content string) (*SendResult, error)

	// Validate checks if the connector is properly configured.
	Validate() error

	// RequiredSettings returns the list of setting keys this connector needs.
	RequiredSettings() []SettingDefinition
}

// SettingGetter provides access to connector settings.
type SettingGetter interface {
	GetConnectorSetting(ctx context.Context, connectorName, key string) (string, bool, error)
}

// ConfigurableConnector can load config from a setting getter.
type ConfigurableConnector interface {
	Connector
	LoadConfig(ctx context.Context, getter SettingGetter) error
}

// SettingDefinition describes a configuration setting for a connector.
type SettingDefinition struct {
	Key         string
	DisplayName string
	Description string
	Required    bool
	Sensitive   bool
}

// ConnectorStatus represents the status of a connector.
type ConnectorStatus struct {
	Name        string
	DisplayName string
	Role        *domain.ConnectorRole // nil if not assigned to any slot
	Configured  bool
}
