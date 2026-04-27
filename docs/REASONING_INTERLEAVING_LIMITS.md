# Reasoning Interleaving: Known Limitations

## Context

The proxy converts between three API formats, each with a different representation
for reasoning/thinking content:

| Format | Reasoning representation |
|--------|------------------------|
| **Responses API** | Standalone `{"type":"reasoning","summary":[...]}` items (type `OutputItem`) interleaved in a flat `input`/`output` array |
| **Anthropic Messages** | `{"type":"thinking","thinking":"..."}` blocks (field `ContentBlock.Thinking`) interleaved within a message's `content` array |
| **Chat Completions** | Single `reasoning_content` string field (`Message.ReasoningContent` in `types/openai.go`) per message |

Anthropic and Responses both support **interleaved** reasoning — a single assistant
turn can contain multiple thinking/reasoning blocks separated by text and tool
calls:

```
thinking:"Step 1" → text:"Looking up..." → tool_use:"search" → thinking:"Step 2" → text:"Result."
```

Chat Completions flattens this into one `reasoning_content` per message.

## Problem: Anthropic → Chat loses interleaving

When converting Anthropic (or Responses) to Chat Completions, interleaved reasoning
blocks must be collapsed into a single `reasoning_content` string.

**Files involved:**
- `convert/anthropic_to_chat.go` — `convertAnthropicContentBlocksToOpenAI()`, `convertAnthropicMessageToOpenAI()`
- `convert/responses_to_chat.go` — `groupProcessor.handleReasoningItem()`, `groupProcessor.pendingReasoning`

**Input** (Anthropic blocks in order):
```json
[
  {"type":"thinking","thinking":"Step 1"},
  {"type":"text","text":"Looking up..."},
  {"type":"tool_use","id":"t1","name":"search","input":{}},
  {"type":"thinking","thinking":"Step 2"},
  {"type":"text","text":"Result."}
]
```

**Output** (Chat message):
```json
{
  "role": "assistant",
  "content": "Looking up...\nResult.",
  "reasoning_content": "Step 1\nStep 2",
  "tool_calls": [{"id":"t1","type":"function","function":{"name":"search","arguments":"{}"}}]
}
```

The mapping of *which* reasoning preceded *which* text is lost.

### Why not split into multiple assistant messages?

Splitting `thinking → text → tool_use → thinking → text` into two assistant
messages:

```json
[
  {"role":"assistant","content":"Looking up...","reasoning_content":"Step 1","tool_calls":[...]},
  {"role":"assistant","content":"Result.","reasoning_content":"Step 2"}
]
```

produces **consecutive assistant messages** with no `user` or `tool` message
between them. The Chat Completions API expects `user ↔ assistant` alternation
(with `tool` allowed between). Consecutive assistants are rejected or
mishandled across providers.

### Why not inject synthetic user messages?

The Chat → Anthropic converter already injects empty placeholder messages when
consecutive same-role messages appear:

- `convert/chat_to_anthropic.go` — `normalizeAnthropicMessages()`

This exists because Anthropic's API **rejects** consecutive same-role messages
outright. Injecting synthetic messages for Anthropic → Chat would cause:

- Providers may interpret empty user messages as prompts to generate new output
- Pollutes conversation history with artifacts the model didn't produce
- Up/downstream models see messages that don't match the user's intent

## What this means in practice

### For correctness

The critical requirement (from the original DeepSeek bug documented in
`docs/REASONING_CONVERSION_FIX.md`) is that `reasoning_content` be **present at
all** when any reasoning exists in the conversation. DeepSeek errors if the key
is missing from any assistant message in a thinking-mode multi-turn
conversation. Interleaving fidelity is not required for correctness.

The backfill logic lives in:
- `convert/responses_to_chat.go` — `ensureReasoningOnAllAssistants()`

### For providers that require interleaving

No known provider requires interleaved reasoning in the Chat Completions format.
Providers that support reasoning (DeepSeek, MiniMax, OpenAI o1/o3) all accept a
single `reasoning_content` per message. If a provider emerges that needs
per-block reasoning, it must use Anthropic or Responses endpoints directly.

## The `normalizeAnthropicMessages` safety net

**File:** `convert/chat_to_anthropic.go`
**Function:** `normalizeAnthropicMessages()`
**Called from:** `convertOpenAIMessagesToAnthropic()`

This function injects empty user/assistant placeholders when consecutive
same-role messages appear in the output. It exists because:

1. `convertOpenAIMessage()` can return **multiple** `MessageInput` values from a
   single OpenAI message (content block splitting via `convertContentBlocks()`).
2. Tool result grouping (`chatAppendToolResultToMessage()`,
   `chatCreateToolResultMessage()`) can merge tool messages into user messages,
   potentially creating `user, user` sequences.
3. Anthropic's API **rejects** consecutive same-role messages.

It's a post-processing normalization pass, not evidence of malformed input.

Helper functions in the same file:
- `emptyAnthropicMessage()` — creates `{Role, Content:[]interface{}{}}`
- `chatIsToolResultMessage()` — detects tool-result user messages
- `chatAppendToolResultToMessage()` — appends tool results in-place
- `chatCreateToolResultMessage()` — creates a new tool-result message

## Conversion coverage matrix

| Direction | Converter type | File | Key function(s) | Reasoning? | Interleaving? |
|-----------|---------------|------|-----------------|------------|---------------|
| Chat → Anthropic | Request | `chat_to_anthropic.go` | `convertOpenAIMessage()`, `applyReasoningToAnthropicMessage()`, `prependReasoningToAnthropicMessage()` | ✅ | N/A (single `ReasoningContent` per message) |
| Chat → Anthropic | Streaming response | `chat_to_anthropic.go` | `ChatToAnthropicTransformer.handleChunk()`, `emitThinkingDelta()`, `emitThinkingStart()` | ✅ | ✅ (block-by-block) |
| Anthropic → Chat | Request | `anthropic_to_chat.go` | `convertAnthropicContentBlocksToOpenAI()`, `convertAnthropicMessageToOpenAI()` | ✅ | ❌ Format limitation |
| Anthropic → Chat | Streaming response | `anthropic_to_chat_streaming.go` | `AnthropicToChatStreamingConverter.handleThinkingDelta()` | ✅ | ❌ Format limitation |
| Anthropic → Responses | Request | `anthropic_to_responses.go` | `anthropicAssistantBlocksToResponsesItems()` | ✅ | ✅ (in-place items) |
| Anthropic → Responses | Streaming response | `anthropic_to_responses_streaming.go` | (handles `summary` on reasoning output items) | ✅ | ✅ (in-place items) |
| Responses → Anthropic | Request | `responses_to_anthropic.go` | reasoning→thinking conversion via `ExtractReasoningText()` in `common.go` | ✅ | ✅ (in-place blocks) |
| Responses → Chat | Request | `responses_to_chat.go` | `handleReasoningItem()`, `ensureReasoningOnAllAssistants()`, `ExtractReasoningText()` | ✅ | ❌ Format limitation |

## Related files

| File | Role |
|------|------|
| `types/openai.go` | `Message.ReasoningContent *string` — the Chat Completions reasoning field |
| `types/openai_responses.go` | `InputItem.Summary interface{}` — Responses API reasoning items |
| `types/anthropic.go` | `ContentBlock.Thinking string` — Anthropic thinking blocks |
| `convert/common.go` | `ExtractReasoningText()` — shared helper for extracting reasoning summary text |
| `docs/REASONING_CONVERSION_FIX.md` | Original fix for DeepSeek reasoning preservation |
