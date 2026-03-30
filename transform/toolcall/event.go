package toolcall

// Event represents a parsed event from the tool call stream.
// Events are emitted by the Parser as it processes input text.
//
// @brief Data structure representing a parsed tool call or content event.
//
// @note Only certain fields are valid for each EventType:
//   - EventContent: Text
//   - EventToolStart: ID, Name, Index
//   - EventToolArgs: Args, Index
//   - EventToolEnd: Index
//   - EventSectionEnd: (no fields)
//
// @note Event instances should not be reused across parsing calls.
//
//	Each Parse call returns new Event instances.
type Event struct {
	// Type identifies the kind of event.
	// Determines which other fields are valid.
	Type EventType

	// Text contains the content string for EventContent events.
	// Empty for all other event types.
	Text string

	// ID contains the tool call identifier for EventToolStart events.
	// Format: "call_<index>_<timestamp>" or original ID from LLM.
	// Empty for all other event types.
	ID string

	// Name contains the function name for EventToolStart events.
	// This is the name of the function being called.
	// Empty for all other event types.
	Name string

	// Args contains the argument data for EventToolArgs events.
	// This is a fragment of the JSON arguments string.
	// Multiple EventToolArgs may be emitted for streaming arguments.
	// Empty for all other event types.
	Args string

	// Index contains the zero-based tool call index.
	// Valid for EventToolStart, EventToolArgs, and EventToolEnd events.
	// Incremented for each new tool call in a section.
	Index int
}
