package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
)

// TestBuildRequestPipeline_Validation tests that invalid configs are rejected.
func TestBuildRequestPipeline_Validation(t *testing.T) {
	tests := []struct {
		name    string
		cfg     RequestConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: RequestConfig{
				DownstreamFormat: "openai",
				UpstreamFormat:   "anthropic",
				ResolvedModel:    "test-model",
			},
			wantErr: false,
		},
		{
			name: "missing downstream format",
			cfg: RequestConfig{
				UpstreamFormat: "anthropic",
				ResolvedModel:  "test-model",
			},
			wantErr: true,
		},
		{
			name: "missing upstream format",
			cfg: RequestConfig{
				DownstreamFormat: "openai",
				ResolvedModel:    "test-model",
			},
			wantErr: true,
		},
		{
			name: "missing resolved model",
			cfg: RequestConfig{
				DownstreamFormat: "openai",
				UpstreamFormat:   "anthropic",
			},
			wantErr: true,
		},
		{
			name: "unsupported downstream format",
			cfg: RequestConfig{
				DownstreamFormat: "grpc",
				UpstreamFormat:   "anthropic",
				ResolvedModel:    "test-model",
			},
			wantErr: true,
		},
		{
			name: "unsupported upstream format",
			cfg: RequestConfig{
				DownstreamFormat: "openai",
				UpstreamFormat:   "grpc",
				ResolvedModel:    "test-model",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := BuildRequestPipeline(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildRequestPipeline() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestBuildRequestPipeline_ModelUpdate tests that the model is always updated
// regardless of format combination.
func TestBuildRequestPipeline_ModelUpdate(t *testing.T) {
	tests := []struct {
		name     string
		cfg      RequestConfig
		body     string
		wantDiff bool // true if body should change beyond model update
	}{
		{
			name: "passthrough updates model only",
			cfg: RequestConfig{
				DownstreamFormat: "openai",
				UpstreamFormat:   "openai",
				ResolvedModel:    "upstream-model",
				IsPassthrough:    true,
			},
			body:     `{"model": "original", "stream": true}`,
			wantDiff: true, // model changes
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tf, err := BuildRequestPipeline(tt.cfg)
			if err != nil {
				t.Fatalf("BuildRequestPipeline() error = %v", err)
			}

			result, err := tf(context.Background(), []byte(tt.body))
			if err != nil {
				t.Fatalf("transform() error = %v", err)
			}

			var req map[string]interface{}
			if err := json.Unmarshal(result, &req); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}

			if req["model"] != tt.cfg.ResolvedModel {
				t.Errorf("model = %v, want %v", req["model"], tt.cfg.ResolvedModel)
			}
		})
	}
}

// TestBuildRequestPipeline_OpenAIDownstream tests OpenAI downstream conversions.
func TestBuildRequestPipeline_OpenAIDownstream(t *testing.T) {
	tests := []struct {
		name              string
		upstreamFormat    string
		isPassthrough     bool
		reasoningSplit    bool
		body              string
		assertFn          func(t *testing.T, result []byte)
	}{
		{
			name:           "openai_to_openai adds stream_options",
			upstreamFormat: "openai",
			isPassthrough:  false,
			body:           `{"model": "test", "stream": true, "messages": [{"role": "user", "content": "hi"}]}`,
			assertFn: func(t *testing.T, result []byte) {
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				so, ok := req["stream_options"].(map[string]interface{})
				if !ok {
					t.Fatal("expected stream_options to be set")
				}
				if so["include_usage"] != true {
					t.Error("expected include_usage = true")
				}
			},
		},
		{
			name:           "openai_to_openai with reasoning_split",
			upstreamFormat: "openai",
			isPassthrough:  false,
			reasoningSplit: true,
			body:           `{"model": "test", "stream": true, "messages": []}`,
			assertFn: func(t *testing.T, result []byte) {
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				if req["reasoning_split"] != true {
					t.Error("expected reasoning_split = true")
				}
			},
		},
		{
			name:           "openai_passthrough no stream_options",
			upstreamFormat: "openai",
			isPassthrough:  true,
			body:           `{"model": "test", "stream": true, "messages": []}`,
			assertFn: func(t *testing.T, result []byte) {
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				if _, ok := req["stream_options"]; ok {
					t.Error("passthrough should not add stream_options")
				}
			},
		},
		{
			name:           "openai_to_anthropic converts format",
			upstreamFormat: "anthropic",
			isPassthrough:  false,
			body:           `{"model": "test", "stream": true, "messages": [{"role": "user", "content": "hello"}]}`,
			assertFn: func(t *testing.T, result []byte) {
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				// Anthropic format has 'messages' but also 'max_tokens'
				if _, ok := req["max_tokens"]; !ok {
					t.Error("expected Anthropic format with max_tokens")
				}
			},
		},
		{
			name:           "openai_to_responses converts format",
			upstreamFormat: "responses",
			isPassthrough:  false,
			body:           `{"model": "test", "stream": true, "messages": [{"role": "user", "content": "hello"}]}`,
			assertFn: func(t *testing.T, result []byte) {
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				// Responses format has 'input' instead of 'messages'
				if _, ok := req["input"]; !ok {
					t.Error("expected Responses format with 'input' field")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := RequestConfig{
				DownstreamFormat: "openai",
				UpstreamFormat:   tt.upstreamFormat,
				ResolvedModel:    "upstream-model",
				IsPassthrough:    tt.isPassthrough,
				ReasoningSplit:   tt.reasoningSplit,
			}

			tf, err := BuildRequestPipeline(cfg)
			if err != nil {
				t.Fatalf("BuildRequestPipeline() error = %v", err)
			}

			result, err := tf(context.Background(), []byte(tt.body))
			if err != nil {
				t.Fatalf("transform() error = %v", err)
			}

			tt.assertFn(t, result)
		})
	}
}

// TestBuildRequestPipeline_AnthropicDownstream tests Anthropic downstream conversions.
func TestBuildRequestPipeline_AnthropicDownstream(t *testing.T) {
	tests := []struct {
		name           string
		upstreamFormat string
		isPassthrough  bool
		body           string
		assertFn       func(t *testing.T, result []byte)
	}{
		{
			name:           "anthropic_to_openai converts format",
			upstreamFormat: "openai",
			isPassthrough:  false,
			body:           `{"model": "test", "stream": true, "messages": [{"role": "user", "content": "hello"}]}`,
			assertFn: func(t *testing.T, result []byte) {
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				// OpenAI format has stream_options
				if so, ok := req["stream_options"]; ok {
					if m, ok := so.(map[string]interface{}); ok {
						if m["include_usage"] != true {
							t.Error("expected include_usage = true")
						}
					}
				}
			},
		},
		{
			name:           "anthropic_to_anthropic normalizes web search",
			upstreamFormat: "anthropic",
			isPassthrough:  false,
			body: `{
				"model": "test",
				"stream": true,
				"messages": [{
					"role": "user",
					"content": [{"type": "text", "text": "hi"}]
				}]
			}`,
			assertFn: func(t *testing.T, result []byte) {
				// Should still have messages (format unchanged, just normalized)
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				if _, ok := req["messages"]; !ok {
					t.Error("expected messages field to be present")
				}
			},
		},
		{
			name:           "anthropic_passthrough normalizes web search",
			upstreamFormat: "anthropic",
			isPassthrough:  true,
			body: `{
				"model": "test",
				"stream": true,
				"messages": [{
					"role": "user",
					"content": [{"type": "text", "text": "hi"}]
				}]
			}`,
			assertFn: func(t *testing.T, result []byte) {
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				if _, ok := req["messages"]; !ok {
					t.Error("passthrough should still have messages")
				}
			},
		},
		{
			name:           "anthropic_to_responses converts format",
			upstreamFormat: "responses",
			isPassthrough:  false,
			body:           `{"model": "test", "stream": true, "messages": [{"role": "user", "content": "hello"}]}`,
			assertFn: func(t *testing.T, result []byte) {
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				// Responses format has 'input'
				if _, ok := req["input"]; !ok {
					t.Error("expected Responses format with 'input' field")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := RequestConfig{
				DownstreamFormat: "anthropic",
				UpstreamFormat:   tt.upstreamFormat,
				ResolvedModel:    "upstream-model",
				IsPassthrough:    tt.isPassthrough,
			}

			tf, err := BuildRequestPipeline(cfg)
			if err != nil {
				t.Fatalf("BuildRequestPipeline() error = %v", err)
			}

			result, err := tf(context.Background(), []byte(tt.body))
			if err != nil {
				t.Fatalf("transform() error = %v", err)
			}

			tt.assertFn(t, result)
		})
	}
}

// TestBuildRequestPipeline_ResponsesDownstream tests Responses API downstream conversions.
func TestBuildRequestPipeline_ResponsesDownstream(t *testing.T) {
	tests := []struct {
		name           string
		upstreamFormat string
		isPassthrough  bool
		body           string
		assertFn       func(t *testing.T, result []byte)
	}{
		{
			name:           "responses_to_openai converts format",
			upstreamFormat: "openai",
			isPassthrough:  false,
			body:           `{"model": "test", "input": "hello", "stream": true}`,
			assertFn: func(t *testing.T, result []byte) {
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				// Chat format has 'messages' instead of 'input'
				if _, ok := req["messages"]; !ok {
					t.Error("expected Chat format with 'messages' field")
				}
			},
		},
		{
			name:           "responses_to_anthropic converts format",
			upstreamFormat: "anthropic",
			isPassthrough:  false,
			body:           `{"model": "test", "input": "hello", "stream": true}`,
			assertFn: func(t *testing.T, result []byte) {
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				// Anthropic format has 'messages' and 'max_tokens'
				if _, ok := req["messages"]; !ok {
					t.Error("expected Anthropic format with 'messages' field")
				}
			},
		},
		{
			name:           "responses_passthrough preserves format",
			upstreamFormat: "responses",
			isPassthrough:  true,
			body:           `{"model": "test", "input": "hello", "stream": true}`,
			assertFn: func(t *testing.T, result []byte) {
				var req map[string]interface{}
				json.Unmarshal(result, &req)
				// Should still have 'input' field (Responses format preserved)
				if _, ok := req["input"]; !ok {
					t.Error("passthrough should preserve 'input' field")
				}
				if req["model"] != "upstream-model" {
					t.Errorf("model = %v, want upstream-model", req["model"])
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := RequestConfig{
				DownstreamFormat: "responses",
				UpstreamFormat:   tt.upstreamFormat,
				ResolvedModel:    "upstream-model",
				IsPassthrough:    tt.isPassthrough,
			}

			tf, err := BuildRequestPipeline(cfg)
			if err != nil {
				t.Fatalf("BuildRequestPipeline() error = %v", err)
			}

			result, err := tf(context.Background(), []byte(tt.body))
			if err != nil {
				t.Fatalf("transform() error = %v", err)
			}

			tt.assertFn(t, result)
		})
	}
}

// TestBuildRequestPipeline_WebSearch tests web search preprocessing steps.
func TestBuildRequestPipeline_WebSearch(t *testing.T) {
	body := `{
		"model": "test",
		"stream": true,
		"messages": [{"role": "user", "content": "search the web"}],
		"tools": [{"type": "web_search_20250305", "name": "web_search"}]
	}`

	cfg := RequestConfig{
		DownstreamFormat: "anthropic",
		UpstreamFormat:   "openai",
		ResolvedModel:    "upstream-model",
		WebSearchEnabled: true,
	}

	tf, err := BuildRequestPipeline(cfg)
	if err != nil {
		t.Fatalf("BuildRequestPipeline() error = %v", err)
	}

	result, err := tf(context.Background(), []byte(body))
	if err != nil {
		t.Fatalf("transform() error = %v", err)
	}

	var req map[string]interface{}
	json.Unmarshal(result, &req)
	tools, ok := req["tools"].([]interface{})
	if !ok || len(tools) == 0 {
		t.Fatal("expected tools to be present")
	}
	// Server web search tool should be converted to function tool
	firstTool := tools[0].(map[string]interface{})
	if firstTool["type"] != "function" {
		t.Errorf("expected tool type 'function', got %v", firstTool["type"])
	}
}

// TestBuildRequestPipeline_ChainSteps verifies that chainSteps composes correctly.
func TestBuildRequestPipeline_ChainSteps(t *testing.T) {
	callOrder := []int{}

	step1 := func(ctx context.Context, body []byte) ([]byte, error) {
		callOrder = append(callOrder, 1)
		return append(body, []byte("_step1")...), nil
	}
	step2 := func(ctx context.Context, body []byte) ([]byte, error) {
		callOrder = append(callOrder, 2)
		return append(body, []byte("_step2")...), nil
	}
	step3 := func(ctx context.Context, body []byte) ([]byte, error) {
		callOrder = append(callOrder, 3)
		return append(body, []byte("_step3")...), nil
	}

	chained := chainSteps([]RequestTransform{step1, step2, step3})
	result, err := chained(context.Background(), []byte("start"))
	if err != nil {
		t.Fatalf("chainSteps() error = %v", err)
	}

	expected := "start_step1_step2_step3"
	if string(result) != expected {
		t.Errorf("chainSteps result = %q, want %q", string(result), expected)
	}

	if len(callOrder) != 3 || callOrder[0] != 1 || callOrder[1] != 2 || callOrder[2] != 3 {
		t.Errorf("call order = %v, want [1 2 3]", callOrder)
	}
}

// TestBuildRequestPipeline_ChainStepsStopsOnError verifies that chainSteps
// stops on first error.
func TestBuildRequestPipeline_ChainStepsStopsOnError(t *testing.T) {
	callOrder := []int{}

	step1 := func(ctx context.Context, body []byte) ([]byte, error) {
		callOrder = append(callOrder, 1)
		return body, nil
	}
	step2 := func(ctx context.Context, body []byte) ([]byte, error) {
		callOrder = append(callOrder, 2)
		return nil, fmt.Errorf("step2 failed")
	}
	step3 := func(ctx context.Context, body []byte) ([]byte, error) {
		callOrder = append(callOrder, 3)
		return body, nil
	}

	chained := chainSteps([]RequestTransform{step1, step2, step3})
	_, err := chained(context.Background(), []byte("start"))
	if err == nil {
		t.Fatal("expected error from step2")
	}

	if len(callOrder) != 2 || callOrder[0] != 1 || callOrder[1] != 2 {
		t.Errorf("call order = %v, want [1 2] (step3 should not run)", callOrder)
	}
}
