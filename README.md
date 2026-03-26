# AI Proxy

A Go-based HTTP proxy that enables Codex with Alibaba and reliable Kimi. Provides format transformation, tool call normalization, and seamless integration between OpenAI, Anthropic, and other API formats.

## What This Project Solves

**Model mapping** and **provider aggregation** solve the routing problem. Applications need to route different models to different providers—Alibaba's Qwen for cost, Anthropic's Claude for reasoning, Kimi for specialized tasks. Without a routing layer, provider selection logic gets hardcoded throughout codebases. This proxy makes "use qwen3-max" a config change, not a code refactor.

**Protocol translation** handles format incompatibility:

| Client Format | OpenAI Provider | Anthropic Provider |
|---------------|-----------------|---------------------|
| OpenAI Chat | ✓ Pass-through | Chat → Messages → Chat |
| Anthropic Messages | Messages → Chat → Messages | ✓ Pass-through |
| OpenAI Responses | Responses → Chat → Chat | Responses → Messages → Responses |

Each conversion handles message structure, tool call formats, streaming semantics, and edge cases around system prompts and multi-modal inputs.

**Alibaba model access** unlocks cost-effective alternatives. Alibaba's Qwen and hosted Kimi models only support OpenAI-compatible endpoints. Codex users can't access them without rewriting integration code. This proxy acts as a universal adapter: Codex talks to Alibaba through OpenAI responses protocol.

**Kimi tool-call extraction** makes Kimi-K2.5/K2 usable. These models embed tool calls in reasoning tokens using proprietary delimiters (`<|tool_calls_section_begin|>`, `<|tool_call_begin|>`) rather than standard formats. Without real-time extraction from the SSE `delta.reasoning` stream, agents receive malformed tool calls and function-calling breaks. This proxy's state-machine parser extracts and reformats tool calls into standard OpenAI/Anthropic structures—making Kimi viable for production agents.

**Server-side web search** enables real-time information retrieval. Models can use the `web_search` tool to fetch current information from the web. The proxy intercepts `server_tool_use` blocks, executes searches via Exa/Brave/DuckDuckGo, and injects results into the response stream—matching Anthropic's built-in web search behavior.

## Features

- **Multi-format support**: OpenAI Chat Completions, Anthropic Messages, and OpenAI Responses API
- **Bidirectional conversion**: Convert between OpenAI and Anthropic formats in both directions
- **Tool call normalization**: Transforms Kimi-K2.5/K2's proprietary tool call format into standard formats
- **Server-side web search**: Execute web searches via Exa, Brave, or DuckDuckGo when models use the `web_search` tool
- **Streaming support**: Real-time SSE streaming with format transformation
- **Request capture**: Optional logging of all requests/responses for debugging
- **Model-based routing**: Route requests to different providers based on model name

## User Guide

### Installation

```bash
# Build from source
go build -o ai-proxy .

# Or install via Makefile
make build
make install  # Installs binary to ~/.local/bin and config to ~/.config/ai-proxy/config.json
```

### Configuration

The proxy requires a JSON configuration file. By default, it searches for `config.json` in XDG standard locations:

1. `--config-file` flag or `CONFIG_FILE` env
2. `$XDG_CONFIG_HOME/ai-proxy/config.json`
3. `$HOME/.config/ai-proxy/config.json`
4. `$XDG_CONFIG_DIRS/ai-proxy/config.json` (default: `/etc/xdg`)

#### Configuration Example

```json
{
  "providers": [
    {
      "name": "alibaba",
      "endpoints": {
        "openai": "https://dashscope.aliyuncs.com/compatible-mode/v1"
      },
      "envApiKey": "ALIBABA_API_KEY"
    },
    {
      "name": "anthropic",
      "endpoints": {
        "anthropic": "https://api.anthropic.com/v1"
      },
      "envApiKey": "ANTHROPIC_API_KEY"
    }
  ],
  "models": {
    "qwen3-max": {
      "provider": "alibaba",
      "model": "qwen3-max-2026-01-23",
      "kimi_tool_call_transform": false
    },
    "kimi-k2.5": {
      "provider": "alibaba",
      "model": "kimi-k2.5",
      "kimi_tool_call_transform": true
    }
  },
  "fallback": {
    "enabled": true,
    "provider": "alibaba",
    "model": "{model}"
  },
  "websearch": {
    "enabled": true,
    "provider": "exa",
    "exa_api_key": "${EXA_API_KEY}",
    "max_results": 10,
    "timeout": 30
  }
}
```

#### Provider Configuration

| Field | Description |
|-------|-------------|
| `name` | Unique identifier for the provider |
| `endpoints` | Map of protocol names to endpoint URLs: `"openai"`, `"anthropic"`, `"responses"` |
| `default` | Default protocol when multiple endpoints configured (optional) |
| `apiKey` | Direct API key (optional) |
| `envApiKey` | Environment variable name for API key |

#### Model Configuration

| Field | Description |
|-------|-------------|
| `provider` | Provider name to route requests to |
| `model` | Actual model identifier on the provider |
| `type` | Output protocol: `"openai"`, `"anthropic"`, `"responses"`, or `"auto"` (default: use provider default) |
| `kimi_tool_call_transform` | Enable Kimi tool-call extraction (default: `false`) |
| `glm5_tool_call_transform` | Enable GLM-5 XML tool-call extraction (default: `false`) |
| `reasoning_split` | Enable separate reasoning output for supported models (default: `false`) |

#### Web Search Configuration

| Field | Description |
|-------|-------------|
| `enabled` | Enable/disable web search service (default: `false`) |
| `provider` | Search backend: `"exa"`, `"brave"`, or `"ddg"` |
| `exa_api_key` | API key for Exa.ai (required if provider is `exa`) |
| `brave_api_key` | API key for Brave Search (required if provider is `brave`) |
| `max_results` | Maximum search results per query (default: `10`) |
| `timeout` | Search request timeout in seconds (default: `30`) |

**Search Backends:**

| Provider | API Key Required | Features |
|----------|-----------------|----------|
| `exa` | Yes ([exa.ai](https://exa.ai)) | High-quality results, content extraction |
| `brave` | Yes ([brave.com](https://brave.com/search/api)) | Fast, comprehensive web index |
| `ddg` | No | Free, no API key needed, limited results |

### Running the Proxy

```bash
# Run with config in XDG default location (~/.config/ai-proxy/config.json)
./ai-proxy

# Run with explicit config file
./ai-proxy --config-file /path/to/config.json

# Run with environment variable
CONFIG_FILE=/path/to/config.json ./ai-proxy

# Run with additional options
./ai-proxy --port 8080 --sse-log-dir ./logs

# Run with conversation store tuning
./ai-proxy --conversation-store-size 2000 --conversation-store-ttl 48h
```

### Command-Line Options

| Flag | Environment | Default | Description |
|------|-------------|---------|-------------|
| `--config-file` | `CONFIG_FILE` | XDG discovery | Path to configuration file |
| `--port` | `PORT` | `8080` | Server listen port |
| `--sse-log-dir` | `SSELOG_DIR` | (disabled) | Directory for request logging |
| `--conversation-store-size` | - | `1000` | Max cached conversations |
| `--conversation-store-ttl` | - | `24h` | Conversation cache TTL |

### Using with Codex

Set the OpenAI base URL and API key:

```bash
export OPENAI_BASE_URL=http://localhost:8080/v1
export OPENAI_API_KEY=your-provider-api-key  # Will be passed through to upstream
```

Then configure `~/.codex/config.toml`:

```toml
model = "qwen3-max"  # Routes to Alibaba Qwen3

[mcp]
enabled = false
```

### API Endpoints

| Method | Path | Description |
|--------|------|-------------|
| GET | `/health` | Health check |
| GET | `/v1/models` | List available models |
| POST | `/v1/chat/completions` | OpenAI-compatible chat completions |
| POST | `/v1/messages` | Anthropic Messages API |
| POST | `/v1/responses` | OpenAI Responses API |

## Web Search Tool

The proxy supports Anthropic-style server-side web search. When enabled, models can use the `web_search` tool to fetch real-time information.

### How It Works

```
Client Request with tools: [{type: "web_search_20250305", name: "web_search"}]
         │
         ▼
Upstream LLM generates response:
  - "I'll search for..."
  - server_tool_use: {name: "web_search", input: {query: "..."}}
         │
         ▼
Proxy intercepts server_tool_use:
  1. Detects web_search tool call
  2. Executes search via configured backend (Exa/Brave/DDG)
  3. Injects synthetic web_search_tool_result event
         │
         ▼
Client receives:
  - Text content
  - server_tool_use block
  - web_search_tool_result block (injected by proxy)
  - Final answer with search results
```

### Usage Example

**Request:**
```json
{
  "model": "claude-sonnet-4-6",
  "max_tokens": 1024,
  "messages": [
    {"role": "user", "content": "What's the latest news about GPT-5?"}
  ],
  "tools": [
    {"type": "web_search_20250305", "name": "web_search"}
  ]
}
```

**Response (streaming):**
```
event: content_block_start
data: {"type": "content_block_start", "index": 0, "content_block": {"type": "text", "text": ""}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 0, "delta": {"type": "text_delta", "text": "I'll search for the latest news about GPT-5."}}

event: content_block_start
data: {"type": "content_block_start", "index": 1, "content_block": {"type": "server_tool_use", "id": "search_001", "name": "web_search"}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 1, "delta": {"type": "input_json_delta", "partial_json": "{\"query\":\"latest news GPT-5 2025\"}"}}

event: content_block_stop
data: {"type": "content_block_stop", "index": 1}

event: content_block_start
data: {"type": "content_block_start", "index": 2, "content_block": {"type": "web_search_tool_result", "tool_use_id": "search_001", "content": [...]}}

event: content_block_stop
data: {"type": "content_block_stop", "index": 2}

event: content_block_start
data: {"type": "content_block_start", "index": 3, "content_block": {"type": "text", "text": ""}}

event: content_block_delta
data: {"type": "content_block_delta", "index": 3, "delta": {"type": "text_delta", "text": "Based on the search results..."}}
```

### Environment Variables

Set API keys via environment variables:

```bash
export EXA_API_KEY=your-exa-api-key
export BRAVE_API_KEY=your-brave-api-key
```

Reference them in config using `${VAR_NAME}` syntax:

```json
{
  "websearch": {
    "enabled": true,
    "provider": "exa",
    "exa_api_key": "${EXA_API_KEY}"
  }
}
```

## Request Flow

```
┌────────────┐
│   Client   │
└─────┬──────┘
      │ 1. Downstream TX
      ▼
┌────────────┐
│   Proxy    │──► Capture (optional)
└─────┬──────┘
      │ 2. Upstream TX
      ▼
┌────────────┐
│   LLM API  │
└─────┬──────┘
      │ 3. Upstream RX
      ▼
┌────────────┐
│   Proxy    │──► Transform (format, tool calls, web search)
└─────┬──────┘
      │ 4. Downstream RX
      ▼
┌────────────┐
│   Client   │
└────────────┘
```

1. **Downstream TX** - Client request to proxy
2. **Upstream TX** - Proxy request to LLM API
3. **Upstream RX** - LLM API response to proxy
4. **Downstream RX** - Proxy response to client

## Project Structure

```
ai-proxy/
├── main.go                     # Entry point, server initialization
├── api/                        # HTTP server and routing
│   ├── server.go               # Server setup and route registration
│   ├── middleware.go           # Capture middleware
│   └── handlers/               # HTTP request handlers
│       ├── interface.go        # Handler interface definition
│       ├── health.go           # Health check endpoint
│       ├── models.go           # Models listing endpoint
│       ├── completions.go      # OpenAI chat completions
│       ├── messages.go         # Anthropic messages
│       ├── responses.go        # OpenAI Responses API
│       ├── count_tokens.go     # Token counting endpoint
│       └── response_recorder.go # Response recording utilities
├── config/                     # Configuration loading
│   ├── cli.go                  # CLI flag parsing, XDG discovery
│   ├── config.go               # Config struct and accessors
│   ├── loader.go               # Config file loading and validation
│   └── schema.go               # JSON schema definitions
├── router/                     # Model routing
│   └── router.go               # Model-to-provider resolution
├── convert/                    # Format conversion
│   ├── interface.go            # Converter interface
│   ├── common.go               # Shared conversion utilities
│   ├── param_convert.go        # Parameter conversion
│   ├── finish_reason.go        # Finish reason mapping
│   ├── anthropic_to_chat.go    # Anthropic → OpenAI Chat
│   ├── anthropic_to_responses.go # Anthropic → OpenAI Responses
│   ├── chat_to_anthropic.go    # OpenAI Chat → Anthropic
│   ├── chat_to_responses.go    # OpenAI Chat → Responses
│   ├── responses_to_anthropic.go # Responses → Anthropic
│   ├── responses_to_anthropic_streaming.go # Streaming variant
│   └── responses_to_chat.go    # Responses → OpenAI Chat
├── transform/                  # Response transformation
│   ├── interface.go            # Transformer interface
│   ├── passthrough.go          # Pass-through transformer
│   ├── sse_writer.go           # SSE streaming writer
│   ├── toolcall/               # Tool call format transformation
│   │   ├── parser.go           # Token parsing
│   │   ├── tokens.go           # Special token definitions
│   │   ├── state.go            # State machine
│   │   ├── formatter.go        # Output formatting
│   │   ├── common.go           # Shared utilities
│   │   ├── openai.go           # OpenAI format support
│   │   ├── anthropic.go        # Anthropic format support
│   │   ├── openai_transformer.go    # OpenAI format transformer
│   │   ├── anthropic_transformer.go # Anthropic format transformer
│   │   └── responses_transformer.go # Responses API transformer
│   └── websearch/              # Web search transformation
│       ├── transformer.go      # SSE transformer for web search interception
│       └── transformer_test.go # Tests for web search transformer
├── websearch/                  # Web search service
│   ├── service.go              # Main service with backend selection
│   ├── adapter.go              # Adapter for transformer integration
│   ├── exa.go                  # Exa.ai backend
│   ├── brave.go                # Brave Search backend
│   └── ddg.go                  # DuckDuckGo backend
├── types/                      # Type definitions
│   ├── openai.go               # OpenAI Chat API types
│   ├── openai_responses.go     # OpenAI Responses API types
│   ├── anthropic.go            # Anthropic API types
│   ├── websearch.go            # Web search types
│   └── sse.go                  # Server-Sent Events types
├── proxy/                      # Upstream API client
│   ├── client.go               # HTTP client for upstream APIs
│   └── request.go              # Request building utilities
├── tokens/                     # Token counting
│   └── counter.go              # Token counter implementation
├── conversation/               # Conversation storage
│   └── store.go                # In-memory conversation cache
├── capture/                    # Request/response capture
│   ├── storage.go              # Log storage management
│   ├── writer.go               # JSON log writer
│   ├── recorder.go             # Request/response recording
│   └── context.go              # Context utilities
└── logging/                    # Logging utilities
    └── logging.go
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
| `<\|tool_calls_section_begin\|>` | Starts the tool calls section |
| `<\|tool_call_begin\|>` | Starts a function call (ID/name follows) |
| `<\|tool_call_argument_begin\|>` | Starts the JSON arguments |
| `<\|tool_call_end\|>` | Ends the current tool call |
| `<\|tool_calls_section_end\|>` | Ends the tool calls section |

### Web Search Interception

The `WebSearchTransformer` wraps the SSE response stream and:

1. Monitors for `content_block_start` events with `type: "server_tool_use"` and `name: "web_search"`
2. Buffers the streaming JSON input via `input_json_delta` events
3. On `content_block_stop`, executes the search via the configured backend
4. Injects synthetic `web_search_tool_result` events into the stream
5. Handles multiple concurrent web search blocks using a map keyed by block ID

### Format Conversions

| Endpoint | Provider Type | Request Transform | Response Transform |
|----------|---------------|-------------------|-------------------|
| `/v1/chat/completions` | OpenAI | None (pass-through) | Tool call normalization* |
| `/v1/chat/completions` | Anthropic | OpenAI → Anthropic | Anthropic → OpenAI |
| `/v1/messages` | Anthropic | None (pass-through) | Tool call normalization* |
| `/v1/messages` | OpenAI | Anthropic → OpenAI | OpenAI → Anthropic |
| `/v1/responses` | OpenAI | Responses → Chat | Chat → Responses |
| `/v1/responses` | Anthropic | Responses → Anthropic | Anthropic → Responses |

*Tool call normalization only applies when `<model>_tool_call_transform: true` is set for the model.

## License

GPL v3
