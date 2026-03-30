package toolcall

import (
	"encoding/json"
	"fmt"

	"ai-proxy/types"
)

func intPtr(i int) *int {
	return &i
}

func serializeAnthropicEvent(event types.Event) []byte {
	data, err := json.Marshal(event)
	if err != nil {
		// Return minimal valid event on marshal error
		return []byte(fmt.Sprintf("event: %s\ndata: {}\n\n", event.Type))
	}
	return []byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event.Type, string(data)))
}

func marshalJSON(v interface{}) []byte {
	data, err := json.Marshal(v)
	if err != nil {
		return []byte("{}")
	}
	return data
}
