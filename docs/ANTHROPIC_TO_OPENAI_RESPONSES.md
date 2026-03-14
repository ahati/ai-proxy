# Anthropic-to-OpenAI Responses API Bridge

## Overview

The `/v1/anthropic-to-openai/responses` endpoint enables clients using the **OpenAI Responses API** format to communicate with **Anthropic-compatible** upstream APIs. This bridge performs bidirectional translation between the two API formats, allowing seamless integration of OpenAI SDK clients with Anthropic-based backends.

## Table of Contents

1. [Architecture](#architecture)
2. [Request Flow](#request-flow)
3. [API Format Conversion](#api-format-conversion)
4. [Configuration](#configuration)
5. [Usage Examples](#usage-examples)
6. [Error Handling](#error-handling)
7. [Testing](#testing)
8. [Implementation Details](#implementation-details)

---

## Architecture

```
┌─────────────────┐      ┌─────────────────────────────┐      ┌─────────────────────┐
│   OpenAI SDK    │ ───▶ │   AI Proxy                  │ ───▶ │  Anthropic API      │
│   Client        │ ◀─── │   /v1/anthropic-to-openai/  │ ◀─── │  (e.g., DashScope)  │
│                 │      │   /responses                │      │                     │
└─────────────────┘      └─────────────────────────────┘      └─────────────────────┘
     Responses API                Transformation Layer              Messages API
```

### Components

| Component | File | Purpose |
|-----------|------|---------|
| Handler | `api/handlers/anthropic_to_openai.go` | Request validation, transformation, and proxying |
| Types | `types/openai_responses.go` | OpenAI Responses API type definitions |
| Transformer | `transform/toolcall/responses_transformer.go` | SSE response transformation |
| Formatter | `transform/toolcall/responses_transformer.go` | Event formatting for Responses API |

---

## Request Flow

### 1. Incoming Request (OpenAI Format)

```json
{
  "model": "claude-3-opus",
  "input": [
    {"type": "message", "role": "user", "content": "Hello"}
  ],
  "instructions": "Be helpful",
  "tools": [
    {
      "type": "function",
      "function": {
        "name": "get_weather",
        "parameters": {"type": "object", "properties": {...}}
      }
    }
  ],
  "tool_choice": "auto",
  "stream": true
}
```

### 2. Transformation to Anthropic Format

```json
{
  "model": "claude-3-opus",
  "messages": [
    {"role": "user", "content": "Hello"}
  ],
  "system": "Be helpful",
  "tools": [
    {
      "name": "get_weather",
      "input_schema": {"type": "object", "properties": {...}}
    }
  ],
  "tool_choice": {"type": "auto"},
  "stream": true
}
```

### 3. Upstream Response (Anthropic SSE)

```
event: message_start
data: {"type": "message_start", "message": {"id": "msg_abc", "role": "assistant"}}

event: content_block_start
data: {"type": "content_block_start", "index": 0, "content_block": {"type": "text"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "Hello"}}
```

### 4. Transformed Response (OpenAI Responses SSE)

```
data: {"type": "response.created", "response": {"id": "resp_abc", "status": "in_progress"}}

data: {"type": "response.content_part.added", "item_id": "resp_abc", "content_index": 0, "part": {"type": "output_text"}}

data: {"type": "response.output_text.delta", "item_id": "resp_abc", "content_index": 0, "delta": "Hello"}
```

---

## API Format Conversion

### Role Mapping

| OpenAI Role | Anthropic Role | Notes |
|-------------|----------------|-------|
| `user` | `user` | Direct mapping |
| `assistant` | `assistant` | Direct mapping |
| `developer` | N/A | Extracted to `system` field |
| `system` | N/A | Extracted to `system` field |

### Tool Choice Conversion

| OpenAI Format | Anthropic Format | Notes |
|---------------|------------------|-------|
| `"auto"` | `{"type": "auto"}` | Default behavior |
| `"required"` | `{"type": "any"}` | Force tool use |
| `"none"` | `null` | Omitted (no tools sent) |
| `{"type": "function", "function": {"name": "xyz"}}` | `{"type": "tool", "name": "xyz"}` | Specific tool |

### Tool Definition Conversion

**OpenAI:**
```json
{
  "type": "function",
  "function": {
    "name": "get_weather",
    "description": "Get weather info",
    "parameters": {"type": "object", "properties": {...}}
  }
}
```

**Anthropic:**
```json
{
  "name": "get_weather",
  "description": "Get weather info",
  "input_schema": {"type": "object", "properties": {...}}
}
```

**Filtered Types:** `web_search`, `file_search`, `computer_use_preview`, `custom` (only `function` tools are converted)

### Content Part Handling

| OpenAI Content Part | Anthropic Equivalent | Notes |
|---------------------|----------------------|-------|
| `input_text` | Text content | Extracted and concatenated |
| `input_image` | N/A | Replaced with "[Image attached]" placeholder |
| `input_file` | N/A | Not supported |

### Response Event Mapping

| Anthropic Event | OpenAI Responses Event |
|-----------------|------------------------|
| `message_start` | `response.created` |
| `content_block_start` (text) | `response.content_part.added` (output_text) |
| `content_block_start` (tool_use) | `response.content_part.added` (function_call) |
| `content_block_delta` (text_delta) | `response.output_text.delta` |
| `content_block_delta` (input_json_delta) | `response.function_call_arguments.delta` |
| `content_block_stop` | `response.content_part.done` |
| `message_stop` | `response.completed` |

---

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `ANTHROPIC_UPSTREAM_URL` | `https://coding-intl.dashscope.aliyuncs.com/apps/anthropic/v1/messages` | Anthropic API endpoint |
| `ANTHROPIC_API_KEY` | (required) | API key for authentication |
| `PORT` | `8080` | Server port |
| `SSELOG_DIR` | (optional) | Directory for request/response logging |

### Command-Line Flags

```bash
./ai-proxy \
  --anthropic-upstream-url="https://api.anthropic.com/v1/messages" \
  --anthropic-api-key="sk-xxx" \
  --port="8080" \
  --sse-log-dir="./logs"
```

---

## Usage Examples

### Basic Text Request

```bash
curl -X POST http://localhost:8080/v1/anthropic-to-openai/responses \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $ANTHROPIC_API_KEY" \
  -d '{
    "model": "claude-3-opus",
    "input": "Hello, how are you?",
    "stream": true
  }'
```

### With Developer Instructions

```bash
curl -X POST http://localhost:8080/v1/anthropic-to-openai/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-opus",
    "instructions": "You are a helpful assistant.",
    "input": [
      {"type": "message", "role": "developer", "content": "Be concise."},
      {"type": "message", "role": "user", "content": "Hello"}
    ],
    "stream": true
  }'
```

### With Tools

```bash
curl -X POST http://localhost:8080/v1/anthropic-to-openai/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-opus",
    "input": "What's the weather in San Francisco?",
    "tools": [
      {
        "type": "function",
        "function": {
          "name": "get_weather",
          "description": "Get current weather",
          "parameters": {
            "type": "object",
            "properties": {
              "location": {"type": "string"}
            },
            "required": ["location"]
          }
        }
      }
    ],
    "tool_choice": "auto",
    "stream": true
  }'
```

### With Content Parts (Codex-style)

```bash
curl -X POST http://localhost:8080/v1/anthropic-to-openai/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-opus",
    "input": [
      {
        "type": "message",
        "role": "user",
        "content": [
          {"type": "input_text", "text": "Hello"},
          {"type": "input_text", "text": "World"}
        ]
      }
    ],
    "stream": true
  }'
```

---

## Error Handling

### Error Response Format

```json
{
  "type": "error",
  "error": {
    "code": "invalid_request_error",
    "message": "..."
  }
}
```

### Common Errors

| Status | Code | Description |
|--------|------|-------------|
| 400 | `invalid_request_error` | Missing model, invalid JSON |
| 502 | `bad_gateway` | Upstream connection failed |
| 504 | `gateway_timeout` | Upstream timeout |

### Logging

When `SSELOG_DIR` is configured, each request creates a JSON log file:

```json
{
  "request_id": "msg_abc",
  "started_at": "2026-03-13T22:11:50Z",
  "duration_ms": 984,
  "method": "POST",
  "path": "/v1/anthropic-to-openai/responses",
  "client_ip": "192.168.1.100:41068",
  "downstream_request": { ... },
  "upstream_request": { ... },
  "upstream_response": { ... }
}
```

---

## Testing

### Unit Tests

```bash
go test ./api/handlers/... -v -run TestAnthropicToOpenAI
go test ./transform/toolcall/... -v -run TestResponses
go test ./types/... -v -run TestResponses
```

### Integration Test

```bash
# Start proxy
ANTHROPIC_API_KEY="your-key" ./ai-proxy --sse-log-dir ./logs &

# Send test request
curl -X POST http://localhost:8080/v1/anthropic-to-openai/responses \
  -H "Content-Type: application/json" \
  -d '{"model": "claude-3-opus", "input": "Hello", "stream": true}'
```

---

## Implementation Details

### File Structure

```
ai-proxy/
├── api/handlers/anthropic_to_openai.go       # Main handler
├── api/handlers/anthropic_to_openai_test.go  # Handler tests
├── transform/toolcall/responses_transformer.go   # SSE transformer
├── transform/toolcall/responses_transformer_test.go
├── types/openai_responses.go                 # Type definitions
└── types/openai_responses_test.go            # Type tests
```

### Key Functions

#### `transformResponsesRequest()`
Converts OpenAI ResponsesRequest to Anthropic MessageRequest.

**Pipeline:**
1. Parse OpenAI JSON
2. Extract system/developer messages
3. Convert user/assistant messages
4. Transform tools
5. Convert tool_choice
6. Marshal Anthropic JSON

#### `convertInputToMessages()`
Handles input array with role mapping and content extraction.

#### `extractContent()`
Extracts text from string or content part arrays.

#### `convertResponsesTools()`
Filters and transforms tool definitions.

#### `convertToolChoice()`
Maps OpenAI tool_choice to Anthropic format.

### Response Transformer

#### `ResponsesTransformer`

State machine for SSE event transformation:

```go
type ResponsesTransformer struct {
    output       io.Writer
    formatter    *ResponsesFormatter
    messageID    string
    model        string
    blockIndex   int
    inToolCall   bool
    inText       bool
    inReasoning  bool
    textContent  strings.Builder
    toolArgs     strings.Builder
}
```

#### Event Handlers

| Method | Purpose |
|--------|---------|
| `handleMessageStart()` | Emit `response.created` |
| `handleContentBlockStart()` | Emit `response.content_part.added` |
| `handleContentBlockDelta()` | Emit text/tool deltas |
| `handleContentBlockStop()` | Emit `response.content_part.done` |
| `handleMessageStop()` | Emit `response.completed` |

---

## Limitations

1. **Image Support**: Images (`input_image`) are replaced with placeholder text
2. **Custom Tools**: Only `function` type tools are converted
3. **Web Search**: `web_search` tool type is filtered out
4. **File Attachments**: Not supported
5. **Tool Choice `none`**: Not supported by Anthropic (omitted)

---

## Future Enhancements

- [ ] Support for image content via base64 encoding
- [ ] Tool result message conversion
- [ ] Multi-turn conversation state management
- [ ] Token usage reporting
- [ ] Response caching

---

## References

- [OpenAI Responses API Documentation](https://platform.openai.com/docs/api-reference/responses)
- [Anthropic Messages API Documentation](https://docs.anthropic.com/en/api/messages)
- [LiteLLM Responses Implementation](https://github.com/BerriAI/litellm/tree/main/litellm/responses)

---

## Changelog

### v1.0.0
- Initial implementation
- Support for text and tool calls
- SSE streaming
- Tool choice conversion
- Content part handling
