package transform

import (
	"encoding/json"
)

// Helper functions for SSE parsing

// findSSEEnd finds the end of an SSE event (double newline).
// Returns the index after the \n\n, or -1 if not found.
func findSSEEnd(data []byte) int {
	for i := 0; i < len(data)-1; i++ {
		if data[i] == '\n' && data[i+1] == '\n' {
			return i + 2
		}
	}
	return -1
}

// extractSSEData extracts the JSON data from an SSE event.
// Returns (json, false) for data events, ("", true) for [DONE].
func extractSSEData(event []byte) (string, bool) {
	// Look for "data: " prefix
	dataPrefix := []byte("data: ")
	start := 0
	for i := 0; i <= len(event)-len(dataPrefix); i++ {
		if string(event[i:i+len(dataPrefix)]) == string(dataPrefix) {
			start = i + len(dataPrefix)
			break
		}
	}

	if start == 0 {
		return "", false
	}

	// Extract content until newline
	data := event[start:]
	end := len(data)
	for i, b := range data {
		if b == '\n' {
			end = i
			break
		}
	}

	content := string(data[:end])

	// Check for [DONE]
	if content == "[DONE]" {
		return "", true
	}

	return content, false
}

// extractAnthropicSSEData extracts JSON from an Anthropic SSE event.
// Handles both "event: type\ndata: {...}\n\n" and "data: {...}\n\n" formats.
func extractAnthropicSSEData(event []byte) string {
	// Look for "data: " in the event
	dataPrefix := []byte("data: ")
	dataStart := -1
	for i := 0; i <= len(event)-len(dataPrefix); i++ {
		if string(event[i:i+len(dataPrefix)]) == string(dataPrefix) {
			dataStart = i + len(dataPrefix)
			break
		}
	}

	if dataStart == -1 {
		return ""
	}

	// Extract content until end of line
	data := event[dataStart:]
	end := len(data)
	for i, b := range data {
		if b == '\n' {
			end = i
			break
		}
	}

	return string(data[:end])
}

// extractEventType extracts the "type" field value from a JSON object.
// Returns empty string if not found.
func extractEventType(jsonStr string) string {
	var obj struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
		return ""
	}
	return obj.Type
}

// ExtractJSONFromSSE extracts the JSON payload from OpenAI-style SSE data.
// Input: "data: {...}\n\n" → Output: "{...}"
// Returns empty string if no valid data found.
//
// @brief Exported helper for extracting JSON from OpenAI SSE-formatted data.
//
// @param data SSE-formatted bytes, typically from SSEWriter.
//
// @return string The extracted JSON, or empty string.
func ExtractJSONFromSSE(data []byte) string {
	jsonData, _ := extractSSEData(data)
	return jsonData
}

// ExtractAnthropicEventFromSSE extracts event type and JSON from Anthropic SSE data.
// Input: "event: type\ndata: {...}\n\n" → Output: "type", "{...}"
//
// @brief Exported helper for extracting event type and JSON from Anthropic SSE.
//
// @param data SSE-formatted bytes with optional event type line.
//
// @return eventType The event type string (e.g., "message_start").
// @return jsonData The JSON data payload.
func ExtractAnthropicEventFromSSE(data []byte) (string, string) {
	var eventType, jsonData string

	lines := splitSSELines(data)
	for _, line := range lines {
		if len(line) > 6 && string(line[:6]) == "event:" {
			eventType = string(line[7:])
		} else if len(line) > 5 && string(line[:5]) == "data:" {
			jsonData = string(line[6:])
		}
	}

	return eventType, jsonData
}

// splitSSELines splits SSE data into individual lines.
func splitSSELines(data []byte) [][]byte {
	var lines [][]byte
	start := 0
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			if i > start {
				lines = append(lines, data[start:i])
			}
			start = i + 1
		}
	}
	if start < len(data) {
		lines = append(lines, data[start:])
	}
	return lines
}