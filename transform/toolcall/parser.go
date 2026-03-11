package toolcall

import (
	"strings"

	"ai-proxy/types"
)

type State int

const (
	StateIdle State = iota
	StateInSection
	StateReadingID
	StateReadingArgs
	StateTrailing
)

type TokenSet struct {
	SectionBegin string
	CallBegin    string
	ArgBegin     string
	CallEnd      string
	SectionEnd   string
}

func DefaultTokenSet() TokenSet {
	return TokenSet{
		SectionBegin: "<|tool_calls_section_begin|>",
		CallBegin:    "<|tool_call_begin|>",
		ArgBegin:     "<|tool_call_argument_begin|>",
		CallEnd:      "<|tool_call_end|>",
		SectionEnd:   "<|tool_calls_section_end|>",
	}
}

type EventHandler interface {
	OnText(text string)
	OnToolCallStart(id, name string, index int)
	OnToolCallArgs(args string, index int)
	OnToolCallEnd(index int)
}

type Parser struct {
	state     State
	buf       string
	tokens    TokenSet
	handler   EventHandler
	toolIdx   int
	currentID string
}

func NewParser(tokens TokenSet, handler EventHandler) *Parser {
	return &Parser{
		state:   StateIdle,
		tokens:  tokens,
		handler: handler,
	}
}

func (p *Parser) Feed(text string) {
	p.buf += text
	p.process()
}

func (p *Parser) Flush() {
	if p.buf != "" {
		switch p.state {
		case StateIdle, StateTrailing:
			p.handler.OnText(p.buf)
		case StateReadingArgs:
			p.handler.OnToolCallArgs(p.buf, p.toolIdx)
		}
		p.buf = ""
	}
}

func (p *Parser) Reset() {
	p.state = StateIdle
	p.buf = ""
	p.toolIdx = 0
	p.currentID = ""
}

func (p *Parser) process() {
	for {
		switch p.state {
		case StateIdle:
			idx := strings.Index(p.buf, p.tokens.SectionBegin)
			if idx < 0 {
				if p.buf != "" {
					p.handler.OnText(p.buf)
					p.buf = ""
				}
				return
			}
			if idx > 0 {
				p.handler.OnText(p.buf[:idx])
			}
			p.buf = p.buf[idx+len(p.tokens.SectionBegin):]
			p.state = StateInSection

		case StateInSection:
			idx := strings.Index(p.buf, p.tokens.CallBegin)
			endIdx := strings.Index(p.buf, p.tokens.SectionEnd)

			if endIdx >= 0 && (idx < 0 || endIdx < idx) {
				trailing := p.buf[endIdx+len(p.tokens.SectionEnd):]
				p.buf = ""
				p.state = StateTrailing
				if trailing != "" {
					p.handler.OnText(trailing)
					p.buf = ""
				}
				return
			}
			if idx < 0 {
				return
			}
			p.buf = p.buf[idx+len(p.tokens.CallBegin):]
			p.state = StateReadingID

		case StateReadingID:
			argIdx := strings.Index(p.buf, p.tokens.ArgBegin)
			if argIdx < 0 {
				return
			}
			rawID := strings.TrimSpace(p.buf[:argIdx])
			p.currentID = types.NormalizeToolID(rawID, p.toolIdx)
			name := types.ParseFunctionName(rawID)
			p.buf = p.buf[argIdx+len(p.tokens.ArgBegin):]
			p.state = StateReadingArgs
			p.handler.OnToolCallStart(p.currentID, name, p.toolIdx)

		case StateReadingArgs:
			endIdx := strings.Index(p.buf, p.tokens.CallEnd)
			if endIdx < 0 {
				if p.buf != "" {
					p.handler.OnToolCallArgs(p.buf, p.toolIdx)
					p.buf = ""
				}
				return
			}
			args := p.buf[:endIdx]
			if args != "" {
				p.handler.OnToolCallArgs(args, p.toolIdx)
			}
			p.handler.OnToolCallEnd(p.toolIdx)
			p.buf = p.buf[endIdx+len(p.tokens.CallEnd):]
			p.toolIdx++
			p.state = StateInSection

		case StateTrailing:
			idx := strings.Index(p.buf, p.tokens.SectionBegin)
			if idx >= 0 {
				if idx > 0 {
					p.handler.OnText(p.buf[:idx])
				}
				p.buf = p.buf[idx+len(p.tokens.SectionBegin):]
				p.state = StateInSection
				continue
			}
			if p.buf != "" {
				p.handler.OnText(p.buf)
				p.buf = ""
			}
			return
		}
	}
}
