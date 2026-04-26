// Package convert provides shared helper functions for converting between
// OpenAI and Anthropic API formats.
package convert

import (
	"ai-proxy/types"
	"encoding/json"
	"fmt"
	"math"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// JSON Helpers
// ─────────────────────────────────────────────────────────────────────────────

// unmarshalArgs parses a JSON arguments string into a map.
func unmarshalArgs(args string) map[string]interface{} {
	result := map[string]interface{}{}
	if args == "" {
		return result
	}
	if err := json.Unmarshal([]byte(args), &result); err != nil {
		return map[string]interface{}{}
	}
	return result
}

// ─────────────────────────────────────────────────────────────────────────────
// Tool Conversion
// ─────────────────────────────────────────────────────────────────────────────

// ConvertOpenAIToolsToAnthropic converts OpenAI tool definitions to Anthropic format.
// OpenAI uses "parameters" while Anthropic uses "input_schema".
func ConvertOpenAIToolsToAnthropic(openTools []types.Tool) []types.ToolDef {
	if len(openTools) == 0 {
		return []types.ToolDef{}
	}

	anthTools := make([]types.ToolDef, 0, len(openTools))
	for _, openTool := range openTools {
		if openTool.Type == "function" {
			anthTools = append(anthTools, types.ToolDef{
				Name:        openTool.Function.Name,
				Description: openTool.Function.Description,
				InputSchema: openTool.Function.Parameters,
			})
		}
	}
	return anthTools
}

// ─────────────────────────────────────────────────────────────────────────────
// Text Extraction
// ─────────────────────────────────────────────────────────────────────────────

// ExtractTextFromContent extracts text from various content formats.
// This is the SINGLE source of truth for text extraction across the codebase.
// All other implementations should call this function.
//
// Content can be:
//   - string: returned directly
//   - []interface{}: array of content blocks (Anthropic/OpenAI format)
//   - []map[string]interface{}: typed content blocks
//   - nil: returns empty string
//
// Supported content block types:
//   - "text", "input_text", "output_text", "refusal": extracts "text" field
//   - "thinking": extracts "thinking" field
func ExtractTextFromContent(content interface{}) string {
	return extractTextFromContentValue(content, true, "\n")
}

// ExtractSystemText extracts system content from Anthropic system payloads.
//
// It accepts the string and block-array forms used by Anthropic and returns a
// concatenated string, ignoring non-text blocks.
func ExtractSystemText(system interface{}) string {
	return extractTextFromContentValue(system, false, "")
}

func extractTextFromContentValue(content interface{}, includeThinking bool, separator string) string {
	if content == nil {
		return ""
	}

	switch c := content.(type) {
	case string:
		return c
	case json.RawMessage:
		return extractTextFromRawMessage(c, includeThinking, separator)
	case []interface{}:
		return extractTextFromInterfaceSlice(c, includeThinking, separator)
	case []map[string]interface{}:
		return extractTextFromMapSlice(c, includeThinking, separator)
	case []types.ContentBlock:
		return extractTextFromContentBlocks(c, includeThinking, separator)
	case types.ContentBlock:
		return extractTextFromContentBlock(c, includeThinking)
	case []types.SystemBlock:
		return extractTextFromSystemBlocks(c, separator)
	case types.SystemBlock:
		return c.Text
	default:
		return ""
	}
}

func extractTextFromRawMessage(data json.RawMessage, includeThinking bool, separator string) string {
	if len(data) == 0 {
		return ""
	}

	var stringValue string
	if err := json.Unmarshal(data, &stringValue); err == nil {
		return stringValue
	}

	var interfaceSlice []interface{}
	if err := json.Unmarshal(data, &interfaceSlice); err == nil {
		return extractTextFromInterfaceSlice(interfaceSlice, includeThinking, separator)
	}

	var mapSlice []map[string]interface{}
	if err := json.Unmarshal(data, &mapSlice); err == nil {
		return extractTextFromMapSlice(mapSlice, includeThinking, separator)
	}

	return ""
}

// extractTextFromInterfaceSlice extracts text from a slice of interface{} (untyped content blocks).
func extractTextFromInterfaceSlice(blocks []interface{}, includeThinking bool, separator string) string {
	var result strings.Builder
	for _, part := range blocks {
		if partMap, ok := part.(map[string]interface{}); ok {
			text := extractTextFromBlock(partMap, includeThinking)
			if text != "" {
				appendExtractedText(&result, text, separator)
			}
			continue
		}

		if contentBlock, ok := part.(types.ContentBlock); ok {
			text := extractTextFromContentBlock(contentBlock, includeThinking)
			if text != "" {
				appendExtractedText(&result, text, separator)
			}
			continue
		}

		if str, ok := part.(string); ok {
			appendExtractedText(&result, str, separator)
		}
	}
	return result.String()
}

// extractTextFromMapSlice extracts text from a slice of map[string]interface{} (typed content blocks).
func extractTextFromMapSlice(blocks []map[string]interface{}, includeThinking bool, separator string) string {
	var result strings.Builder
	for _, part := range blocks {
		text := extractTextFromBlock(part, includeThinking)
		if text != "" {
			appendExtractedText(&result, text, separator)
		}
	}
	return result.String()
}

func extractTextFromContentBlocks(blocks []types.ContentBlock, includeThinking bool, separator string) string {
	var result strings.Builder
	for _, part := range blocks {
		text := extractTextFromContentBlock(part, includeThinking)
		if text != "" {
			appendExtractedText(&result, text, separator)
		}
	}
	return result.String()
}

func extractTextFromSystemBlocks(blocks []types.SystemBlock, separator string) string {
	var result strings.Builder
	for _, part := range blocks {
		if part.Text != "" {
			appendExtractedText(&result, part.Text, separator)
		}
	}
	return result.String()
}

func extractTextFromContentBlock(block types.ContentBlock, includeThinking bool) string {
	switch block.Type {
	case "text", "input_text", "output_text":
		return block.Text
	case "thinking":
		if includeThinking {
			return block.Thinking
		}
	}
	return ""
}

func appendExtractedText(builder *strings.Builder, text, separator string) {
	if builder.Len() > 0 && separator != "" {
		builder.WriteString(separator)
	}
	builder.WriteString(text)
}

// extractTextFromBlock extracts text from a single content block based on its type.
func extractTextFromBlock(block map[string]interface{}, includeThinking bool) string {
	blockType, _ := block["type"].(string)
	switch blockType {
	case "text", "input_text", "output_text":
		if text, ok := block["text"].(string); ok {
			return text
		}
	case "refusal":
		if refusal, ok := block["refusal"].(string); ok {
			return refusal
		}
	case "thinking":
		if includeThinking {
			if thinking, ok := block["thinking"].(string); ok {
				return thinking
			}
		}
	}
	return ""
}

// ─────────────────────────────────────────────────────────────────────────────
// Temperature Clamping
// ─────────────────────────────────────────────────────────────────────────────

// ClampTemperatureToAnthropic clamps temperature from OpenAI range (0-2) to Anthropic range (0-1).
func ClampTemperatureToAnthropic(t float64) float64 {
	return math.Min(math.Max(t, 0), 1.0)
}

// ─────────────────────────────────────────────────────────────────────────────
// Data URI Helpers
// ─────────────────────────────────────────────────────────────────────────────

// ParseDataURI parses a data URI into media type and base64 data.
// Expected format: "data:<mediaType>;base64,<data>"
// Returns mediaType, data, error.
func ParseDataURI(uri string) (string, string, error) {
	if !strings.HasPrefix(uri, "data:") {
		return "", "", fmt.Errorf("not a data URI: %q", uri)
	}
	rest := strings.TrimPrefix(uri, "data:")
	semi := strings.Index(rest, ";")
	if semi < 0 {
		return "", "", fmt.Errorf("malformed data URI (missing semicolon): %q", uri)
	}
	mediaType := rest[:semi]
	rest = rest[semi+1:]
	if !strings.HasPrefix(rest, "base64,") {
		return "", "", fmt.Errorf("only base64 data URIs are supported: %q", uri)
	}
	data := strings.TrimPrefix(rest, "base64,")
	return mediaType, data, nil
}

// BuildDataURI constructs a data URI from media type and base64 data.
func BuildDataURI(mediaType, data string) string {
	return fmt.Sprintf("data:%s;base64,%s", mediaType, data)
}

// ─────────────────────────────────────────────────────────────────────────────
// JSON Helpers for Tool Choice
// ─────────────────────────────────────────────────────────────────────────────

// MustMarshal marshals v to JSON and panics on error (for use in tests/examples).
func MustMarshal(v interface{}) json.RawMessage {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return b
}

// IsJSONString reports whether raw starts with a JSON string ('"').
func IsJSONString(raw json.RawMessage) bool {
	return len(raw) > 0 && raw[0] == '"'
}

// ─────────────────────────────────────────────────────────────────────────────
// Tool-Choice Converters
// ─────────────────────────────────────────────────────────────────────────────

// AnthropicToolChoiceObject is the object form of Anthropic tool_choice.
type AnthropicToolChoiceObject struct {
	Type string `json:"type"` // "auto" | "any" | "tool"
	Name string `json:"name,omitempty"`
}

// ChatToolChoiceObject is the object form of Chat Completions tool_choice.
type ChatToolChoiceObject struct {
	Type     string `json:"type"` // "function"
	Function struct {
		Name string `json:"name"`
	} `json:"function"`
}

// ResponsesToolChoiceObject is the object form of Responses API tool_choice.
type ResponsesToolChoiceObject struct {
	Type string `json:"type"` // "function"
	Name string `json:"name"`
}

// AnthropicToolChoiceToResponses converts an Anthropic tool_choice into Responses API form.
func AnthropicToolChoiceToResponses(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if IsJSONString(raw) {
		var s string
		_ = json.Unmarshal(raw, &s)
		switch s {
		case "auto":
			return MustMarshal("auto")
		case "any":
			return MustMarshal("required")
		}
		return MustMarshal("auto")
	}
	var obj AnthropicToolChoiceObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	return MustMarshal(ResponsesToolChoiceObject{Type: "function", Name: obj.Name})
}

// ResponsesToolChoiceToChat converts a Responses API tool_choice into Chat Completions form.
func ResponsesToolChoiceToChat(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if IsJSONString(raw) {
		// "auto", "none", "required" all map 1-to-1.
		return raw
	}
	var obj ResponsesToolChoiceObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	out := ChatToolChoiceObject{Type: "function"}
	out.Function.Name = obj.Name
	return MustMarshal(out)
}

// ChatToolChoiceToResponses converts a Chat Completions tool_choice to Responses API form.
func ChatToolChoiceToResponses(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return nil
	}
	if IsJSONString(raw) {
		// "auto", "none", "required" all map directly.
		return raw
	}
	var obj ChatToolChoiceObject
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil
	}
	return MustMarshal(ResponsesToolChoiceObject{Type: "function", Name: obj.Function.Name})
}

// ─────────────────────────────────────────────────────────────────────────────
// Reasoning / Thinking Budget Mapping
// ─────────────────────────────────────────────────────────────────────────────

// BudgetToReasoningEffort maps an Anthropic thinking budget_tokens value to a
// Responses API reasoning effort string.
func BudgetToReasoningEffort(budget int) string {
	switch {
	case budget <= 4000:
		return "low"
	case budget <= 10000:
		return "medium"
	default:
		return "high"
	}
}

// marshalToolChoice marshals a ToolChoice to JSON. Used by AnthropicToResponsesRequest.
func marshalToolChoice(tc *types.ToolChoice) json.RawMessage {
	if tc == nil {
		return nil
	}
	b, _ := json.Marshal(tc)
	return b
}

// extractSystemFromRequest extracts system content from Anthropic system payloads.
func extractSystemFromRequest(system interface{}) string {
	if system == nil {
		return ""
	}
	switch s := system.(type) {
	case string:
		return s
	case json.RawMessage:
		return ExtractSystemText(s)
	default:
		b, _ := json.Marshal(system)
		return ExtractSystemText(b)
	}
}

// extractToolResultContent extracts text content from a tool_result content field.
func extractToolResultContent(content interface{}) string {
	if content == nil {
		return ""
	}
	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var parts []string
		for _, item := range c {
			if b, ok := item.(map[string]interface{}); ok {
				if b["type"] == "text" {
					if text, ok := b["text"].(string); ok {
						parts = append(parts, text)
					}
				}
			}
		}
		return strings.Join(parts, "\n")
	default:
		b, _ := json.Marshal(content)
		return string(b)
	}
}

// ExtractReasoningText extracts the concatenated summary text from a reasoning
// item's "summary" array. Each summary entry has the form
// {"type":"summary_text","text":"..."}; all non-empty texts are joined with
// newlines.
func ExtractReasoningText(item map[string]interface{}) string {
	summaryRaw, ok := item["summary"]
	if !ok {
		return ""
	}
	summaryArray, ok := summaryRaw.([]interface{})
	if !ok {
		return ""
	}
	var parts []string
	for _, part := range summaryArray {
		partMap, ok := part.(map[string]interface{})
		if !ok {
			continue
		}
		if t, _ := partMap["type"].(string); t == "summary_text" {
			if text, _ := partMap["text"].(string); text != "" {
				parts = append(parts, text)
			}
		}
	}
	return strings.Join(parts, "\n")
}
