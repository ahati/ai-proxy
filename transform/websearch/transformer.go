// Package websearch provides SSE transformation for intercepting web search tool calls.
// This transformer wraps another SSETransformer and intercepts tool_use blocks
// for the web_search tool, executing searches and emitting synthetic results.
package websearch

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/tmaxmax/go-sse"

	"ai-proxy/logging"
	"ai-proxy/transform"
	"ai-proxy/types"
)

// pendingBlock tracks state for an in-progress tool_use block.
type pendingBlock struct {
	inputBuffer *bytes.Buffer // Accumulated input JSON deltas
	blockID     string        // The tool_use ID
	startIndex  int           // The content block index where this block started
}

// Service defines the interface for executing web searches.
// This is defined locally to avoid import cycles with the main websearch package.
type Service interface {
	// ExecuteSearch performs a web search and returns raw search results.
	ExecuteSearch(ctx context.Context, query string, allowedDomains, blockedDomains []string) ([]types.WebSearchResult, error)
}

// webSearchTools is the set of tool names that trigger web search interception.
// Both "WebSearch" (tool definition name) and "web_search" (model output name) are intercepted.
var webSearchTools = map[string]bool{
	"WebSearch":  true, // Claude Code WebSearch tool (the name in tool definition)
	"web_search": true, // The tool name the model outputs
}

// isWebSearchTool returns true if the tool name is a web search tool.
func isWebSearchTool(name string) bool {
	return webSearchTools[name]
}

// Transformer wraps an SSETransformer and intercepts tool_use blocks for web_search.
type Transformer struct {
	base    transform.SSETransformer
	service Service

	mu            sync.Mutex
	pendingBlocks map[string]*pendingBlock // keyed by block ID
	blockIndex    int                      // tracks current content block index
	indexToID     map[int]string           // maps index to block ID for delta matching
}

// NewTransformer creates a new web search interception transformer.
func NewTransformer(base transform.SSETransformer, service Service) *Transformer {
	return &Transformer{
		base:          base,
		service:       service,
		pendingBlocks: make(map[string]*pendingBlock),
		indexToID:     make(map[int]string),
	}
}

// Initialize prepares the transformer by delegating to the base transformer.
func (t *Transformer) Initialize() error {
	return t.base.Initialize()
}

// Transform processes a single SSE event, intercepting web_search tool calls.
func (t *Transformer) Transform(event *sse.Event) error {
	if event.Data == "" {
		return t.base.Transform(event)
	}

	if event.Data == "[DONE]" {
		return t.base.Transform(event)
	}

	// Parse the event to check for web_search interception
	var evt eventParser
	if err := json.Unmarshal([]byte(event.Data), &evt); err != nil {
		return t.base.Transform(event)
	}

	switch evt.Type {
	case "content_block_start":
		return t.handleContentBlockStart(event, &evt)
	case "content_block_delta":
		return t.handleContentBlockDelta(event, &evt)
	case "content_block_stop":
		return t.handleContentBlockStop(event, &evt)
	default:
		return t.base.Transform(event)
	}
}

// eventParser is used to parse SSE events for interception decisions.
type eventParser struct {
	Type         string          `json:"type"`
	Index        *int            `json:"index,omitempty"`
	Delta        json.RawMessage `json:"delta,omitempty"`
	ContentBlock json.RawMessage `json:"content_block,omitempty"`
}

// handleContentBlockStart processes content_block_start events.
// For web_search tool_use blocks, it tracks the block for later injection of results.
func (t *Transformer) handleContentBlockStart(event *sse.Event, evt *eventParser) error {
	// Always track index incrementally for ALL blocks
	idx := t.blockIndex
	if evt.Index != nil {
		idx = *evt.Index
	}
	t.blockIndex = idx + 1

	// Check if this is a tool_use or server_tool_use block for web_search
	if evt.ContentBlock != nil {
		var block struct {
			Type string `json:"type"`
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if err := json.Unmarshal(evt.ContentBlock, &block); err == nil {
			// Check for both tool_use and server_tool_use with name in webSearchTools
			if (block.Type == "tool_use" || block.Type == "server_tool_use") && isWebSearchTool(block.Name) {
				// Start tracking this block
				t.mu.Lock()
				t.pendingBlocks[block.ID] = &pendingBlock{
					inputBuffer: bytes.NewBuffer(nil),
					blockID:     block.ID,
					startIndex:  idx,
				}
				t.indexToID[idx] = block.ID
				t.mu.Unlock()

				logging.InfoMsg("[WebSearch] Intercepted tool_use block: id=%s, name=%s, type=%s", block.ID, block.Name, block.Type)
			}
		}
	}

	// Always pass through to base transformer
	return t.base.Transform(event)
}

// handleContentBlockDelta processes content_block_delta events.
// For tracked web_search blocks, it buffers the input for later use.
func (t *Transformer) handleContentBlockDelta(event *sse.Event, evt *eventParser) error {
	// Get index from event
	if evt.Index == nil {
		return t.base.Transform(event)
	}
	idx := *evt.Index

	// Check if this index corresponds to a tracked web_search block
	t.mu.Lock()
	blockID, isTracked := t.indexToID[idx]
	var pending *pendingBlock
	if isTracked {
		pending = t.pendingBlocks[blockID]
	}
	t.mu.Unlock()

	if isTracked && pending != nil {
		// This is a delta for our tracked web_search block
		if evt.Delta != nil {
			var delta struct {
				Type        string `json:"type"`
				PartialJSON string `json:"partial_json,omitempty"`
			}
			if err := json.Unmarshal(evt.Delta, &delta); err == nil {
				if delta.Type == "input_json_delta" && delta.PartialJSON != "" {
					// Accumulate the partial JSON
					t.mu.Lock()
					pending.inputBuffer.WriteString(delta.PartialJSON)
					t.mu.Unlock()
				}
			}
		}
	}

	// Always pass through to base transformer
	return t.base.Transform(event)
}

// handleContentBlockStop processes content_block_stop events.
// For tracked web_search blocks, it executes the search and emits the result.
func (t *Transformer) handleContentBlockStop(event *sse.Event, evt *eventParser) error {
	// Get index from event
	if evt.Index == nil {
		return t.base.Transform(event)
	}
	idx := *evt.Index

	// Check if this index corresponds to a tracked web_search block
	t.mu.Lock()
	blockID, isTracked := t.indexToID[idx]
	var pending *pendingBlock
	if isTracked {
		pending = t.pendingBlocks[blockID]
		// Clean up tracking
		delete(t.pendingBlocks, blockID)
		delete(t.indexToID, idx)
	}
	t.mu.Unlock()

	if !isTracked || pending == nil {
		// Not a web_search block, just pass through
		return t.base.Transform(event)
	}

	// First, pass through the content_block_stop event
	if err := t.base.Transform(event); err != nil {
		return err
	}

	// Now parse the accumulated input and execute search
	var input types.WebSearchInput
	inputJSON := pending.inputBuffer.String()
	if inputJSON != "" && inputJSON != "{}" {
		if err := json.Unmarshal([]byte(inputJSON), &input); err != nil {
			// Malformed input - emit error result
			logging.ErrorMsg("[WebSearch] Failed to parse input JSON: %v", err)
			return t.emitWebSearchResult(pending.blockID, []types.WebSearchResult{{
				Type:      "web_search_error",
				ErrorCode: "parse_error",
				Message:   fmt.Sprintf("Failed to parse input JSON: %v", err),
			}})
		}
	}

	logging.InfoMsg("[WebSearch] Executing search for block %s: query=%q", pending.blockID, input.Query)

	// Execute the search
	results, err := t.executeSearch(&input)
	if err != nil {
		logging.ErrorMsg("[WebSearch] Search failed: %v", err)
		return t.emitWebSearchResult(pending.blockID, []types.WebSearchResult{{
			Type:      "web_search_error",
			ErrorCode: "search_failed",
			Message:   err.Error(),
		}})
	}

	logging.InfoMsg("[WebSearch] Injecting tool_result for block %s: %d results", pending.blockID, len(results))

	// Emit the synthetic web_search_tool_result event
	return t.emitWebSearchResult(pending.blockID, results)
}

// executeSearch executes the web search based on input.
func (t *Transformer) executeSearch(input *types.WebSearchInput) ([]types.WebSearchResult, error) {
	if t.service == nil {
		return []types.WebSearchResult{{
			Type:      "web_search_error",
			ErrorCode: "service_unavailable",
			Message:   "Web search service is not configured",
		}}, nil
	}

	// Execute search query
	if input.Query != "" {
		return t.service.ExecuteSearch(context.Background(), input.Query, input.AllowedDomains, input.BlockedDomains)
	}

	// No query provided - return empty results
	return []types.WebSearchResult{}, nil
}

// emitWebSearchResult emits a synthetic web_search_tool_result event in Anthropic format.
func (t *Transformer) emitWebSearchResult(toolUseID string, results []types.WebSearchResult) error {
	// Create synthetic SSE event for web_search_tool_result in Anthropic format
	eventData := map[string]interface{}{
		"type":  "content_block_start",
		"index": t.blockIndex,
		"content_block": map[string]interface{}{
			"type":        "web_search_tool_result", // Anthropic's expected type
			"tool_use_id": toolUseID,
			"content":     results,
		},
	}

	t.blockIndex++

	data, err := json.Marshal(eventData)
	if err != nil {
		return fmt.Errorf("marshal event data: %w", err)
	}

	event := &sse.Event{
		Type: "content_block_start",
		Data: string(data),
	}

	if err := t.base.Transform(event); err != nil {
		return err
	}

	// Emit content_block_stop for the web_search_tool_result
	stopData := map[string]interface{}{
		"type":  "content_block_stop",
		"index": t.blockIndex - 1,
	}

	stopJSON, err := json.Marshal(stopData)
	if err != nil {
		return fmt.Errorf("marshal stop event: %w", err)
	}

	stopEvent := &sse.Event{
		Type: "content_block_stop",
		Data: string(stopJSON),
	}

	return t.base.Transform(stopEvent)
}

// Flush flushes the base transformer.
func (t *Transformer) Flush() error {
	return t.base.Flush()
}

// HandleCancel handles cancellation requests.
func (t *Transformer) HandleCancel() error {
	return t.base.HandleCancel()
}

// Process handles a PipelineEvent by delegating to the existing Transform method.
// This makes Transformer implement the transform.Stage interface.
//
// @brief Implements Stage.Process by converting PipelineEvents to SSE events.
//
// @param event The pipeline event to process. Must not be zero-value.
//
// @return error Returns nil on success, or any error from the underlying Transform.
//
// @note EventDone delegates to Close rather than Transform, allowing proper cleanup.
// All other event types are converted to sse.Event and passed to Transform.
func (t *Transformer) Process(event transform.PipelineEvent) error {
	switch event.Type {
	case transform.EventAnthropicEvent:
		return t.Transform(&sse.Event{
			Type: event.SSEType,
			Data: string(event.Data),
		})
	case transform.EventSSE:
		return t.Transform(&sse.Event{
			Type: event.SSEType,
			Data: string(event.Data),
		})
	case transform.EventDone:
		return t.Close()
	default:
		return t.Transform(&sse.Event{
			Data: string(event.Data),
		})
	}
}

// Close cleans up resources and closes the base transformer.
func (t *Transformer) Close() error {
	// Clean up any pending blocks
	t.mu.Lock()
	t.pendingBlocks = make(map[string]*pendingBlock)
	t.indexToID = make(map[int]string)
	t.mu.Unlock()

	return t.base.Close()
}
