# AI Proxy

A Go-based HTTP proxy for LLM APIs with OpenAI and Anthropic compatibility. Provides format transformation, tool call normalization, and seamless integration between different API formats.

## Features

- **Multi-format support**: OpenAI Chat Completions, Anthropic Messages, and OpenAI Responses API
- **Bidirectional conversion**: Convert between OpenAI and Anthropic formats in both directions
- **Tool call normalization**: Transforms Kimi-K2.5/K2's proprietary tool call format into standard formats
- **Streaming support**: Real-time SSE streaming with format transformation
- **Request capture**: Optional logging of all requests/responses for debugging

## API Endpoints

| Method | Path | Request Format | Upstream Format | Response Format | Description |
|--------|------|----------------|-----------------|-----------------|-------------|
| `GET` | `/health` | N/A | N/A | N/A | Health check |
| `GET` | `/v1/models` | OpenAI | N/A | OpenAI | List available models |
| `POST` | `/v1/chat/completions` | OpenAI | OpenAI | OpenAI | Chat completions with tool call normalization |
| `POST` | `/v1/messages` | Anthropic | Anthropic | Anthropic | Anthropic messages with tool call normalization |
| `POST` | `/v1/openai-to-anthropic/messages` | Anthropic | OpenAI | Anthropic | Bridge: Anthropic client → OpenAI backend |
| `POST` | `/v1/anthropic-to-openai/responses` | OpenAI Responses | Anthropic | OpenAI Responses | Bridge: OpenAI SDK → Anthropic backend |

## Architecture

```
                                    ┌─────────────────────────────────────┐
                                    │           AI Proxy                  │
                                    │                                     │
┌──────────────┐                    │  ┌─────────────────────────────┐    │
│  OpenAI SDK  │──▶ /v1/chat/completions ──▶│ OpenAI Transformer │───┼──▶ OpenAI Upstream
│              │◀── OpenAI Response ────────│ (tool call fix)    │◀──┼─── OpenAI Response
└──────────────┘                    │  └─────────────────────────────┘    │
                                    │                                     │
┌──────────────┐                    │  ┌─────────────────────────────┐    │
│ Anthropic SDK│──▶ /v1/messages ─────▶│ Anthropic Transformer │─────┼──▶ Anthropic Upstream
│              │◀─ Anthropic Response ─│ (tool call fix)       │◀────┼─── Anthropic Response
└──────────────┘                    │  └─────────────────────────────┘    │
                                    │                                     │
┌──────────────┐                    │  ┌─────────────────────────────┐    │
│ Anthropic SDK│──▶ /v1/openai-to-anthropic/messages ──▶│ Bridge │───┼──▶ OpenAI Upstream
│              │◀── Anthropic Response ─────────────────│(A→O→A) │◀──┼─── OpenAI Response
└──────────────┘                    │  └─────────────────────────────┘    │
                                    │                                     │
┌──────────────┐                    │  ┌─────────────────────────────┐    │
│ OpenAI SDK   │──▶ /v1/anthropic-to-openai/responses ──▶│ Bridge │──┼──▶ Anthropic Upstream
│ (Responses)  │◀── OpenAI Response ─────────────────────│(O→A→O)│◀──┼─── Anthropic Response
└──────────────┘                    │  └─────────────────────────────┘    │
                                    │                                     │
                                    └─────────────────────────────────────┘
```

## Tool Call Transformation

Kimi-K2.5 and K2 models output tool/function calls using special delimiter tokens embedded in the SSE `reasoning` field, rather than standard formats. This proxy transforms these in real-time during streaming.

### Non-Standard Format (Input)

```
<|tool_calls_section_begin|>
<|tool_call_begin|>functions.bash:15<|tool_call_argument_begin|>{"command": "ls -la"}<|tool_call_end|>
<|tool_call_begin|>functions.task:16<|tool_call_argument_begin|>{"description": "..."}<|tool_call_end|>
<|tool_calls_section_end|>
```

### OpenAI Format (Output)

```json
{
  "choices": [{
    "delta": {
      "tool_calls": [{
        "id": "call_abc123",
        "type": "function",
        "function": {
          "name": "bash",
          "arguments": "{\"command\": \"ls -la\"}"
        }
      }]
    }
  }]
}
```

### Anthropic Format (Output)

```json
{
  "type": "content_block_delta",
  "index": 1,
  "delta": {
    "type": "input_json_delta",
    "partial_json": "{\"command\": \"ls -la\"}"
  }
}
```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `8080` | Server port |
| `UPSTREAM_URL` | `https://llm.chutes.ai/v1/chat/completions` | OpenAI-compatible upstream URL |
| `UPSTREAM_API_KEY` | (empty) | API key for OpenAI-compatible upstream |
| `ANTHROPIC_UPSTREAM_URL` | `https://coding-intl.dashscope.aliyuncs.com/apps/anthropic/v1/messages` | Anthropic-compatible upstream URL |
| `ALIBABA_ANTHROPIC_API_KEY` | (empty) | API key for Anthropic upstream |
| `SSELOG_DIR` | (empty) | Directory for request/response logging |

### Endpoint Configuration

| Endpoint | Upstream URL | API Key |
|----------|--------------|---------|
| `/v1/chat/completions` | `UPSTREAM_URL` | `UPSTREAM_API_KEY` |
| `/v1/messages` | `ANTHROPIC_UPSTREAM_URL` | `ALIBABA_ANTHROPIC_API_KEY` |
| `/v1/openai-to-anthropic/messages` | `UPSTREAM_URL` | `UPSTREAM_API_KEY` |
| `/v1/anthropic-to-openai/responses` | `ANTHROPIC_UPSTREAM_URL` | `ALIBABA_ANTHROPIC_API_KEY` |

## Usage

### Build and Run

```bash
# Build
go build -o ai-proxy .

# Run with default configuration
./ai-proxy

# Run with custom upstreams
UPSTREAM_URL=https://api.example.com/v1/chat/completions \
UPSTREAM_API_KEY=your-key \
ANTHROPIC_UPSTREAM_URL=https://api.anthropic.com/v1/messages \
ALIBABA_ANTHROPIC_API_KEY=your-anthropic-key \
PORT=3000 \
./ai-proxy

# Run with request logging
SSELOG_DIR=./logs ./ai-proxy
```

### Example Requests

#### OpenAI Chat Completions

```bash
curl -X POST http://localhost:8080/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "moonshotai/Kimi-K2.5-TEE",
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

#### Anthropic Messages

```bash
curl -X POST http://localhost:8080/v1/messages \
  -H "Content-Type: application/json" \
  -H "Anthropic-Version: 2023-06-01" \
  -d '{
    "model": "kimi-k2.5",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

#### OpenAI-to-Anthropic Bridge

Use Anthropic SDK with an OpenAI-compatible backend:

```bash
curl -X POST http://localhost:8080/v1/openai-to-anthropic/messages \
  -H "Content-Type: application/json" \
  -H "Anthropic-Version: 2023-06-01" \
  -d '{
    "model": "gpt-4",
    "max_tokens": 1024,
    "messages": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

#### Anthropic-to-OpenAI Responses Bridge

Use OpenAI SDK (Responses API) with an Anthropic backend:

```bash
curl -X POST http://localhost:8080/v1/anthropic-to-openai/responses \
  -H "Content-Type: application/json" \
  -d '{
    "model": "claude-3-5-sonnet-20241022",
    "input": [{"role": "user", "content": "Hello"}],
    "stream": true
  }'
```

## Request Capture

When `SSELOG_DIR` is set, all requests are captured to structured JSON files:

```bash
SSELOG_DIR=./logs ./ai-proxy

# Logs are organized by date
ls logs/$(date +%Y-%m-%d)/
```

**Captured data** (4 capture points):
1. **Downstream TX** - Client request to proxy
2. **Upstream TX** - Proxy request to LLM API
3. **Upstream RX** - LLM API response to proxy
4. **Downstream RX** - Proxy response to client

## Project Structure

```
ai-proxy/
├── main.go                 # Entry point, server initialization
├── api/                    # HTTP server and routing
│   ├── server.go           # Server setup and route registration
│   ├── middleware.go       # Capture middleware
│   └── handlers/           # HTTP request handlers
│       ├── health.go       # Health check endpoint
│       ├── models.go       # Models listing endpoint
│       ├── completions.go  # OpenAI chat completions
│       ├── messages.go     # Anthropic messages endpoint
│       ├── bridge.go       # OpenAI-to-Anthropic bridge
│       ├── anthropic_to_openai.go  # Anthropic-to-OpenAI bridge
│       ├── count_tokens.go # Token counting endpoint
│       └── common.go       # Shared handler utilities
├── config/                 # Configuration loading
│   └── config.go
├── logging/                # Logging utilities
│   └── logging.go
├── proxy/                  # Upstream API client
│   ├── client.go           # HTTP client for upstream APIs
│   └── request.go          # Request building utilities
├── transform/              # Response transformation
│   ├── interface.go        # Transformer interface
│   ├── passthrough.go      # Pass-through transformer
│   └── toolcall/           # Tool call format transformation
│       ├── transformer.go  # OpenAI transformer state machine
│       ├── anthropic_transformer.go  # Anthropic transformer
│       ├── responses_transformer.go   # OpenAI Responses API transformer
│       ├── parser.go       # Token parsing
│       ├── tokens.go       # Special token definitions
│       ├── formatter.go    # Output formatting
│       ├── anthropic.go    # Anthropic format support
│       ├── openai.go       # OpenAI format support
│       └── common.go       # Shared utilities
├── types/                  # Type definitions
│   ├── openai.go           # OpenAI API types
│   ├── openai_responses.go # OpenAI Responses API types
│   ├── anthropic.go        # Anthropic API types
│   └── sse.go              # Server-Sent Events types
└── capture/                # Request/response capture
    ├── storage.go          # Log storage management
    ├── writer.go           # JSON log writer
    ├── recorder.go         # Request/response recording
    └── context.go          # Context utilities
```

## Development

```bash
# Run tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific test
go test -v -run TestFunctionName ./...

# Format code
go fmt ./...

# Static analysis
go vet ./...

# Tidy dependencies
go mod tidy
```

## Technical Details

### Tool Call Transformation

The `ToolCallTransformer` implements a 5-state machine (`IDLE → IN_SECTION → READING_ID → READING_ARGS → TRAILING`) that:

1. Buffers incoming reasoning text across SSE chunks
2. Detects special delimiter tokens
3. Extracts function name and arguments
4. Emits properly formatted tool calls in the target format

### Supported Special Tokens

| Token | Description |
|-------|-------------|
| `<|tool_calls_section_begin|>` | Starts the tool calls section |
| `<|tool_call_begin|>` | Starts a function call (ID/name follows) |
| `<|tool_call_argument_begin|>` | Starts the JSON arguments |
| `<|tool_call_end|>` | Ends the current tool call |
| `<|tool_calls_section_end|>` | Ends the tool calls section |

### Format Conversions

| Conversion | Request Transform | Response Transform |
|------------|-------------------|-------------------|
| OpenAI → OpenAI | None (pass-through) | Tool call normalization |
| Anthropic → Anthropic | None (pass-through) | Tool call normalization |
| Anthropic → OpenAI → Anthropic | Anthropic to OpenAI | OpenAI to Anthropic |
| OpenAI Responses → Anthropic → OpenAI Responses | Responses to Anthropic | Anthropic to Responses |

## License

MIT
