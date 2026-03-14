# Using Alibaba Coding Plan Models with Codex

This guide explains how to configure and use Alibaba Cloud Model Studio's Coding Plan models with Codex CLI.

## Overview

The setup uses a local proxy (`ai-proxy`) to translate between Anthropic API format (used by Codex) and OpenAI-compatible format (used by Alibaba models). This allows Codex to work seamlessly with Qwen, GLM, Kimi, and MiniMax models.

## Configuration Files

### 1. Codex Configuration (`~/.codex/config.toml`)

```toml
# ==========================================
# GLOBAL SETTINGS & DEFAULTS
# ==========================================
model = "glm-5"                          # Default model to use
model_provider = "proxy_ai"              # Provider name (defined below)
model_catalog_json = "/home/vscode/custom_models.json"  # Custom model catalog
model_reasoning_effort = "high"           # Reasoning level: low, medium, high
service_tier = "fast"
hide_agent_reasoning = false              # Show reasoning in UI
show_raw_agent_reasoning = true           # Show raw reasoning content
model_reasoning_summary = "detailed"      # Reasoning summary style

# ==========================================
# PROVIDER DEFINITIONS
# ==========================================

[model_providers.proxy_ai]
name = "proxy"
base_url = "http://host.docker.internal:8080/v1/anthropic-to-openai"
api_key = "sk-cp-dummy"                   # Placeholder key (proxy handles auth)

[projects."/workspaces/your-project"]
trust_level = "trusted"
```

### 2. Custom Models Catalog (`~/custom_models.json`)

The model catalog defines available models and their capabilities:

```json
{
  "fetched_at": "2026-03-13T20:00:00.000000000Z",
  "etag": "W/\"custom-models-anthropic-1\"",
  "client_version": "0.114.0",
  "models": [
    {
      "slug": "qwen3.5-plus",
      "display_name": "Qwen 3.5 Plus",
      "description": "Alibaba Qwen 3.5 Plus via Anthropic endpoint",
      "default_reasoning_level": "medium",
      "supported_reasoning_levels": [
        {"effort": "low", "description": "Fast responses"},
        {"effort": "medium", "description": "Balanced"},
        {"effort": "high", "description": "Deep reasoning"}
      ],
      "shell_type": "shell_command",
      "visibility": "list",
      "supported_in_api": true,
      "priority": 10,
      "base_instructions": "You are a helpful AI assistant.",
      "supports_reasoning_summaries": true,
      "default_reasoning_summary": "auto",
      "support_verbosity": true,
      "default_verbosity": "low",
      "apply_patch_tool_type": "freeform",
      "web_search_tool_type": "text",
      "truncation_policy": {"mode": "bytes", "limit": 10000},
      "supports_parallel_tool_calls": true,
      "supports_image_detail_original": false,
      "context_window": 1000000,
      "effective_context_window_percent": 95,
      "input_modalities": ["text", "image"]
    }
  ]
}
```

## Model Specifications

### Qwen Models (Alibaba)

#### Qwen3.5 Plus

A balanced model with strong reasoning and multimodal capabilities.

| Property | Value |
|----------|-------|
| **Slug** | `qwen3.5-plus` |
| **Context Window** | 1,000,000 tokens |
| **Max Output** | 65,536 tokens |
| **Max CoT** | 81,920 tokens |
| **Reasoning** | Yes (enabled by default) |
| **Tool Calling** | Yes |
| **Input Modalities** | text, image, video |
| **Output Modalities** | text |
| **Open Weights** | No |

**Pricing** (per 1M tokens):
| Input Tokens | Input Cost | Output Cost |
|--------------|------------|-------------|
| 0 - 256K | $0.40 | $2.40 |
| 256K - 1M | $0.50 | $3.00 |

**Notes**: Thinking mode is enabled by default.

---

#### Qwen3 Max

The most powerful Qwen model for complex tasks, with thinking mode support.

| Property | Value |
|----------|-------|
| **Slug** | `qwen3-max` |
| **Context Window** | 262,144 tokens |
| **Max Output** | 32,768 tokens |
| **Max CoT** | 81,920 tokens |
| **Reasoning** | Yes |
| **Tool Calling** | Yes |
| **Input Modalities** | text |
| **Output Modalities** | text |
| **Open Weights** | No |

**Pricing** (per 1M tokens):
| Input Tokens | Input Cost | Output Cost |
|--------------|------------|-------------|
| 0 - 32K | $1.20 | $6.00 |
| 32K - 128K | $2.40 | $12.00 |
| 128K - 252K | $3.00 | $15.00 |

**Notes**: Thinking mode integrates web search, web extractor, and code interpreter tools.

---

#### Qwen3 Coder Plus

Advanced coding model with agent capabilities and maximum context.

| Property | Value |
|----------|-------|
| **Slug** | `qwen3-coder-plus` |
| **Context Window** | 1,000,000 tokens |
| **Max Output** | 65,536 tokens |
| **Reasoning** | No |
| **Tool Calling** | Yes |
| **Input Modalities** | text |
| **Output Modalities** | text |
| **Open Weights** | Yes |

**Pricing** (per 1M tokens):
| Input Tokens | Input Cost | Output Cost |
|--------------|------------|-------------|
| 0 - 32K | $1.00 | $5.00 |
| 32K - 128K | $1.80 | $9.00 |
| 128K - 256K | $3.00 | $15.00 |
| 256K - 1M | $6.00 | $60.00 |

**Notes**: Supports context cache (implicit: 20%, explicit: 10% pricing). Best for large codebases.

---

#### Qwen3 Coder Next

Next-generation coding model with large context support.

| Property | Value |
|----------|-------|
| **Slug** | `qwen3-coder-next` |
| **Context Window** | 262,144 tokens |
| **Max Output** | 65,536 tokens |
| **Reasoning** | No |
| **Tool Calling** | Yes |
| **Input Modalities** | text |
| **Output Modalities** | text |
| **Open Weights** | Yes |

**Pricing** (per 1M tokens):
| Input Tokens | Input Cost | Output Cost |
|--------------|------------|-------------|
| 0 - 32K | $0.30 | $1.50 |
| 32K - 128K | $0.50 | $2.50 |
| 128K - 256K | $0.80 | $4.00 |

**Notes**: Most cost-effective for coding tasks.

---

### GLM Models (Zhipu AI)

Hybrid reasoning models designed specifically for agents.

**Note**: GLM models are Chinese mainland only (Beijing region).

#### GLM-5

Latest flagship model from Zhipu AI with SOTA coding performance.

| Property | Value |
|----------|-------|
| **Slug** | `glm-5` |
| **Context Window** | 202,752 tokens |
| **Max Output** | 16,384 tokens |
| **Max CoT** | 32,768 tokens |
| **Reasoning** | Yes |
| **Tool Calling** | Yes |
| **Input Modalities** | text |
| **Output Modalities** | text |
| **Open Weights** | No |

**Pricing** (per 1M tokens):
| Input Tokens | Input Cost | Output Cost |
|--------------|------------|-------------|
| 0 - 32K | $0.573 | $2.58 |
| 32K - 166K | $0.86 | $3.154 |

**Notes**: Charges same rate for thinking and non-thinking modes. Excellent for coding tasks.

---

#### GLM-4.7

Previous generation GLM model with strong reasoning capabilities.

| Property | Value |
|----------|-------|
| **Slug** | `glm-4.7` |
| **Context Window** | 169,984 tokens |
| **Max Output** | ~16,384 tokens |
| **Reasoning** | Yes |
| **Tool Calling** | Yes |
| **Input Modalities** | text |
| **Output Modalities** | text |
| **Open Weights** | No |

**Pricing** (per 1M tokens):
| Input Tokens | Input Cost | Output Cost |
|--------------|------------|-------------|
| 0 - 32K | $0.431 | $2.007 |
| 32K - 166K | $0.574 | $2.294 |

**Notes**: Charges same rate for thinking and non-thinking modes.

---

### Kimi Models (Moonshot AI)

Large language models optimized for coding and tool calling.

**Note**: Kimi models are Chinese mainland only (Beijing region).

#### Kimi K2.5

Advanced multimodal model with strong coding and visual understanding capabilities.

| Property | Value |
|----------|-------|
| **Slug** | `kimi-k2.5` |
| **Context Window** | 262,144 tokens |
| **Max Output** | 32,768 tokens |
| **Max CoT** | 32,768 tokens |
| **Reasoning** | Yes |
| **Tool Calling** | Yes |
| **Input Modalities** | text, image |
| **Output Modalities** | text |
| **Open Weights** | No |

**Pricing** (per 1M tokens):
| Mode | Input Cost | Output Cost |
|------|------------|-------------|
| Thinking | $0.574 | $3.011 |
| Non-thinking | $0.574 | $3.011 |

**Notes**: Supports both thinking and non-thinking modes. Good for visual understanding tasks.

---

### MiniMax Models

Text models with strong code generation capabilities.

#### MiniMax M2.5

Latest text model from MiniMax with peak performance for complex tasks.

| Property | Value |
|----------|-------|
| **Slug** | `MiniMax-M2.5` |
| **Context Window** | 200,000 tokens |
| **Max Output** | 128,000 tokens |
| **Reasoning** | Yes |
| **Tool Calling** | Yes |
| **Input Modalities** | text |
| **Output Modalities** | text |
| **Open Weights** | No |

**Features**:
- Optimized for code generation and refactoring
- Polyglot code mastery
- Precision code refactoring
- Advanced reasoning
- Real-time streaming

---

## Deployment Regions

| Model Family | Region |
|--------------|--------|
| Qwen (most) | International, Global, US, Chinese Mainland |
| GLM | Chinese Mainland only (Beijing) |
| Kimi | Chinese Mainland only (Beijing) |
| MiniMax | Varies by configuration |

## Quick Start

### 1. Start the Proxy

```bash
# Build and run the proxy
cd /workspaces/kimi-k2.5-fix-proxy
go build -o ai-proxy . && ./ai-proxy

# Or with custom settings
PORT=8080 UPSTREAM_API_KEY=your-alibaba-key ./ai-proxy
```

### 2. Configure Codex

Create or edit `~/.codex/config.toml`:

```toml
model = "qwen3-coder-plus"
model_provider = "proxy_ai"
model_catalog_json = "/home/vscode/custom_models.json"
model_reasoning_effort = "high"

[model_providers.proxy_ai]
name = "proxy"
base_url = "http://localhost:8080/v1/anthropic-to-openai"
api_key = "sk-cp-dummy"
```

### 3. Run Codex

```bash
# Use default model from config
codex

# Override model for this session
codex --model qwen3-max

# With specific reasoning effort
codex --model glm-5 --reasoning-effort high
```

## Model Selection Guide

### For Coding Tasks

| Task Type | Recommended Model | Why |
|-----------|-------------------|-----|
| Large codebase analysis | `qwen3-coder-plus` | 1M context window |
| Fast coding iterations | `qwen3-coder-next` | Lower cost, good performance |
| Complex reasoning | `glm-5` | SOTA coding benchmarks |
| Chinese code comments | `glm-5`, `kimi-k2.5` | Better Chinese understanding |

### For General Tasks

| Task Type | Recommended Model | Why |
|-----------|-------------------|-----|
| Multimodal (images) | `qwen3.5-plus`, `kimi-k2.5` | Image input support |
| Deep reasoning | `qwen3-max`, `glm-5` | Strong reasoning capabilities |
| Cost-effective | `qwen3-coder-next` | Lower pricing tiers |

## Reasoning Configuration

Control reasoning behavior in `config.toml`:

```toml
# Reasoning effort level
model_reasoning_effort = "high"  # low, medium, high

# Show/hide reasoning in UI
hide_agent_reasoning = false
show_raw_agent_reasoning = true

# Reasoning summary style
model_reasoning_summary = "detailed"  # auto, concise, detailed
```

Or via command line:

```bash
codex --reasoning-effort high
```

## Environment Variables

The proxy supports these environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `PORT` | Proxy server port | `8080` |
| `UPSTREAM_API_KEY` | Alibaba API key | (required) |
| `SSELOG_DIR` | Directory for request logs | (disabled) |
| `UPSTREAM_BASE_URL` | Upstream API URL | Alibaba endpoint |

## Switching Models

### Method 1: Command Line Flag

```bash
codex --model qwen3-max
codex --model glm-5
codex --model kimi-k2.5
```

### Method 2: Edit Config File

Change the `model` field in `~/.codex/config.toml`:

```toml
model = "qwen3-coder-plus"  # Change this value
```

### Method 3: Environment Variable

```bash
CODEX_MODEL=qwen3-max codex
```

## Troubleshooting

### Proxy Not Running

**Error**: Connection refused to `localhost:8080`

**Solution**: Start the proxy first:
```bash
./ai-proxy
```

### Model Not Found

**Error**: Model not in catalog

**Solution**: Check `custom_models.json` contains the model slug, or use a valid model name.

### API Key Issues

**Error**: Authentication failed

**Solution**: Set `UPSTREAM_API_KEY` with your Alibaba Cloud API key:
```bash
UPSTREAM_API_KEY=sk-xxx ./ai-proxy
```

### Reasoning Not Working

**Symptom**: Model doesn't show extended thinking

**Solution**: 
1. Ensure model supports reasoning (check `supports_reasoning_summaries: true`)
2. Set `model_reasoning_effort = "high"` in config
3. Use `--reasoning-effort high` flag

## Cost Optimization

### Tiered Pricing

Most models have tiered pricing based on context usage:

```toml
# Example: qwen3-coder-plus pricing
# 0-32K:    $1.00 input / $5.00 output per 1M tokens
# 32K-128K: $1.80 input / $9.00 output per 1M tokens
# 128K-256K: $3.00 input / $15.00 output per 1M tokens
# 256K-1M:  $6.00 input / $60.00 output per 1M tokens
```

### Tips

1. **Use smaller context when possible**: Stay in lower pricing tiers
2. **Choose appropriate models**: `qwen3-coder-next` is cheaper than `qwen3-coder-plus`
3. **Disable reasoning for simple tasks**: Set `model_reasoning_effort = "low"`
4. **Use context caching**: Available on some models (reduces cost 10-20%)

## Advanced Configuration

### Multiple Providers

Define multiple providers for different use cases:

```toml
[model_providers.proxy_ai]
name = "proxy"
base_url = "http://localhost:8080/v1/anthropic-to-openai"
api_key = "sk-cp-dummy"

[model_providers.direct_anthropic]
name = "anthropic"
api_key = "sk-ant-xxx"
```

### Project-Specific Settings

Override settings per project:

```toml
[projects."/workspaces/my-project"]
trust_level = "trusted"
model = "qwen3-coder-plus"

[projects."/workspaces/another-project"]
trust_level = "trusted"
model = "glm-5"
```

## Related Documentation

- [Tool Call Design](./TOOL_CALL_DESIGN.md) - How tool calling works
- [Logging](./LOGGING.md) - Request/response capture and debugging
