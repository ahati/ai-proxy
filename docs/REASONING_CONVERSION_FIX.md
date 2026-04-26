# Reasoning Items Conversion Fix

## Problem Statement

When converting OpenAI Responses API requests to upstream provider formats, `reasoning` items from previous conversation turns were being **silently dropped**. This caused errors from providers that require reasoning content to be passed back in subsequent requests:

```json
{
  "error": {
    "message": "Error from provider (DeepSeek): The `reasoning_content` in the thinking mode must be passed back to the API.",
    "type": "invalid_request_error"
  }
}
```

## Root Cause

The proxy routes `/v1/responses` to two different upstream formats depending on the model configuration:

| Route | Upstream Format | File |
|-------|----------------|------|
| Responses → Anthropic | `/v1/messages` | `convert/responses_to_anthropic.go` |
| Responses → Chat Completions | `/v1/chat/completions` | `convert/responses_to_chat.go` |

**Both** converters had the same bug — reasoning items were dropped:

```go
// responses_to_anthropic.go — explicit drop:
case "reasoning":
    // Dropped when converting into an Anthropic request.

// responses_to_chat.go — implicit drop (no case at all):
switch itemType {
case "function_call":       // handled
case "message":             // handled
case "function_call_output": // handled
// reasoning:               // ← missing, silently ignored
}
```

## Fix

### Part 1: `types/openai.go` — Add `ReasoningContent` to `Message`

The `Message` struct (used in Chat Completions request messages) lacked a `ReasoningContent` field. Added as a `*string` (pointer) to distinguish three states:

- `nil` → field omitted from JSON (no reasoning in the conversation)
- `&""` → `"reasoning_content": ""` (prior turns had reasoning, this turn doesn't)
- `&"text"` → `"reasoning_content": "text"` (this turn has reasoning)

```go
// ReasoningContent contains the model's reasoning process from previous turns.
// Uses *string to distinguish nil (omit from JSON) from "" (present but empty).
ReasoningContent *string `json:"reasoning_content,omitempty"`
```

DeepSeek requires `reasoning_content` as an explicit key on **every** assistant message in a multi-turn thinking-mode conversation, even when the value is empty for the current turn.

### Part 2: `convert/common.go` — Shared `ExtractReasoningText` helper

Added a shared helper to extract concatenated summary text from a reasoning item's `summary` array:

```go
func ExtractReasoningText(item map[string]interface{}) string
```

This replaces the former near-duplicate helpers `extractReasoningTextFromItem` and `extractReasoningSummaryText` that existed in the two converter files.

### Part 3: `convert/responses_to_anthropic.go` — Convert reasoning → thinking

Replaced the drop with conversion to Anthropic `thinking` content blocks:

```go
case "reasoning":
    reasoningText := ExtractReasoningText(msg)
    if reasoningText != "" {
        block := map[string]interface{}{
            "type":     "thinking",
            "thinking": reasoningText,
        }
        appendMessage("assistant", []interface{}{block})
    }
```

The `signature` field is deliberately omitted — creating a valid cryptographic signature requires additional implementation beyond the scope of this fix.

### Part 4: `convert/responses_to_chat.go` — Convert reasoning → `reasoning_content`

Added `pendingReasoning` to `groupProcessor` and a `handleReasoningItem` method that buffers reasoning text, then attaches it to the next assistant message. Reasoning is tracked through the grouping system so it can be combined with tool calls even when no message body exists.

Added `ensureReasoningOnAllAssistants` — when any reasoning exists in the conversation, **all** assistant messages get a `ReasoningContent` pointer (at least `&""`). This satisfies DeepSeek's requirement that every assistant message carries the key in multi-turn thinking mode.

### Part 5: `convert/anthropic_to_responses_streaming.go` — Handle summary on reasoning output items

The reverse direction (Anthropic SSE → Responses format) now extracts `summary_text` parts from reasoning-type output items and populates the `Summary` field.

### Conversion Mapping

| Responses API | Anthropic Messages | Chat Completions |
|--------------|-------------------|------------------|
| `type: "reasoning"` | `type: "thinking"` block | `reasoning_content` field on message |
| `summary[].text` | `thinking` (concatenated) | `reasoning_content` (concatenated) |

### Example

**Input (Responses API):**
```json
[
  {"type": "message", "role": "user", "content": "Hello"},
  {"type": "reasoning", "summary": [{"type": "summary_text", "text": "Analyzing request..."}]},
  {"type": "function_call", "name": "exec_command", "call_id": "c1", "arguments": "{}"},
  {"type": "message", "role": "assistant", "content": ""}
]
```

**Output (Chat Completions):**
```json
{
  "role": "assistant",
  "content": "",
  "reasoning_content": "Analyzing request...",
  "tool_calls": [{"id": "c1", "function": {"name": "exec_command", "arguments": "{}"}}]
}
```

**Backfill behavior:** When turn 1 has reasoning but turn 2 does not, turn 2's assistant message still gets `"reasoning_content": ""` (not omitted) — required by DeepSeek.

## Files Changed

| File | Change |
|------|--------|
| `types/openai.go` | Added `ReasoningContent *string` field to `Message` struct |
| `convert/common.go` | Added shared `ExtractReasoningText` helper |
| `convert/responses_to_anthropic.go` | Convert reasoning → thinking block; use `ExtractReasoningText` |
| `convert/responses_to_anthropic_test.go` | Added `TestConvertResponsesInputItems_ReasoningToThinking` |
| `convert/responses_to_chat.go` | Added `handleReasoningItem`, `ensureReasoningOnAllAssistants`, backfill logic |
| `convert/responses_to_chat_test.go` | Added 5 tests: basic reasoning, tool calls, flushed tool calls, backfill, no-backfill-when-none |
| `convert/anthropic_to_responses_streaming.go` | Handle `summary` field on reasoning output items |
