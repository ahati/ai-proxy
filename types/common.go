package types

import (
	"fmt"
	"strings"
	"time"
)

type ToolCall struct {
	ID       string       `json:"id,omitempty"`
	Type     string       `json:"type,omitempty"`
	Index    int          `json:"index"`
	Function ToolFunction `json:"function"`
}

type ToolFunction struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

func NormalizeToolID(raw string, index int) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "call_") {
		return raw
	}
	return fmt.Sprintf("call_%d_%d", index, time.Now().UnixMilli())
}

func ParseFunctionName(raw string) string {
	raw = strings.TrimSpace(raw)
	if i := strings.LastIndex(raw, ":"); i >= 0 {
		raw = raw[:i]
	}
	if i := strings.LastIndex(raw, "."); i >= 0 {
		raw = raw[i+1:]
	}
	return raw
}
