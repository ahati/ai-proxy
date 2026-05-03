package types

// WebSocket event types for the Responses API WebSocket mode.
// See: https://developers.openai.com/api/docs/guides/websocket-mode

// WSRequest represents a client-to-server WebSocket event.
// The main event type is "response.create" which mirrors the HTTP Responses API.
type WSRequest struct {
	// Type is the event type, always "response.create" for now.
	Type string `json:"type"`
	// Model is the model identifier to use.
	Model string `json:"model"`
	// Input is the input items for this turn.
	// For continuation, only new items need to be sent.
	Input []InputItem `json:"input,omitempty"`
	// Instructions provides system-level instructions.
	Instructions string `json:"instructions,omitempty"`
	// PreviousResponseID links to the prior response for conversation continuation.
	// When set, only incremental input needs to be provided.
	PreviousResponseID string `json:"previous_response_id,omitempty"`
	// Tools is a list of tools the model may call.
	Tools []ResponsesTool `json:"tools,omitempty"`
	// ToolChoice specifies which tool the model should use.
	ToolChoice interface{} `json:"tool_choice,omitempty"`
	// Store controls whether the response should be stored server-side.
	Store *bool `json:"store,omitempty"`
	// MaxOutputTokens is the maximum number of tokens to generate.
	MaxOutputTokens int `json:"max_output_tokens,omitempty"`
	// Temperature controls randomness in output generation.
	Temperature *float64 `json:"temperature,omitempty"`
	// TopP controls diversity via nucleus sampling.
	TopP *float64 `json:"top_p,omitempty"`
	// Reasoning enables reasoning mode for supported models.
	Reasoning *ReasoningConfig `json:"reasoning,omitempty"`
	// ParallelToolCalls enables parallel tool calling.
	ParallelToolCalls *bool `json:"parallel_tool_calls,omitempty"`
	// ResponseFormat specifies the format of the response.
	ResponseFormat *ResponseFormat `json:"response_format,omitempty"`
	// Metadata contains arbitrary metadata for the request.
	Metadata map[string]interface{} `json:"metadata,omitempty"`
	// Generate controls whether to generate output.
	// When false, prepares request state without generating output (warmup).
	Generate *bool `json:"generate,omitempty"`
	// EncryptedReasoning is used in ZDR mode to pass encrypted reasoning blobs.
	EncryptedReasoning string `json:"encrypted_reasoning,omitempty"`
	// Truncation controls how the response is truncated.
	Truncation string `json:"truncation,omitempty"`
	// User is a unique identifier for the end-user.
	User string `json:"user,omitempty"`
}

// WSEventType represents the type of a WebSocket server-to-client event.
type WSEventType string

const (
	// Response events
	WSEventResponseCreated         WSEventType = "response.created"
	WSEventResponseInProgress      WSEventType = "response.in_progress"
	WSEventResponseCompleted       WSEventType = "response.completed"
	WSEventResponseFailed          WSEventType = "response.failed"
	WSEventResponseCancelled       WSEventType = "response.cancelled"
	WSEventResponseQueued          WSEventType = "response.queued"
	WSEventResponseIncomplete      WSEventType = "response.incomplete"
	WSEventResponseOutputItemAdded WSEventType = "response.output_item.added"

	// Content events
	WSEventResponseContentPartAdded WSEventType = "response.content_part.added"
	WSEventResponseContentPartDone  WSEventType = "response.content_part.done"
	WSEventResponseOutputTextDelta  WSEventType = "response.output_text.delta"
	WSEventResponseOutputTextDone   WSEventType = "response.output_text.done"
	WSEventResponseRefusalDelta     WSEventType = "response.refusal.delta"
	WSEventResponseRefusalDone      WSEventType = "response.refusal.done"

	// Function call events
	WSEventResponseFunctionCallArgumentsDelta WSEventType = "response.function_call_arguments.delta"
	WSEventResponseFunctionCallArgumentsDone  WSEventType = "response.function_call_arguments.done"

	// Reasoning events
	WSEventResponseReasoningDelta        WSEventType = "response.reasoning.delta"
	WSEventResponseReasoningDone         WSEventType = "response.reasoning.done"
	WSEventResponseReasoningSummaryDelta WSEventType = "response.reasoning_summary.delta"
	WSEventResponseReasoningSummaryDone  WSEventType = "response.reasoning_summary.done"

	// Error event
	WSEventError WSEventType = "error"
)

// WSEvent represents a server-to-client WebSocket event.
// Events follow the same structure as SSE streaming events.
type WSEvent struct {
	// Type is the event type.
	Type WSEventType `json:"type"`
	// ResponseID is the ID of the response (for response events).
	ResponseID string `json:"response_id,omitempty"`
	// SequenceNumber is the sequence number for ordering.
	SequenceNumber int `json:"sequence_number,omitempty"`
	// Item is the output item (for item events).
	Item *OutputItem `json:"item,omitempty"`
	// ContentIndex is the index of the content part.
	ContentIndex *int `json:"content_index,omitempty"`
	// Delta is the incremental content.
	Delta string `json:"delta,omitempty"`
	// Response contains the full response object (for completed events).
	Response *ResponsesResponse `json:"response,omitempty"`
	// Error contains error details (for error events).
	Error *WSErrorDetail `json:"error,omitempty"`
}

// WSErrorDetail represents error details in a WebSocket error event.
type WSErrorDetail struct {
	// Type is the error type.
	Type string `json:"type,omitempty"`
	// Code is the error code.
	Code string `json:"code"`
	// Message is the human-readable error message.
	Message string `json:"message"`
	// Param is the parameter that caused the error.
	Param string `json:"param,omitempty"`
}

// WSErrorResponse represents an error response sent over WebSocket.
type WSErrorResponse struct {
	// Type is always "error".
	Type string `json:"type"`
	// Status is the HTTP status code.
	Status int `json:"status"`
	// Error contains the error details.
	Error WSErrorDetail `json:"error"`
}

// WebSocket error codes as defined by OpenAI.
const (
	WSErrorPreviousResponseNotFound        = "previous_response_not_found"
	WSErrorWebsocketConnectionLimitReached = "websocket_connection_limit_reached"
	WSErrorInvalidRequest                  = "invalid_request_error"
)

// NewWSError creates a WebSocket error response.
func NewWSError(code, message string, status int) WSErrorResponse {
	return WSErrorResponse{
		Type:   "error",
		Status: status,
		Error: WSErrorDetail{
			Code:    code,
			Message: message,
		},
	}
}

// NewWSErrorWithParam creates a WebSocket error response with a parameter.
func NewWSErrorWithParam(code, message, param string, status int) WSErrorResponse {
	return WSErrorResponse{
		Type:   "error",
		Status: status,
		Error: WSErrorDetail{
			Code:    code,
			Message: message,
			Param:   param,
		},
	}
}
