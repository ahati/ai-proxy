package types

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWSRequestParsing(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    WSRequest
		wantErr bool
	}{
		{
			name: "basic response.create",
			json: `{
				"type": "response.create",
				"model": "gpt-4o",
				"input": [
					{
						"type": "message",
						"role": "user",
						"content": "Hello"
					}
				]
			}`,
			want: WSRequest{
				Type:  "response.create",
				Model: "gpt-4o",
				Input: []InputItem{
					{
						Type:    "message",
						Role:    "user",
						Content: "Hello",
					},
				},
			},
		},
		{
			name: "response.create with previous_response_id",
			json: `{
				"type": "response.create",
				"model": "gpt-4o",
				"previous_response_id": "resp_123",
				"input": [
					{
						"type": "function_call_output",
						"call_id": "call_abc",
						"output": "result"
					}
				]
			}`,
			want: WSRequest{
				Type:               "response.create",
				Model:              "gpt-4o",
				PreviousResponseID: "resp_123",
				Input: []InputItem{
					{
						Type:   "function_call_output",
						CallID: "call_abc",
						Output: "result",
					},
				},
			},
		},
		{
			name: "response.create with generate false (warmup)",
			json: `{
				"type": "response.create",
				"model": "gpt-4o",
				"generate": false,
				"tools": [
					{
						"type": "function",
						"name": "search",
						"description": "Search for information"
					}
				]
			}`,
			want: WSRequest{
				Type:     "response.create",
				Model:    "gpt-4o",
				Generate: wsBoolPtr(false),
				Tools: []ResponsesTool{
					{
						Type:        "function",
						Name:        "search",
						Description: "Search for information",
					},
				},
			},
		},
		{
			name: "response.create with store false",
			json: `{
				"type": "response.create",
				"model": "gpt-4o",
				"store": false,
				"input": []
			}`,
			want: WSRequest{
				Type:  "response.create",
				Model: "gpt-4o",
				Store: wsBoolPtr(false),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got WSRequest
			err := json.Unmarshal([]byte(tt.json), &got)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.Model, got.Model)
			assert.Equal(t, tt.want.PreviousResponseID, got.PreviousResponseID)

			// Check generate field
			if tt.want.Generate != nil {
				assert.NotNil(t, got.Generate)
				assert.Equal(t, *tt.want.Generate, *got.Generate)
			}

			// Check store field
			if tt.want.Store != nil {
				assert.NotNil(t, got.Store)
				assert.Equal(t, *tt.want.Store, *got.Store)
			}
		})
	}
}

func TestWSEventParsing(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    WSEvent
		wantErr bool
	}{
		{
			name: "response.created event",
			json: `{
				"type": "response.created",
				"response_id": "resp_123",
				"sequence_number": 1
			}`,
			want: WSEvent{
				Type:           WSEventResponseCreated,
				ResponseID:     "resp_123",
				SequenceNumber: 1,
			},
		},
		{
			name: "response.completed event",
			json: `{
				"type": "response.completed",
				"response_id": "resp_123",
				"response": {
					"id": "resp_123",
					"status": "completed"
				}
			}`,
			want: WSEvent{
				Type:       WSEventResponseCompleted,
				ResponseID: "resp_123",
				Response: &ResponsesResponse{
					ID:     "resp_123",
					Status: "completed",
				},
			},
		},
		{
			name: "response.output_text.delta event",
			json: `{
				"type": "response.output_text.delta",
				"response_id": "resp_123",
				"content_index": 0,
				"delta": "Hello"
			}`,
			want: WSEvent{
				Type:         WSEventResponseOutputTextDelta,
				ResponseID:   "resp_123",
				ContentIndex: wsIntPtr(0),
				Delta:        "Hello",
			},
		},
		{
			name: "error event",
			json: `{
				"type": "error",
				"error": {
					"code": "previous_response_not_found",
					"message": "Previous response not found"
				}
			}`,
			want: WSEvent{
				Type: WSEventError,
				Error: &WSErrorDetail{
					Code:    "previous_response_not_found",
					Message: "Previous response not found",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got WSEvent
			err := json.Unmarshal([]byte(tt.json), &got)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want.Type, got.Type)
			assert.Equal(t, tt.want.ResponseID, got.ResponseID)
			assert.Equal(t, tt.want.Delta, got.Delta)

			if tt.want.ContentIndex != nil {
				assert.NotNil(t, got.ContentIndex)
				assert.Equal(t, *tt.want.ContentIndex, *got.ContentIndex)
			}

			if tt.want.Error != nil {
				assert.NotNil(t, got.Error)
				assert.Equal(t, tt.want.Error.Code, got.Error.Code)
				assert.Equal(t, tt.want.Error.Message, got.Error.Message)
			}
		})
	}
}

func TestWSErrorResponse(t *testing.T) {
	tests := []struct {
		name string
		err  WSErrorResponse
	}{
		{
			name: "previous_response_not_found",
			err: NewWSErrorWithParam(
				WSErrorPreviousResponseNotFound,
				"Previous response with id 'resp_abc' not found.",
				"previous_response_id",
				400,
			),
		},
		{
			name: "websocket_connection_limit_reached",
			err: NewWSError(
				WSErrorWebsocketConnectionLimitReached,
				"Responses websocket connection limit reached (60 minutes). Create a new websocket connection to continue.",
				400,
			),
		},
		{
			name: "invalid_request_error",
			err: NewWSError(
				WSErrorInvalidRequest,
				"Invalid request format",
				400,
			),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Verify the error can be serialized
			jsonBytes, err := json.Marshal(tt.err)
			assert.NoError(t, err)
			assert.Contains(t, string(jsonBytes), `"type":"error"`)

			// Verify the error can be deserialized
			var got WSErrorResponse
			err = json.Unmarshal(jsonBytes, &got)
			assert.NoError(t, err)
			assert.Equal(t, tt.err.Type, got.Type)
			assert.Equal(t, tt.err.Status, got.Status)
			assert.Equal(t, tt.err.Error.Code, got.Error.Code)
			assert.Equal(t, tt.err.Error.Message, got.Error.Message)
		})
	}
}

// Helper functions for websocket tests (with unique names to avoid conflicts)
func wsBoolPtr(b bool) *bool {
	return &b
}

func wsIntPtr(i int) *int {
	return &i
}
