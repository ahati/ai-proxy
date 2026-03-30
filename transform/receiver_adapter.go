package transform

import (
	"encoding/json"
	"fmt"
	"io"
)

// ChatToSSEAdapter wraps an OpenAIChatReceiver and provides an io.Writer interface.
// It parses SSE-formatted data and forwards the JSON payload to the receiver.
//
// @brief Adapter that converts SSE-formatted output to OpenAIChatReceiver calls.
//
// @note This adapter enables the chain pattern:
//
//	Transformer → SSEWriter → ChatToSSEAdapter → OpenAIChatReceiver
//
// @note The input is expected to be SSE-formatted: "data: {...}\n\n"
// The adapter extracts the JSON and calls receiver.Receive(json).
type ChatToSSEAdapter struct {
	receiver OpenAIChatReceiver
	buffer   []byte
}

// NewChatToSSEAdapter creates an adapter that forwards parsed chunks to a receiver.
//
// @brief Creates a new ChatToSSEAdapter wrapping the given receiver.
//
// @param receiver The OpenAIChatReceiver to forward parsed chunks to.
//
// @return *ChatToSSEAdapter A new adapter instance.
//
// @pre receiver must not be nil.
// @post The adapter is ready to receive SSE-formatted data.
func NewChatToSSEAdapter(receiver OpenAIChatReceiver) *ChatToSSEAdapter {
	return &ChatToSSEAdapter{
		receiver: receiver,
	}
}

// Write parses SSE-formatted data and forwards JSON to the receiver.
//
// @brief Implements io.Writer by parsing SSE and forwarding JSON.
//
// @param p SSE-formatted data, typically "data: {...}\n\n".
//
// @return int The number of bytes accepted (always len(p)).
// @return error Returns error if parsing or receiver fails.
//
// @note This method handles:
//   - "data: {...}\n\n" → extracts JSON, calls receiver.Receive(json)
//   - "data: [DONE]\n\n" → calls receiver.ReceiveDone()
func (a *ChatToSSEAdapter) Write(p []byte) (int, error) {
	a.buffer = append(a.buffer, p...)

	for {
		// Look for complete SSE event (ends with \n\n)
		idx := findSSEEnd(a.buffer)
		if idx == -1 {
			break
		}

		event := a.buffer[:idx]
		a.buffer = a.buffer[idx:]

		// Parse "data: ..." format
		jsonData, done := extractSSEData(event)
		if done {
			if err := a.receiver.ReceiveDone(); err != nil {
				return len(p), err
			}
			continue
		}

		if jsonData != "" {
			if err := a.receiver.Receive(jsonData); err != nil {
				return len(p), err
			}
		}
	}

	return len(p), nil
}

// Flush flushes the underlying receiver.
//
// @brief Flushes any buffered data in the receiver.
func (a *ChatToSSEAdapter) Flush() error {
	return a.receiver.Flush()
}

// AnthropicToSSEAdapter wraps an AnthropicEventReceiver and provides an io.Writer interface.
// It parses SSE-formatted Anthropic events and forwards the JSON payload to the receiver.
//
// @brief Adapter that converts SSE-formatted Anthropic output to AnthropicEventReceiver calls.
type AnthropicToSSEAdapter struct {
	receiver AnthropicEventReceiver
	buffer   []byte
}

// NewAnthropicToSSEAdapter creates an adapter for Anthropic events.
//
// @brief Creates a new AnthropicToSSEAdapter wrapping the given receiver.
func NewAnthropicToSSEAdapter(receiver AnthropicEventReceiver) *AnthropicToSSEAdapter {
	return &AnthropicToSSEAdapter{
		receiver: receiver,
	}
}

// Write parses SSE-formatted Anthropic events and forwards JSON to the receiver.
//
// @brief Implements io.Writer by parsing SSE and forwarding JSON.
//
// @note Handles both "event: type\ndata: {...}\n\n" and "data: {...}\n\n" formats.
func (a *AnthropicToSSEAdapter) Write(p []byte) (int, error) {
	a.buffer = append(a.buffer, p...)

	for {
		idx := findSSEEnd(a.buffer)
		if idx == -1 {
			break
		}

		event := a.buffer[:idx]
		a.buffer = a.buffer[idx:]

		// Extract JSON from SSE event
		jsonData := extractAnthropicSSEData(event)
		if jsonData != "" {
			if err := a.receiver.Receive(jsonData); err != nil {
				return len(p), err
			}
		}
	}

	return len(p), nil
}

// Flush flushes the underlying receiver.
func (a *AnthropicToSSEAdapter) Flush() error {
	return a.receiver.Flush()
}

// SSEChatWriter wraps an io.Writer and implements OpenAIChatReceiver.
// It formats chunks as SSE events and writes them to the underlying writer.
//
// @brief Adapter that implements OpenAIChatReceiver and writes SSE-formatted output.
//
// @note This is used when the final output should be SSE-formatted OpenAI chunks.
type SSEChatWriter struct {
	writer io.Writer
}

// NewSSEChatWriter creates an OpenAIChatReceiver that writes SSE to the given writer.
//
// @brief Creates a new SSEChatWriter wrapping the given writer.
//
// @param writer The io.Writer to write SSE-formatted chunks to.
//
// @return *SSEChatWriter A new OpenAIChatReceiver implementation.
func NewSSEChatWriter(writer io.Writer) *SSEChatWriter {
	return &SSEChatWriter{
		writer: writer,
	}
}

// Receive writes the chunk JSON as an SSE data event.
//
// @brief Implements OpenAIChatReceiver.Receive by writing SSE-formatted output.
//
// @param chunkJSON The raw JSON of a types.Chunk.
//
// @return error Returns error if the underlying write fails.
func (w *SSEChatWriter) Receive(chunkJSON string) error {
	_, err := fmt.Fprintf(w.writer, "data: %s\n\n", chunkJSON)
	return err
}

// ReceiveDone writes the SSE [DONE] marker.
//
// @brief Implements OpenAIChatReceiver.ReceiveDone by writing "data: [DONE]\n\n".
func (w *SSEChatWriter) ReceiveDone() error {
	_, err := w.writer.Write([]byte("data: [DONE]\n\n"))
	return err
}

// Flush is a no-op for SSEChatWriter.
//
// @brief Implements OpenAIChatReceiver.Flush (no-op).
func (w *SSEChatWriter) Flush() error {
	return nil
}

// SSEAnthropicWriter wraps an io.Writer and implements AnthropicEventReceiver.
// It formats Anthropic events as SSE and writes them to the underlying writer.
//
// @brief Adapter that implements AnthropicEventReceiver and writes SSE-formatted output.
type SSEAnthropicWriter struct {
	writer io.Writer
}

// NewSSEAnthropicWriter creates an AnthropicEventReceiver that writes SSE to the given writer.
//
// @brief Creates a new SSEAnthropicWriter wrapping the given writer.
func NewSSEAnthropicWriter(writer io.Writer) *SSEAnthropicWriter {
	return &SSEAnthropicWriter{
		writer: writer,
	}
}

// Receive writes the event JSON as an SSE event.
//
// @brief Implements AnthropicEventReceiver.Receive by writing SSE-formatted output.
//
// @note Anthropic events include an event type, so we parse it and write "event: type\ndata: ...\n\n".
func (w *SSEAnthropicWriter) Receive(eventJSON string) error {
	// Extract event type from JSON
	eventType := extractEventType(eventJSON)
	if eventType != "" {
		_, err := fmt.Fprintf(w.writer, "event: %s\ndata: %s\n\n", eventType, eventJSON)
		return err
	}
	_, err := fmt.Fprintf(w.writer, "data: %s\n\n", eventJSON)
	return err
}

// ReceiveDone writes the message_stop event for Anthropic streams.
//
// @brief Implements AnthropicEventReceiver.ReceiveDone by writing message_stop.
func (w *SSEAnthropicWriter) ReceiveDone() error {
	_, err := w.writer.Write([]byte("event: message_stop\ndata: {}\n\n"))
	return err
}

// Flush is a no-op for SSEAnthropicWriter.
func (w *SSEAnthropicWriter) Flush() error {
	return nil
}

// Helper functions

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
