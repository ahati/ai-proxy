package convert

import (
	"encoding/json"
	"strings"
)

// NormalizeWebSearchToolResultsInMessages converts web_search_tool_result blocks
// to tool_result blocks in a request body for compatibility with providers that
// don't support web_search_tool_result natively.
//
// This is needed because Claude Code sends web_search_tool_result in follow-up
// requests, but most upstream providers only understand tool_result.
func NormalizeWebSearchToolResultsInMessages(body []byte) []byte {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}

	messages, ok := req["messages"].([]interface{})
	if !ok {
		return body
	}

	modified := false
	for i, msg := range messages {
		msgMap, ok := msg.(map[string]interface{})
		if !ok {
			continue
		}

		content, ok := msgMap["content"].([]interface{})
		if !ok {
			continue
		}

		for j, block := range content {
			blockMap, ok := block.(map[string]interface{})
			if !ok {
				continue
			}

			blockType, _ := blockMap["type"].(string)
			if blockType == "web_search_tool_result" {
				newBlock := map[string]interface{}{
					"type":        "tool_result",
					"tool_use_id": blockMap["tool_use_id"],
					"content":     ConvertWebSearchContentToText(blockMap["content"]),
				}
				if isError, ok := blockMap["is_error"].(bool); ok && isError {
					newBlock["is_error"] = true
				}
				content[j] = newBlock
				modified = true
			}
		}
		msgMap["content"] = content
		messages[i] = msgMap
	}

	if !modified {
		return body
	}

	req["messages"] = messages
	result, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return result
}

// ConvertWebSearchContentToText converts web_search_result objects to text content.
// This handles the conversion from Anthropic's web_search_tool_result content format
// to a format that can be understood by providers that don't support it natively.
//
// Each web_search_result contains:
//   - type: "web_search_result"
//   - title: page title
//   - url: page URL
//   - encrypted_content: encrypted page content (Anthropic native)
//   - content: plain page content (legacy proxy format)
func ConvertWebSearchContentToText(content interface{}) interface{} {
	if content == nil {
		return nil
	}

	switch c := content.(type) {
	case string:
		return c
	case []interface{}:
		var parts []string
		for _, item := range c {
			if b, ok := item.(map[string]interface{}); ok {
				parts = append(parts, formatWebSearchResultBlock(b))
			}
		}
		if len(parts) == 1 {
			return parts[0]
		}
		return strings.Join(parts, "\n\n")
	default:
		return content
	}
}

// formatWebSearchResultBlock formats a single content block from a web_search_tool_result.
func formatWebSearchResultBlock(b map[string]interface{}) string {
	blockType, _ := b["type"].(string)

	switch blockType {
	case "web_search_result":
		title, _ := b["title"].(string)
		url, _ := b["url"].(string)
		// Check encrypted_content (Anthropic native) first, then fall back to content (legacy)
		contentText, _ := b["encrypted_content"].(string)
		if contentText == "" {
			contentText, _ = b["content"].(string)
		}

		var part string
		if title != "" {
			part = "## " + title + "\n"
		}
		if url != "" {
			part += "URL: " + url + "\n"
		}
		if contentText != "" {
			part += contentText
		}
		return part

	case "web_search_tool_result_error":
		errorCode, _ := b["error_code"].(string)
		return "Search error: " + errorCode

	case "text":
		if text, ok := b["text"].(string); ok {
			return text
		}
		return ""

	default:
		return ""
	}
}

// ConvertServerWebSearchToFunctionTool converts web_search_20250305 server tools
// to regular function tools in the request body.
//
// Anthropic's web_search_20250305 is a server-side tool that Anthropic's API
// executes internally (emitting server_tool_use + web_search_tool_result blocks).
// Non-Anthropic providers don't support this, so we convert it to a regular
// function tool that the model can call, and the proxy's web search transformer
// will intercept the tool_use and inject the search results.
//
// This also converts any server_tool_use blocks in message history to regular
// tool_use blocks, since non-Anthropic providers don't understand server_tool_use.
func ConvertServerWebSearchToFunctionTool(body []byte) []byte {
	var req map[string]interface{}
	if err := json.Unmarshal(body, &req); err != nil {
		return body
	}

	modified := false

	// Convert tools: web_search_20250305 -> function tool
	if tools, ok := req["tools"].([]interface{}); ok {
		for i, tool := range tools {
			toolMap, ok := tool.(map[string]interface{})
			if !ok {
				continue
			}
			toolType, _ := toolMap["type"].(string)
			if toolType == "web_search_20250305" || toolType == "web_search_20250209" {
				// Convert to a regular function tool
				toolMap["type"] = "custom"
				// Keep the name "web_search" and add an input_schema
				if _, hasSchema := toolMap["input_schema"]; !hasSchema {
					toolMap["input_schema"] = map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"query": map[string]interface{}{
								"type":        "string",
								"description": "The search query to use",
								"minLength":   2,
							},
							"allowed_domains": map[string]interface{}{
								"type":        "array",
								"items":       map[string]interface{}{"type": "string"},
								"description": "Only include search results from these domains",
							},
							"blocked_domains": map[string]interface{}{
								"type":        "array",
								"items":       map[string]interface{}{"type": "string"},
								"description": "Never include search results from these domains",
							},
						},
						"required":             []string{"query"},
						"additionalProperties": false,
					}
				}
				// Remove server-tool-specific fields
				delete(toolMap, "max_uses")
				delete(toolMap, "user_location")
				tools[i] = toolMap
				modified = true
			}
		}
		req["tools"] = tools
	}

	// Convert server_tool_use blocks in message history to tool_use
	if messages, ok := req["messages"].([]interface{}); ok {
		for i, msg := range messages {
			msgMap, ok := msg.(map[string]interface{})
			if !ok {
				continue
			}
			content, ok := msgMap["content"].([]interface{})
			if !ok {
				continue
			}
			for j, block := range content {
				blockMap, ok := block.(map[string]interface{})
				if !ok {
					continue
				}
				blockType, _ := blockMap["type"].(string)
				if blockType == "server_tool_use" {
					blockMap["type"] = "tool_use"
					content[j] = blockMap
					modified = true
				}
			}
			msgMap["content"] = content
			messages[i] = msgMap
		}
		req["messages"] = messages
	}

	if !modified {
		return body
	}

	result, err := json.Marshal(req)
	if err != nil {
		return body
	}
	return result
}
