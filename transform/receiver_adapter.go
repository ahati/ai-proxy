package transform

// Helper functions for SSE parsing (used by exported functions below)

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

// Exported helper functions

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
