package transform

import (
	"fmt"
	"io"

	"ai-proxy/types"
)

// Config holds all parameters needed to build a transformer pipeline.
// It consolidates route information, feature flags, and responses-specific
// options into a single struct that the pipeline builder uses to select
// and configure stages.
//
// @brief Configuration for declarative pipeline construction.
//
// @note Config replaces the scattered Set* calls that handlers previously
//
//	made after constructing transformers (SetKimiToolCallTransform,
//	SetInputItems, etc.).
type Config struct {
	// UpstreamFormat is the format produced by the upstream provider.
	// Values: "openai", "anthropic"
	UpstreamFormat string

	// DownstreamFormat is the format expected by the client.
	// Values: "openai", "anthropic", "responses"
	DownstreamFormat string

	// Tool call extraction from reasoning content.
	KimiToolCallTransform bool
	GLM5ToolCallTransform bool

	// ReasoningSplit enables separate reasoning output.
	ReasoningSplit bool

	// WebSearchEnabled controls whether web search interception is active.
	WebSearchEnabled bool

	// Responses-specific options (only used when DownstreamFormat == "responses").

	// InputItems stores the parsed input items for conversation storage.
	InputItems []types.InputItem

	// Store controls whether to store the conversation.
	Store bool

	// PreviousResponseID is the ID of the previous response for conversation chain.
	PreviousResponseID string

	// ReasoningSummaryMode controls how reasoning is summarized.
	// Values: "" (no summary), "concise", "detailed"
	ReasoningSummaryMode string

	// EncryptedReasoning stores the encrypted reasoning blob from the request.
	EncryptedReasoning string
}

// Validate checks the config for errors.
//
// @brief Validates that required fields are set and combinations are valid.
//
// @return error Returns error for invalid configurations.
func (c Config) Validate() error {
	switch c.UpstreamFormat {
	case "openai", "anthropic":
		// valid
	default:
		return fmt.Errorf("invalid upstream format: %q", c.UpstreamFormat)
	}

	switch c.DownstreamFormat {
	case "openai", "anthropic", "responses":
		// valid
	default:
		return fmt.Errorf("invalid downstream format: %q", c.DownstreamFormat)
	}

	return nil
}

// NeedsToolCallExtraction returns true if any tool call extraction is enabled.
//
// @brief Checks if Kimi or GLM-5 tool call extraction is needed.
func (c Config) NeedsToolCallExtraction() bool {
	return c.KimiToolCallTransform || c.GLM5ToolCallTransform
}

// NeedsWebSearch returns true if web search interception is enabled.
//
// @brief Checks if web search is enabled.
func (c Config) NeedsWebSearch() bool {
	return c.WebSearchEnabled
}

// IsPassthrough returns true if no transformation is needed.
//
// @brief Checks if upstream format matches downstream format with no features enabled.
func (c Config) IsPassthrough() bool {
	if c.UpstreamFormat != c.DownstreamFormat {
		return false
	}
	if c.NeedsToolCallExtraction() {
		return false
	}
	if c.NeedsWebSearch() {
		return false
	}
	return true
}

// SSEWriterStage is a terminal Stage that writes PipelineEvents as SSE text
// to an io.Writer. It serves as the final output stage in any pipeline.
//
// @brief Final pipeline stage that serializes events to SSE text.
type SSEWriterStage struct {
	writer *SSEWriter
}

// NewSSEWriterStage creates a terminal output stage.
//
// @brief Creates an SSEWriterStage wrapping the given writer.
func NewSSEWriterStage(w io.Writer) *SSEWriterStage {
	return &SSEWriterStage{writer: NewSSEWriter(w)}
}

// Process writes the PipelineEvent as SSE text.
//
// @brief Implements Stage.Process by writing SSE to the underlying writer.
func (s *SSEWriterStage) Process(event PipelineEvent) error {
	switch event.Type {
	case EventOpenAIChunk:
		return s.writer.WriteData(event.Data)
	case EventAnthropicEvent:
		if event.SSEType != "" {
			return s.writer.WriteEvent(event.SSEType, event.Data)
		}
		return s.writer.WriteData(event.Data)
	case EventSSE:
		if event.SSEType != "" {
			return s.writer.WriteEvent(event.SSEType, event.Data)
		}
		return s.writer.WriteData(event.Data)
	case EventDone:
		return s.writer.WriteDone()
	default:
		return nil
	}
}

// Initialize is a no-op for SSEWriterStage.
func (s *SSEWriterStage) Initialize() error { return nil }

// Flush is a no-op for SSEWriterStage (SSE writes are unbuffered).
func (s *SSEWriterStage) Flush() error { return nil }

// Close is a no-op for SSEWriterStage.
func (s *SSEWriterStage) Close() error { return nil }
