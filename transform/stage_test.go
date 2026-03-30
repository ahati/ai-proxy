package transform

import (
	"errors"
	"testing"
)

// mockStage is a test double for Stage that records Process calls.
type mockStage struct {
	events    []PipelineEvent
	initCalls int
	flushErr  error
	closeErr  error
}

func (m *mockStage) Process(event PipelineEvent) error {
	m.events = append(m.events, event)
	return nil
}

func (m *mockStage) Initialize() error {
	m.initCalls++
	return nil
}

func (m *mockStage) Flush() error {
	return m.flushErr
}

func (m *mockStage) Close() error {
	return m.closeErr
}

// errStage always returns an error from Process.
type errStage struct {
	err error
}

func (e *errStage) Process(_ PipelineEvent) error { return e.err }
func (e *errStage) Initialize() error             { return nil }
func (e *errStage) Flush() error                  { return nil }
func (e *errStage) Close() error                  { return nil }

// TestPipeline_Process verifies events flow through all stages.
//
// @brief Tests that Process sends events to all stages in order.
func TestPipeline_Process(t *testing.T) {
	s1 := &mockStage{}
	s2 := &mockStage{}
	p := NewPipeline(s1, s2)

	event := PipelineEvent{Type: EventOpenAIChunk, Data: []byte(`{"id":"test"}`)}
	if err := p.Process(event); err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	// Both stages should receive the event
	if len(s1.events) != 1 {
		t.Errorf("stage1 received %d events, want 1", len(s1.events))
	}
	if len(s2.events) != 1 {
		t.Errorf("stage2 received %d events, want 1", len(s2.events))
	}
	if s1.events[0].Type != EventOpenAIChunk {
		t.Errorf("stage1 event type = %v, want EventOpenAIChunk", s1.events[0].Type)
	}
	if s2.events[0].Type != EventOpenAIChunk {
		t.Errorf("stage2 event type = %v, want EventOpenAIChunk", s2.events[0].Type)
	}
}

// TestPipeline_Process_StopsOnError verifies that processing stops
// when a stage returns an error.
//
// @brief Tests error propagation stops the pipeline.
func TestPipeline_Process_StopsOnError(t *testing.T) {
	expectedErr := errors.New("stage error")
	s1 := &errStage{err: expectedErr}
	s2 := &mockStage{}
	p := NewPipeline(s1, s2)

	event := PipelineEvent{Type: EventOpenAIChunk, Data: []byte("test")}
	err := p.Process(event)
	if err == nil {
		t.Fatal("Process() should return error")
	}

	// Second stage should NOT receive the event
	if len(s2.events) != 0 {
		t.Errorf("stage2 received %d events, want 0 (pipeline should stop on error)", len(s2.events))
	}
}

// TestPipeline_Initialize verifies Initialize is called on all stages.
//
// @brief Tests that Initialize delegates to all stages.
func TestPipeline_Initialize(t *testing.T) {
	s1 := &mockStage{}
	s2 := &mockStage{}
	p := NewPipeline(s1, s2)

	if err := p.Initialize(); err != nil {
		t.Fatalf("Initialize() error = %v", err)
	}

	if s1.initCalls != 1 {
		t.Errorf("stage1 initCalls = %d, want 1", s1.initCalls)
	}
	if s2.initCalls != 1 {
		t.Errorf("stage2 initCalls = %d, want 1", s2.initCalls)
	}
}

// TestPipeline_Flush verifies Flush is called on all stages.
//
// @brief Tests that Flush delegates to all stages.
func TestPipeline_Flush(t *testing.T) {
	s1 := &mockStage{}
	s2 := &mockStage{}
	p := NewPipeline(s1, s2)

	if err := p.Flush(); err != nil {
		t.Fatalf("Flush() error = %v", err)
	}
}

// TestPipeline_Flush_Error verifies Flush error propagation.
//
// @brief Tests that Flush returns the first error.
func TestPipeline_Flush_Error(t *testing.T) {
	s1 := &mockStage{flushErr: errors.New("flush error")}
	s2 := &mockStage{}
	p := NewPipeline(s1, s2)

	err := p.Flush()
	if err == nil {
		t.Fatal("Flush() should return error")
	}
}

// TestPipeline_Close verifies Close is called in reverse order.
//
// @brief Tests that Close calls stages in reverse order.
func TestPipeline_Close(t *testing.T) {
	s1 := &mockStage{}
	s2 := &mockStage{}
	p := NewPipeline(s1, s2)

	if err := p.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
}

// TestPipeline_Close_Error verifies Close error propagation.
//
// @brief Tests that Close returns the first error from any stage.
func TestPipeline_Close_Error(t *testing.T) {
	s1 := &mockStage{}
	s2 := &mockStage{closeErr: errors.New("close error")}
	p := NewPipeline(s1, s2)

	err := p.Close()
	if err == nil {
		t.Fatal("Close() should return error")
	}
}

// TestPipeline_Stages verifies the Stages accessor.
//
// @brief Tests that Stages() returns the stages in order.
func TestPipeline_Stages(t *testing.T) {
	s1 := &mockStage{}
	s2 := &mockStage{}
	p := NewPipeline(s1, s2)

	stages := p.Stages()
	if len(stages) != 2 {
		t.Fatalf("Stages() returned %d stages, want 2", len(stages))
	}
	if stages[0] != s1 {
		t.Error("Stages()[0] != s1")
	}
	if stages[1] != s2 {
		t.Error("Stages()[1] != s2")
	}
}

// TestPipeline_MultipleEvents verifies processing multiple events in sequence.
//
// @brief Tests that pipeline handles a stream of events correctly.
func TestPipeline_MultipleEvents(t *testing.T) {
	s := &mockStage{}
	p := NewPipeline(s)

	events := []PipelineEvent{
		{Type: EventOpenAIChunk, Data: []byte(`{"id":"1"}`)},
		{Type: EventOpenAIChunk, Data: []byte(`{"id":"2"}`)},
		{Type: EventDone},
	}

	for _, e := range events {
		if err := p.Process(e); err != nil {
			t.Fatalf("Process() error = %v", err)
		}
	}

	if len(s.events) != 3 {
		t.Errorf("stage received %d events, want 3", len(s.events))
	}
	if s.events[0].Type != EventOpenAIChunk {
		t.Errorf("event 0 type = %v, want EventOpenAIChunk", s.events[0].Type)
	}
	if s.events[2].Type != EventDone {
		t.Errorf("event 2 type = %v, want EventDone", s.events[2].Type)
	}
}
