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
//   - content: page content
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
		contentText, _ := b["content"].(string)

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
