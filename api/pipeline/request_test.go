package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"ai-proxy/config"
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
		name           string
		upstreamFormat string
		isPassthrough  bool
		reasoningSplit bool
		body           string
		assertFn       func(t *testing.T, result []byte)
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

// TestStepInjectSamplingParams tests the sampling parameter injection.
func TestStepInjectSamplingParams(t *testing.T) {
	tests := []struct {
		name            string
		params          *config.SamplingParams
		body            string
		wantTemperature interface{}
		wantTopP        interface{}
		wantTopK        interface{}
		wantPresence    interface{}
		wantFrequency   interface{}
	}{
		{
			name:            "nil params returns unchanged",
			params:          nil,
			body:            `{"model": "test", "messages": []}`,
			wantTemperature: nil,
		},
		{
			name:            "empty params returns unchanged",
			params:          &config.SamplingParams{},
			body:            `{"model": "test", "messages": []}`,
			wantTemperature: nil,
		},
		{
			name:            "inject temperature when not set (override default true)",
			params:          &config.SamplingParams{Temperature: float64Ptr(0.7)},
			body:            `{"model": "test", "messages": []}`,
			wantTemperature: 0.7,
		},
		{
			name:            "override temperature when already set (override default true)",
			params:          &config.SamplingParams{Temperature: float64Ptr(0.7)},
			body:            `{"model": "test", "messages": [], "temperature": 0.3}`,
			wantTemperature: 0.7, // config overrides client
		},
		{
			name: "do not override when override=false",
			params: &config.SamplingParams{
				Override:    boolPtr(false),
				Temperature: float64Ptr(0.7),
			},
			body:            `{"model": "test", "messages": [], "temperature": 0.3}`,
			wantTemperature: 0.3, // client wins
		},
		{
			name: "apply when client not set and override=false",
			params: &config.SamplingParams{
				Override:    boolPtr(false),
				Temperature: float64Ptr(0.7),
			},
			body:            `{"model": "test", "messages": []}`,
			wantTemperature: 0.7, // config applies when client doesn't set
		},
		{
			name: "inject all params",
			params: &config.SamplingParams{
				Temperature:      float64Ptr(0.8),
				TopP:             float64Ptr(0.9),
				TopK:             intPtr(40),
				PresencePenalty:  float64Ptr(0.2),
				FrequencyPenalty: float64Ptr(0.1),
			},
			body:            `{"model": "test", "messages": []}`,
			wantTemperature: 0.8,
			wantTopP:        0.9,
			wantTopK:        40,
			wantPresence:    0.2,
			wantFrequency:   0.1,
		},
		{
			name: "partial override with override=true",
			params: &config.SamplingParams{
				Temperature: float64Ptr(0.5),
				TopP:        float64Ptr(0.95),
			},
			body:            `{"model": "test", "messages": [], "temperature": 0.9, "top_k": 30}`,
			wantTemperature: 0.5,  // overridden
			wantTopP:        0.95, // injected
			wantTopK:        30,   // client value preserved (not in params)
		},
		{
			name: "partial override with override=false",
			params: &config.SamplingParams{
				Override:    boolPtr(false),
				Temperature: float64Ptr(0.5),
				TopP:        float64Ptr(0.95),
			},
			body:            `{"model": "test", "messages": [], "temperature": 0.9}`,
			wantTemperature: 0.9,  // client wins
			wantTopP:        0.95, // config applies (client didn't set)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := stepInjectSamplingParams(tt.params)
			result, err := step(context.Background(), []byte(tt.body))
			if err != nil {
				t.Fatalf("stepInjectSamplingParams() error = %v", err)
			}

			var req map[string]interface{}
			if err := json.Unmarshal(result, &req); err != nil {
				t.Fatalf("failed to unmarshal result: %v", err)
			}

			// Check temperature
			if tt.wantTemperature != nil {
				if val, ok := req["temperature"].(float64); !ok || val != tt.wantTemperature {
					t.Errorf("temperature = %v, want %v", req["temperature"], tt.wantTemperature)
				}
			} else if _, ok := req["temperature"]; ok {
				t.Errorf("temperature should not be set, got %v", req["temperature"])
			}

			// Check top_p
			if tt.wantTopP != nil {
				if val, ok := req["top_p"].(float64); !ok || val != tt.wantTopP {
					t.Errorf("top_p = %v, want %v", req["top_p"], tt.wantTopP)
				}
			}

			// Check top_k (JSON numbers are float64)
			if tt.wantTopK != nil {
				topKVal, ok := req["top_k"].(float64)
				if !ok || int(topKVal) != tt.wantTopK {
					t.Errorf("top_k = %v, want %v", req["top_k"], tt.wantTopK)
				}
			}

			// Check presence_penalty
			if tt.wantPresence != nil {
				if val, ok := req["presence_penalty"].(float64); !ok || val != tt.wantPresence {
					t.Errorf("presence_penalty = %v, want %v", req["presence_penalty"], tt.wantPresence)
				}
			}

			// Check frequency_penalty
			if tt.wantFrequency != nil {
				if val, ok := req["frequency_penalty"].(float64); !ok || val != tt.wantFrequency {
					t.Errorf("frequency_penalty = %v, want %v", req["frequency_penalty"], tt.wantFrequency)
				}
			}
		})
	}
}

// TestBuildRequestPipeline_WithSamplingParams tests the full pipeline with
// sampling params integration.
func TestBuildRequestPipeline_WithSamplingParams(t *testing.T) {
	cfg := RequestConfig{
		DownstreamFormat: "openai",
		UpstreamFormat:   "openai",
		ResolvedModel:    "upstream-model",
		IsPassthrough:    false,
		SamplingParams: &config.SamplingParams{
			Temperature: float64Ptr(0.7),
			TopP:        float64Ptr(0.95),
		},
	}

	tf, err := BuildRequestPipeline(cfg)
	if err != nil {
		t.Fatalf("BuildRequestPipeline() error = %v", err)
	}

	body := `{"model": "original", "stream": true, "messages": [{"role": "user", "content": "hi"}]}`
	result, err := tf(context.Background(), []byte(body))
	if err != nil {
		t.Fatalf("transform() error = %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// Model should be updated
	if req["model"] != "upstream-model" {
		t.Errorf("model = %v, want upstream-model", req["model"])
	}

	// Sampling params should be injected
	if req["temperature"] != 0.7 {
		t.Errorf("temperature = %v, want 0.7", req["temperature"])
	}
	if req["top_p"] != 0.95 {
		t.Errorf("top_p = %v, want 0.95", req["top_p"])
	}
}

// TestBuildRequestPipeline_SamplingParamsPassthrough tests that sampling params
// are injected even in passthrough mode.
func TestBuildRequestPipeline_SamplingParamsPassthrough(t *testing.T) {
	cfg := RequestConfig{
		DownstreamFormat: "openai",
		UpstreamFormat:   "openai",
		ResolvedModel:    "upstream-model",
		IsPassthrough:    true,
		SamplingParams: &config.SamplingParams{
			Temperature: float64Ptr(0.5),
		},
	}

	tf, err := BuildRequestPipeline(cfg)
	if err != nil {
		t.Fatalf("BuildRequestPipeline() error = %v", err)
	}

	body := `{"model": "original", "stream": true, "messages": []}`
	result, err := tf(context.Background(), []byte(body))
	if err != nil {
		t.Fatalf("transform() error = %v", err)
	}

	var req map[string]interface{}
	if err := json.Unmarshal(result, &req); err != nil {
		t.Fatalf("failed to unmarshal result: %v", err)
	}

	// Model should be updated
	if req["model"] != "upstream-model" {
		t.Errorf("model = %v, want upstream-model", req["model"])
	}

	// Sampling params should be injected
	if req["temperature"] != 0.5 {
		t.Errorf("temperature = %v, want 0.5", req["temperature"])
	}
}

// Helper functions for tests
func float64Ptr(v float64) *float64 {
	return &v
}

func boolPtr(v bool) *bool {
	return &v
}
