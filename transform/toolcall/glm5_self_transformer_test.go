package toolcall

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"ai-proxy/types"

	"github.com/tmaxmax/go-sse"
)

func TestGLM5ToolCallsInAnthropicFormat(t *testing.T) {
	tests := []struct {
		name     string
		events   []types.Event
		validate func(t *testing.T, output string)
	}{
		{
			name: "single GLM-5 tool call in thinking block - Anthropic format",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_abc",
						Model: "glm-5",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `<tool_call>exec_command<arg_key>cmd</arg_key><arg_value>ls -la</arg_value></tool_call>`}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Should contain tool_use block
				if !strings.Contains(output, `"type":"tool_use"`) {
					t.Error("Expected tool_use in output")
				}
				// Should contain function name
				if !strings.Contains(output, `"name":"exec_command"`) {
					t.Errorf("Expected function name 'exec_command' in output, got: %s", output)
				}
				// Should contain input_json_delta for the args
				if !strings.Contains(output, `"type":"input_json_delta"`) {
					t.Error("Expected input_json_delta")
				}
				// Should contain the arguments
				if !strings.Contains(output, `"cmd":"ls -la"`) && !strings.Contains(output, `\"cmd\":\"ls -la\"`) {
					t.Errorf("Expected arguments in output, got: %s", output)
				}
			},
		},
		{
			name: "GLM-5 tool call split across multiple chunks - Anthropic format",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_abc",
						Model: "glm-5",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				// First chunk: partial opening
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `<tool_`}),
				},
				// Second chunk: rest of opening and function name
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `call>search`}),
				},
				// Third chunk: first arg key
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `<arg_key>query</arg_key>`}),
				},
				// Fourth chunk: first arg value
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `<arg_value>hello world</arg_value>`}),
				},
				// Fifth chunk: closing tag
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `</tool_call>`}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Should contain tool_use block
				if !strings.Contains(output, `"type":"tool_use"`) {
					t.Error("Expected tool_use in output")
				}
				// Should contain function name
				if !strings.Contains(output, `"name":"search"`) {
					t.Errorf("Expected function name 'search' in output, got: %s", output)
				}
				// Should contain the arguments
				if !strings.Contains(output, `"query":"hello world"`) && !strings.Contains(output, `\"query\":\"hello world\"`) {
					t.Error("Expected arguments in output")
				}
			},
		},
		{
			name: "multiple GLM-5 tool calls in thinking - Anthropic format",
			events: []types.Event{
				{
					Type: "message_start",
					Message: &types.MessageInfo{
						ID:    "msg_multi",
						Model: "glm-5",
					},
				},
				{
					Type:         "content_block_start",
					Index:        intPtr(0),
					ContentBlock: mustMarshal(types.ContentBlock{Type: "thinking"}),
				},
				{
					Type:  "content_block_delta",
					Index: intPtr(0),
					Delta: mustMarshal(types.ThinkingDelta{Type: "thinking_delta", Thinking: `<tool_call>read<arg_key>file</arg_key><arg_value>a.txt</arg_value></tool_call> and <tool_call>write<arg_key>file</arg_key><arg_value>b.txt</arg_value></tool_call>`}),
				},
				{
					Type:  "content_block_stop",
					Index: intPtr(0),
				},
				{
					Type: "message_stop",
				},
			},
			validate: func(t *testing.T, output string) {
				// Should contain both tool_use blocks
				if !strings.Contains(output, `"name":"read"`) {
					t.Error("Expected function name 'read' in output")
				}
				if !strings.Contains(output, `"name":"write"`) {
					t.Error("Expected function name 'write' in output")
				}
				// Should contain middle text as thinking
				if !strings.Contains(output, `"thinking":" and "`) {
					t.Error("Expected middle text as thinking in output")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			transformer := NewAnthropicTransformer(&buf)
			// Enable GLM-5 tool call transformation for these tests
			transformer.SetGLM5ToolCallTransform(true)

			for _, e := range tt.events {
				data, _ := json.Marshal(e)
				err := transformer.Transform(&sse.Event{Data: string(data)})
				if err != nil {
					t.Errorf("Transform returned error: %v", err)
				}
			}

			output := buf.String()
			t.Logf("Output:\n%s", output)

			if tt.validate != nil {
				tt.validate(t, output)
			}
		})
	}
}

func TestOpenAITransformer_ReasoningContentPreservedWithGLM5(t *testing.T) {
	buf := &bytes.Buffer{}
	tr := NewOpenAITransformer(buf)
	tr.SetGLM5ToolCallTransform(true)

	chunks := []types.Chunk{
		{
			ID:    "chatcmpl-test",
			Model: "glm-5",
			Choices: []types.Choice{{
				Delta: types.Delta{ReasoningContent: "thinking"},
			}},
		},
		{
			ID:    "chatcmpl-test",
			Model: "glm-5",
			Choices: []types.Choice{{
				Delta: types.Delta{Content: "answer"},
			}},
		},
	}

	for _, chunk := range chunks {
		data, _ := json.Marshal(chunk)
		event := &sse.Event{Data: string(data)}
		if err := tr.Transform(event); err != nil {
			t.Fatalf("Transform failed: %v", err)
		}
	}

	if err := tr.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	output := buf.String()
	t.Logf("Output:\n%s", output)

	if !strings.Contains(output, `"reasoning_content":"thinking"`) {
		t.Errorf("expected reasoning_content:thinking")
	}
	if strings.Contains(output, `"content":"thinking"`) {
		t.Errorf("reasoning should not be content")
	}
	if !strings.Contains(output, `"content":"answer"`) {
		t.Errorf("expected content:answer")
	}
}

func TestOpenAITransformer_ReasoningContentPreservedWithKimi(t *testing.T) {
	buf := &bytes.Buffer{}
	tr := NewOpenAITransformer(buf)
	tr.SetKimiToolCallTransform(true)

	chunks := []types.Chunk{
		{
			ID:    "chatcmpl-test",
			Model: "kimi-k2.5",
			Choices: []types.Choice{{
				Delta: types.Delta{Reasoning: "deep thought"},
			}},
		},
		{
			ID:    "chatcmpl-test",
			Model: "kimi-k2.5",
			Choices: []types.Choice{{
				Delta: types.Delta{Content: "response"},
			}},
		},
	}

	for _, chunk := range chunks {
		data, _ := json.Marshal(chunk)
		event := &sse.Event{Data: string(data)}
		if err := tr.Transform(event); err != nil {
			t.Fatalf("Transform failed: %v", err)
		}
	}

	if err := tr.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	output := buf.String()
	t.Logf("Output:\n%s", output)

	if !strings.Contains(output, `"reasoning_content":"deep thought"`) {
		t.Errorf("expected reasoning_content:deep thought")
	}
	if strings.Contains(output, `"content":"deep thought"`) {
		t.Errorf("reasoning should not be content")
	}
}

func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return data
}
