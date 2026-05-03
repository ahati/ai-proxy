# Fix: WS Bridge Truncation & Duplicate Log IDs

**Date**: 2026-05-04
**Status**: Draft
**Branch**: kimi-k2.5-fix-proxy

## Problem Summary

Two bugs discovered in the WebSocket Responses API bridge:

1. **Conversation history truncation** (multi-turn): When codex sends `"store": false` in `response.create` events, the proxy skips both conversation chain lookup AND conversation storage. Upstream REST models receive only new messages without full history, losing all context on followup turns.

2. **In-memory log duplicate IDs**: The `CaptureContext`/`Recorder` is created once per WS connection and reused for all turns. All turns share the same 8-char UUID. Additionally, `generateConnectionID()` uses `time.Now().UnixNano()` which can collide.

## Root Causes

| # | Root Cause | Location | Evidence |
|---|-----------|----------|----------|
| 1a | `previous_response_id` lookup gated on `shouldStore` | `convert/responses_to_chat.go:68` | `if req.PreviousResponseID != "" && c.shouldStore` |
| 1b | Conversation storage gated on `shouldStore` | `convert/chat_to_responses.go:958` | `if !t.shouldStore { return }` |
| 1c | `Flush()` `continue` skips capture when `clientConn` nil | `api/websocket/connection.go:652-654` | `continue` inside nil guard |
| 1d | Only first `data:` line parsed per SSE event | `api/websocket/connection.go:597-602` | `break` after first match |
| 2a | Recorder reused across turns | `api/websocket/handler.go:70-73` | Single `cc.Recorder` for all turns |
| 2b | Nanosecond timestamp as ID | `api/websocket/connection.go:562-563` | `time.Now().UnixNano()` |

## Design

### Fix 1a/1b: In-memory conversation accumulation + persistence for cross-connection resume

**Core insight**: One WS connection = one ongoing conversation. The `Connection` should hold the growing conversation state in-memory, eliminating needless chain walks for within-connection turns. The conversation store is used only for cross-connection resume (when `codex exec resume` opens a new WS connection).

**Two sources of conversation history**:

| Scenario | History Source |
|----------|---------------|
| Turn N within same WS connection | `Connection.history` (in-memory accumulation) |
| New connection with `previous_response_id` | `conversation.DefaultStore` (chain walk) |

**Two concerns for `store` flag**:

- **Operational**: The bridge MUST maintain conversation state internally to reconstruct full message history for upstream REST APIs, regardless of what `store` says. Within a connection, history is in-memory. Across connections, history comes from the store.
- **API semantic**: The `store` flag controls whether a response is *persisted* (visible via `/v1/responses/:id` CRUD endpoints), per OpenAI's spec.

These concerns are separated: conversations are always stored in the `DefaultStore` (needed for cross-connection resume), with a `Persisted` flag controlled by `store` that governs CRUD visibility and cleanup on close.

#### Changes

**`conversation/store.go`** — Add `Persisted` field and `DeleteFromDefault()`:
```go
type Conversation struct {
    // ... existing fields ...
    // Persisted indicates whether this conversation should be visible via
    // the Responses API CRUD endpoints. Controlled by the OpenAI `store` flag.
    // When false (store:false), the conversation is kept only for bridge
    // operation and is cleaned up when the WebSocket connection closes.
    Persisted bool
}

// DeleteFromDefault removes a conversation from the default store by ID.
func DeleteFromDefault(id string) {
    if DefaultStore == nil {
        return
    }
    DefaultStore.Delete(id)
}
```

**`api/websocket/connection.go`** — Add in-memory history accumulation:
```go
type Connection struct {
    // ... existing fields ...
    
    // history accumulates input+output items across turns within this connection.
    // Used to reconstruct full message history for upstream REST APIs without
    // hitting the conversation store. One conversation per WS session.
    history []types.InputItem
    
    // responseIDs tracks all conversation IDs generated in this connection.
    // Used for cleanup of non-persisted conversations on Close().
    responseIDs []string
}
```

**`api/websocket/connection.go`** — Prepend history in `handleClientMessage()` before building request:
```go
func (c *Connection) handleClientMessage(message []byte) error {
    // ... parse req, resolve route, etc. ...
    
    // Prepend accumulated conversation history to the current request input.
    // This gives the upstream model full context without needing a chain walk.
    // Only prepend for followup turns (when we have prior history).
    c.mu.Lock()
    if len(c.history) > 0 {
        req.Input = append(c.history, req.Input...)
    }
    c.mu.Unlock()
    
    responsesBody := c.buildResponsesBody(&req, route)
    // ... rest of flow ...
}
```

**`api/websocket/connection.go`** — Append turn output to history after upstream response completes (in `finalizeCapture` or new helper):
```go
func (c *Connection) accumulateTurnHistory(reqInput []types.InputItem, outputItems []types.OutputItem) {
    c.mu.Lock()
    defer c.mu.Unlock()
    // Append this turn's input to history
    c.history = append(c.history, reqInput...)
    // Convert output items to input items for next turn
    for _, out := range outputItems {
        c.history = append(c.history, types.InputItem{
            Type:    out.Type,
            Role:    "assistant",
            Content: out.Content,
            // ... map other fields ...
        })
    }
}
```

**`convert/responses_to_chat.go`** — Always walk chain when `previous_response_id` present (for cross-connection resume):
```go
// BEFORE: if req.PreviousResponseID != "" && c.shouldStore {
// AFTER:  if req.PreviousResponseID != "" {
if req.PreviousResponseID != "" {
    chain, err := conversation.WalkChainFromDefaultWithOwnership(req.PreviousResponseID, "")
    // ... existing chain handling ...
}
```

**`convert/chat_to_responses.go`** — Always store, set `Persisted` from `store`:
```go
func (t *ChatToResponsesTransformer) storeConversation(outputItems []map[string]interface{}) {
    // Always store for cross-connection resume support
    // t.shouldStore controls ONLY the Persisted flag
    // ... build conv ...
    conv.Persisted = t.shouldStore
    conversation.StoreInDefault(conv)
}
```

**`convert/anthropic_to_responses_streaming.go`** — Same change: always store, set `Persisted`.

**`api/websocket/connection.go`** — Track response IDs and clean on close:
```go
func (c *Connection) Close() {
    // ... existing close logic ...
    // Clean up non-persisted conversations from the store
    for _, id := range c.responseIDs {
        conv := conversation.GetFromDefault(id)
        if conv != nil && !conv.Persisted {
            conversation.DeleteFromDefault(id)
        }
    }
}
```

**CRUD endpoints** — Filter by `Persisted`:
The `/v1/responses/:id` endpoint handler checks `conv.Persisted` before returning the conversation. Non-persisted conversations return 404.

#### Data flow

```
─── WITHIN-CONNECTION TURNS (in-memory, no store lookup) ───

Turn 1: codex WS ──► response.create {store:false, input: [msg1]}
                        │
                        ▼
                 handleClientMessage()
                 history = [] → no prepend
                 req.Input = [msg1]
                        │
                        ▼
                 upstream responds with output [resp1]
                        │
                        ▼
                 accumulateTurnHistory():
                   history = [msg1, resp1_as_input]
                   responseIDs += ["resp_id_1"]

Turn 2: codex WS ──► response.create {store:false, input: [msg2]}
                        │
                        ▼
                 handleClientMessage()
                 history = [msg1, resp1_as_input] → prepend!
                 req.Input = [msg1, resp1_as_input, msg2]  ← full context!
                        │
                        ▼
                 upstream responds with output [resp2]
                        │
                        ▼
                 accumulateTurnHistory():
                   history = [msg1, resp1_as_input, msg2, resp2_as_input]

─── CROSS-CONNECTION RESUME (store chain walk) ───

codex exec resume ──► NEW WS connection
                      response.create {previous_response_id:"resp_id_2", input: [msg3]}
                        │
                        ▼
                 history = [] (new connection, empty)
                 req.Input = [msg3]
                        │
                        ▼
                 transformRequest → responses_to_chat.go
                   previous_response_id="resp_id_2" → walk chain from store
                   prepend [msg1, resp1, msg2, resp2] to messages
                   upstream gets [msg1, resp1, msg2, resp2, msg3] ← full context! ✓

WS close ──► Connection.Close()
                for each id in responseIDs:
                  if !conv.Persisted → DeleteFromDefault(id)
```

### Fix 1c: Flush() capture on nil clientConn

**`api/websocket/connection.go:643-661`** — Move `captureDownstreamEvent` outside nil guard:

```go
func (w *wsMessageWriter) Flush() {
    if w.buf.Len() > 0 {
        remaining := w.buf.String()
        w.buf.Reset()
        for _, line := range strings.Split(remaining, "\n") {
            if strings.HasPrefix(line, "data: ") {
                dataStr := line[6:]
                if dataStr != "" {
                    if w.conn.clientConn != nil {
                        w.conn.clientConn.SetWriteDeadline(time.Now().Add(WriteDeadline))
                        w.conn.clientConn.WriteMessage(websocket.TextMessage, []byte(dataStr))
                    }
                    // Always capture, even when clientConn is nil
                    w.captureDownstreamEvent("data: " + dataStr + "\n\n")
                }
            }
        }
    }
}
```

### Fix 1d: Multi-line SSE data concatenation

**`api/websocket/connection.go:594-602`** — Concatenate all `data:` lines:

```go
// Parse SSE wire format: extract all data lines (SSE spec allows multi-line)
var dataLines []string
for _, line := range strings.Split(event, "\n") {
    if strings.HasPrefix(line, "data: ") {
        dataLines = append(dataLines, line[6:])
    }
}
dataStr := strings.Join(dataLines, "\n")
```

### Fix 2a: Per-turn unique RequestID

**`api/websocket/handler.go:68-73`** — No change to handler; instead modify how turns record:

**`api/websocket/connection.go`** — Generate per-turn capture ID in `proxyRequest()`:
```go
func (c *Connection) proxyRequest(...) error {
    // Generate per-turn unique request ID for capture logs
    turnID := uuid.New().String()[:8]
    if c.captureRecorder != nil {
        c.captureRecorder.SetRequestID(turnID)
    }
    // ... rest of proxyRequest ...
}
```

This ensures each turn gets a unique ID in the capture memory store (visible at `/ui/api/logs`), while the connection's `responseIDs` tracks conversation store IDs for cleanup.

### Fix 2b: Unique connection ID

**`api/websocket/connection.go:562-563`** — Use UUID:
```go
func generateConnectionID() string {
    return "ws_" + uuid.New().String()
}
```

## Files Changed

| File | Change |
|------|--------|
| `conversation/store.go` | Add `Persisted bool` to `Conversation`; add `DeleteFromDefault()` helper |
| `api/websocket/connection.go` | Add `history []types.InputItem` + `responseIDs []string` fields; prepend history in `handleClientMessage()`; accumulate turn output in new helper; cleanup non-persisted in `Close()`; fix `Flush()`; fix multi-line SSE; fix `generateConnectionID()`; per-turn UUID in `proxyRequest()` |
| `convert/responses_to_chat.go` | Remove `shouldStore` gate from chain lookup (line 68) — for cross-connection resume |
| `convert/chat_to_responses.go` | Always store; set `Persisted` from `shouldStore`; return output items for accumulation |
| `convert/anthropic_to_responses_streaming.go` | Same: always store; set `Persisted` from `shouldStore`; return output items |
| `api/websocket/handler.go` | No changes needed (uuid already imported via capture package) |
| `api/handlers/response_get.go` | Filter by `Persisted` — non-persisted conversations return 404 |
| `api/handlers/response_input_items.go` | Same: filter by `Persisted` |

## Test Plan

### Test 1: Within-connection history accumulation (in-memory)

**Setup**: WS connection, Turn 1 sends `{input: [msg1]}` with `store: false`. Turn 2 sends `{input: [msg2]}` (no `previous_response_id`).

**Verify**: Turn 2's upstream REST request body contains `[msg1, <turn1_assistant_output>, msg2]` — full history prepended from `Connection.history`. No conversation store lookup needed for Turn 2.

### Test 2: Cross-connection resume (store chain walk)

**Setup**: WS connection A runs one turn with `store: false`, gets response ID `resp_A`. Connection A closes. WS connection B opens with `previous_response_id: resp_A`.

**Verify**: Connection B's upstream REST request body contains Turn A's history (retrieved from conversation store chain walk) prepended to new input.

### Test 3: Non-persisted conversations cleaned on close

**Setup**: WS connection with `store: false`, two turns. Close connection.

**Verify**: `conversation.DefaultStore` no longer contains either conversation. Direct store lookup returns nil.

### Test 4: Persisted conversations survive close

**Setup**: WS connection with `store: true`, one turn. Close connection.

**Verify**: `conversation.DefaultStore` still contains the conversation. CRUD endpoint returns it.

### Test 5: CRUD hides non-persisted conversations

**Setup**: Store a conversation with `Persisted: false`. Query `/v1/responses/:id`.

**Verify**: Returns 404.

### Test 6: Per-turn unique capture IDs

**Setup**: WS connection with 3 turns. Query `/ui/api/logs`.

**Verify**: 3 distinct `request_id` values, each an 8-char UUID. No duplicates.

### Test 7: Flush capture with nil clientConn

**Setup**: Call `wsMessageWriter.Write()` with valid SSE, then set `clientConn = nil`, call `Flush()`.

**Verify**: `captureWriter.Chunks()` contains all events including flushed ones.

### Test 8: Multi-line SSE data

**Setup**: Write SSE event with `data: line1\ndata: line2\n\n` to `wsMessageWriter`.

**Verify**: WS client receives `"line1\nline2"` as a single message. `captureWriter` records the concatenated data.

### Test 9: Unique connection IDs

**Setup**: Create 100 connections rapidly.

**Verify**: All 100 connection IDs are unique. No collisions.

### Test 10: Regression — existing tests pass

**Setup**: `go test ./...`

**Verify**: All existing tests pass. Coverage ≥ 90%.

### Test 11: Integration — multi-turn codex with store:false

**Setup**: `codex exec "read file X"` then `codex exec resume --last "analyze what you read"`.

**Verify**: Second codex call receives full conversation context and produces a meaningful, context-aware response (not a generic "what can I help with?").

## Backward Compatibility

- Existing tests: all must pass unchanged or with minimal updates (test expectations for new `Persisted` field)
- HTTP (non-WS) responses API: unaffected — `store` semantics unchanged for REST clients
- WS clients sending `store: true`: behavior unchanged — conversations persisted as before
- WS clients sending `store: false`: behavior FIXED — conversations now have context continuity while remaining non-persisted
- `/ui/api/logs`: new entries get unique IDs; existing entries unchanged
