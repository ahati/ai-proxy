package protocols

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"ai-proxy/config"
	"ai-proxy/logging"
	"ai-proxy/transform/toolcall"
	"ai-proxy/types"

	"github.com/gin-gonic/gin"
)

type BridgeAdapter struct{}

func NewBridgeAdapter() *BridgeAdapter {
	return &BridgeAdapter{}
}

type AnthropicToOpenAIRequest struct {
	Model       string                    `json:"model"`
	Messages    []AnthropicMessageInput   `json:"messages"`
	MaxTokens   int                       `json:"max_tokens,omitempty"`
	Stream      bool                      `json:"stream,omitempty"`
	Tools       []AnthropicToolDefinition `json:"tools,omitempty"`
	System      interface{}               `json:"system,omitempty"`
	Temperature float64                   `json:"temperature,omitempty"`
	TopP        float64                   `json:"top_p,omitempty"`
}

type AnthropicMessageInput struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type AnthropicToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Stream      bool            `json:"stream,omitempty"`
	Tools       []OpenAITool    `json:"tools,omitempty"`
	System      string          `json:"system,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	TopP        float64         `json:"top_p,omitempty"`
}

type OpenAIMessage struct {
	Role       string           `json:"role"`
	Content    interface{}      `json:"content,omitempty"`
	ToolCalls  []OpenAIToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type OpenAITool struct {
	Type     string         `json:"type"`
	Function OpenAIFunction `json:"function"`
}

type OpenAIFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters,omitempty"`
}

type OpenAIToolCall struct {
	ID       string                 `json:"id"`
	Type     string                 `json:"type"`
	Index    int                    `json:"index"`
	Function OpenAIToolCallFunction `json:"function"`
}

type OpenAIToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

func (a *BridgeAdapter) TransformRequest(body []byte) ([]byte, error) {
	var anthReq AnthropicToOpenAIRequest
	if err := json.Unmarshal(body, &anthReq); err != nil {
		return nil, err
	}

	openReq := OpenAIRequest{
		Model:       anthReq.Model,
		MaxTokens:   anthReq.MaxTokens,
		Stream:      anthReq.Stream,
		Temperature: anthReq.Temperature,
		TopP:        anthReq.TopP,
	}

	if system := a.extractSystemMessage(anthReq.System); system != "" {
		openReq.System = system
	}

	for _, msg := range anthReq.Messages {
		openMsg := a.convertMessage(msg)
		openReq.Messages = append(openReq.Messages, openMsg)
	}

	for _, tool := range anthReq.Tools {
		openTool := a.convertTool(tool)
		openReq.Tools = append(openReq.Tools, openTool)
	}

	return json.Marshal(openReq)
}

func (a *BridgeAdapter) extractSystemMessage(system interface{}) string {
	if system == nil {
		return ""
	}
	if s, ok := system.(string); ok {
		return s
	}
	if arr, ok := system.([]interface{}); ok {
		var content strings.Builder
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				if text, ok := m["text"].(string); ok {
					content.WriteString(text)
				}
			}
		}
		return content.String()
	}
	return ""
}

func (a *BridgeAdapter) convertMessage(anthMsg AnthropicMessageInput) OpenAIMessage {
	openMsg := OpenAIMessage{
		Role: anthMsg.Role,
	}

	switch content := anthMsg.Content.(type) {
	case string:
		openMsg.Content = content
	case []interface{}:
		var textContent strings.Builder
		var toolCalls []OpenAIToolCall
		var toolCallID string

		for _, item := range content {
			if m, ok := item.(map[string]interface{}); ok {
				switch m["type"] {
				case "text":
					if text, ok := m["text"].(string); ok {
						if textContent.Len() > 0 {
							textContent.WriteString("\n")
						}
						textContent.WriteString(text)
					}
				case "tool_use":
					if id, ok := m["id"].(string); ok {
						if name, ok := m["name"].(string); ok {
							input, _ := json.Marshal(m["input"])
							toolCalls = append(toolCalls, OpenAIToolCall{
								ID:   id,
								Type: "function",
								Function: OpenAIToolCallFunction{
									Name:      name,
									Arguments: string(input),
								},
							})
						}
					}
				case "tool_result":
					if id, ok := m["tool_use_id"].(string); ok {
						toolCallID = id
					}
				}
			}
		}

		if textContent.Len() > 0 {
			openMsg.Content = textContent.String()
		}
		if len(toolCalls) > 0 {
			openMsg.ToolCalls = toolCalls
		}
		if toolCallID != "" {
			openMsg.ToolCallID = toolCallID
		}
	}

	return openMsg
}

func (a *BridgeAdapter) convertTool(anthTool AnthropicToolDefinition) OpenAITool {
	return OpenAITool{
		Type: "function",
		Function: OpenAIFunction{
			Name:        anthTool.Name,
			Description: anthTool.Description,
			Parameters:  anthTool.InputSchema,
		},
	}
}

func (a *BridgeAdapter) ValidateRequest(body []byte) error {
	if !a.IsStreamingRequest(body) {
		return ErrNonStreamingNotSupported
	}
	return nil
}

func (a *BridgeAdapter) CreateTransformer(w io.Writer, base types.StreamChunk) *ToolCallTransformer {
	output := toolcall.NewAnthropicOutput(w, toolcall.ContextText, 0)
	return NewToolCallTransformer(w, base, output)
}

func (a *BridgeAdapter) UpstreamURL(cfg *config.Config) string {
	return cfg.OpenAIUpstreamURL
}

func (a *BridgeAdapter) UpstreamAPIKey(cfg *config.Config) string {
	return cfg.OpenAIUpstreamAPIKey
}

func (a *BridgeAdapter) ForwardHeaders(src, dst http.Header) {
	for k, v := range src {
		if strings.HasPrefix(k, "X-") || k == "Extra" {
			dst[k] = v
		}
	}
	for _, h := range []string{"Connection", "Keep-Alive", "Upgrade", "TE"} {
		if v := src.Get(h); v != "" {
			dst.Set(h, v)
		}
	}
}

func (a *BridgeAdapter) SendError(c *gin.Context, status int, msg string) {
	logging.ErrorMsg("Bridge handler error: %s", msg)
	c.JSON(status, types.Error{
		Type: "error",
		Error: types.ErrorDetail{
			Type:    "invalid_request_error",
			Message: msg,
		},
	})
}

func (a *BridgeAdapter) IsStreamingRequest(body []byte) bool {
	var req struct {
		Stream bool `json:"stream"`
	}
	json.Unmarshal(body, &req)
	return req.Stream
}
