# AI Proxy for Kimi-K2.5 / K2

A Go-based HTTP proxy that transforms Kimi-K2.5 and K2's proprietary tool call format into OpenAI-compatible `tool_calls` format, enabling seamless integration with OpenAI-compatible clients and SDKs.

## Problem Statement

Kimi-K2.5 and K2 models output tool/function calls using special delimiter tokens embedded in the SSE `reasoning` field, rather than the standard OpenAI `tool_calls` format. Cloud providers typically fix this server-side, but self-hosted deployments or direct API access expose this incompatibility, breaking OpenAI-compatible clients, SDKs, and tools.

### Non-Standard Tool Call Format (Example)

Kimi-K2.5 and K2 use special delimiter tokens instead of OpenAI's structured JSON:

```
<|tool_calls_section_begin|>
<|tool_call_begin|>functions.bash:15<|tool_call_argument_begin|>{"command": "ls -la"}<|tool_call_end|>
<|tool_call_begin|>functions.task:16<|tool_call_argument_begin|>{"description": "..."}<|tool_call_end|>
<|tool_calls_section_end|>
```

**Note:** This behavior is intermittent—it occurs only sometimes depending on the model's response. The proxy handles both cases: when tool calls appear in reasoning tokens (transforms them) and when they use standard format (passes through unchanged).

### OpenAI's Expected Format

OpenAI-compatible clients expect tool calls in this structured format:

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

## Solution

This proxy sits between your application and the Kimi-K2.5/K2 upstream API, transforming the non-standard tool call format in real-time during SSE streaming:

```
┌─────────────┐      ┌──────────────────────┐      ┌────────────────────┐
│   Client    │ ───▶ │   AI Proxy           │ ───▶ │  Kimi-K2.5/K2 API  │
│ (OpenAI SDK)│ ◀─── │ (ToolCallTransformer)│ ◀─── │(e.g. llm.chutes.ai)│
└─────────────┘      └──────────────────────┘      └────────────────────┘
```

### Key Features

- **Real-time transformation**: Converts tool call tokens to OpenAI format during streaming
- **Token reassembly**: Handles special tokens split across multiple SSE chunks via state machine buffering
- **Full OpenAI compatibility**: Exposes standard `/v1/chat/completions`, `/v1/models`, and `/health` endpoints
- **Pass-through for non-tool responses**: Regular text completions pass through unchanged

## API Endpoints

| Method | Path | Format | Description |
|--------|------|--------|-------------|
| `GET` | `/health` | N/A | Health check |
| `GET` | `/v1/models` | OpenAI | List available models |
| `POST` | `/v1/chat/completions` | OpenAI | Chat completions (streaming) |
| `POST` | `/v1/messages` | Anthropic | Chat completions (streaming) |
| `POST` | `/v1/openai-to-anthropic/messages` | Anthropic | Reverse proxy: Anthropic format → OpenAI upstream → Anthropic response |
| `POST` | `/v1/anthropic-to-openai/responses` | OpenAI | Reverse proxy: OpenAI Responses API format → Anthropic upstream → OpenAI Responses API response |

## Configuration

### OpenAI Format (`/v1/chat/completions`)

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `PORT` | `8080` | Server port |
| `UPSTREAM_URL` | `https://llm.chutes.ai/v1/chat/completions` | OpenAI-compatible upstream URL |
| `UPSTREAM_API_KEY` | (empty) | API key for OpenAI-compatible upstream |

### Anthropic Format (`/v1/messages`)

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `ANTHROPIC_UPSTREAM_URL` | `https://coding-intl.dashscope.aliyuncs.com/apps/anthropic/v1/messages` | Anthropic-compatiable upstream URL |
| `ANTHROPIC_API_KEY` | (empty) | API key for Anthropic upstream |

### OpenAI-to-Anthropic Reverse Proxy (`/v1/openai-to-anthropic/messages`)

This endpoint accepts requests in **Anthropic format**, forwards them to an **OpenAI-compatible upstream**, and transforms the response back to **Anthropic format**. Useful when you have clients expecting Anthropic API responses but need to use an OpenAI-compatible backend.

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `UPSTREAM_URL` | `https://llm.chutes.ai/v1/chat/completions` | OpenAI-compatible upstream URL |
| `UPSTREAM_API_KEY` | (empty) | API key for OpenAI-compatible upstream |

### Anthropic-to-OpenAI Responses Reverse Proxy (`/v1/anthropic-to-openai/responses`)

This endpoint accepts requests in **OpenAI Responses API format**, forwards them to an **Anthropic-compatible upstream**, and transforms the response back to **OpenAI Responses API format**. Useful when you have clients using the OpenAI SDK (responses API) but need to call an Anthropic-compatible backend.

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `ANTHROPIC_UPSTREAM_URL` | `https://coding-intl.dashscope.aliyuncs.com/apps/anthropic/v1/messages` | Anthropic-compatible upstream URL |
| `ANTHROPIC_API_KEY` | (empty) | API key for Anthropic upstream |

### Common

| Environment Variable | Default | Description |
|---------------------|---------|-------------|
| `SSELOG_DIR` | (empty) | Directory for SSE debug logs |

## Usage

```bash
# Build
go build -o ai-proxy .

# Run with OpenAI-compatible upstream (default)
./ai-proxy

# Run with custom OpenAI upstream
UPSTREAM_URL=https://llm.chutes.ai/v1/chat/completions \
UPSTREAM_API_KEY=your-key \
./ai-proxy

# Run with Anthropic upstream
ANTHROPIC_UPSTREAM_URL=https://coding-intl.dashscope.aliyuncs.com/apps/anthropic/v1/messages \
ANTHROPIC_API_KEY=your-anthropic-key \
./ai-proxy

# Run with OpenAI-to-Anthropic reverse proxy
UPSTREAM_URL=https://llm.chutes.ai/v1/chat/completions \
UPSTREAM_API_KEY=your-key \
./ai-proxy

# Run with both upstreams
UPSTREAM_URL=https://llm.chutes.ai/v1/chat/completions \
UPSTREAM_API_KEY=your-key \
ANTHROPIC_UPSTREAM_URL=https://coding-intl.dashscope.aliyuncs.com/apps/anthropic/v1/messages \
ANTHROPIC_API_KEY=your-anthropic-key \
PORT=3000 \
./ai-proxy
```

## Technical Details

### Tool Call Transformation

The `ToolCallTransformer` implements a 5-state machine (`IDLE → IN_SECTION → READING_ID → READING_ARGS → TRAILING`) that:

1. Buffers incoming reasoning text across SSE chunks
2. Detects special delimiter tokens
3. Extracts function name and arguments
4. Emits properly formatted OpenAI `tool_calls` deltas

### Supported Special Tokens

| Token | Description |
|-------|-------------|
| `<|tool_calls_section_begin|>` | Starts the tool calls section |
| `<|tool_call_begin|>` | Starts a function call (ID/name follows) |
| `<|tool_call_argument_begin|>` | Starts the JSON arguments |
| `<|tool_call_end|>` | Ends the current tool call |
| `<|tool_calls_section_end|>` | Ends the tool calls section |

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
│       ├── anthropic_to_openai.go  # Anthropic-to-OpenAI responses bridge
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
│   └── toolcall/           # Tool call format transformation
│       ├── transformer.go  # State machine transformer
│       ├── parser.go       # Token parsing
│       ├── tokens.go       # Special token definitions
│       ├── formatter.go    # Output formatting
│       ├── anthropic.go    # Anthropic format support
│       ├── openai.go       # OpenAI format support
│       └── responses_transformer.go  # OpenAI Responses API transformer
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

# Format code
go fmt ./...

# Static analysis
go vet ./...
```

## License

MIT