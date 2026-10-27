package modeltest_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"kbase/internal/model"
	"kbase/internal/model/modeltest"
)

// TestMockListModels: the configured listing comes back by value, and an
// unconfigured mock reports an empty list rather than nil.
func TestMockListModels(t *testing.T) {
	want := []model.ModelInfo{{ID: "gemma-4-31b-it"}, {ID: "gemma-4-26b-a4b-it"}}
	m := modeltest.NewScriptedMock(nil, want)
	got, err := m.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels: %v", err)
	}
	if len(got) != len(want) {
		t.Fatalf("models: got %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("models[%d]: got %+v, want %+v", i, got[i], want[i])
		}
	}
	// Mutating the returned slice must not reach the mock's copy.
	got[0] = model.ModelInfo{ID: "tampered"}
	again, _ := m.ListModels(context.Background())
	if again[0] != want[0] {
		t.Errorf("ListModels returned an aliased slice: %+v", again[0])
	}

	empty, err := modeltest.NewScriptedMock(nil, nil).ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels (no models): %v", err)
	}
	if empty == nil || len(empty) != 0 {
		t.Errorf("unconfigured ListModels: got %+v, want an empty slice", empty)
	}
}

func TestMockScriptedStreamSplitsResponse(t *testing.T) {
	scripted := []model.Response{
		{Content: "abcdefghijklmnop", FinishReason: "stop", Usage: model.Usage{TotalTokens: 99}},
	}
	m := modeltest.NewScriptedMock(scripted, nil)
	m.SetMockChunks(4)

	sr, err := m.ConsultStream(context.Background(), model.Request{})
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	defer sr.Close()

	var got strings.Builder
	chunks := 0
	var lastChunk model.Chunk
	for {
		chunk, err := sr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		got.WriteString(chunk.Content)
		chunks++
		lastChunk = chunk
	}
	if chunks != 4 {
		t.Errorf("chunks: got %d, want 4", chunks)
	}
	if got.String() != scripted[0].Content {
		t.Errorf("concat: got %q, want %q", got.String(), scripted[0].Content)
	}
	// Final chunk carries the metadata.
	if lastChunk.FinishReason != "stop" {
		t.Errorf("last chunk finish_reason: got %q, want stop", lastChunk.FinishReason)
	}
	if lastChunk.Usage.TotalTokens != 99 {
		t.Errorf("last chunk usage: got %d, want 99", lastChunk.Usage.TotalTokens)
	}

	final := sr.Final()
	if final.Content != scripted[0].Content {
		t.Errorf("Final.Content: got %q, want %q", final.Content, scripted[0].Content)
	}
	if final.FinishReason != "stop" {
		t.Errorf("Final.FinishReason: got %q, want stop", final.FinishReason)
	}
}

func TestMockStreamCloseIdempotent(t *testing.T) {
	m := modeltest.NewScriptedMock([]model.Response{{Content: "x"}}, nil)
	sr, err := m.ConsultStream(context.Background(), model.Request{})
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	if err := sr.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := sr.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
	if _, err := sr.Next(); !errors.Is(err, io.EOF) {
		t.Errorf("Next after Close: got %v, want io.EOF", err)
	}
}

func TestMockStreamCtxCancelBetweenChunks(t *testing.T) {
	m := modeltest.NewScriptedMock([]model.Response{{Content: "abcdefgh"}}, nil)
	m.SetMockChunks(4)
	ctx, cancel := context.WithCancel(context.Background())
	sr, err := m.ConsultStream(ctx, model.Request{})
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	defer sr.Close()
	// One chunk delivered, then cancel; the next Next must report ctx err.
	if _, err := sr.Next(); err != nil {
		t.Fatalf("first Next: %v", err)
	}
	cancel()
	_, err = sr.Next()
	if !errors.Is(err, context.Canceled) {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestMockStreamDefaultChunkCount(t *testing.T) {
	m := modeltest.NewScriptedMock([]model.Response{{Content: strings.Repeat("a", 64)}}, nil)
	// No SetMockChunks call → modeltest.DefaultMockChunks.
	sr, err := m.ConsultStream(context.Background(), model.Request{})
	if err != nil {
		t.Fatalf("ConsultStream: %v", err)
	}
	defer sr.Close()
	chunks := 0
	for {
		_, err := sr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		chunks++
	}
	if chunks != modeltest.DefaultMockChunks {
		t.Errorf("chunk count: got %d, want %d", chunks, modeltest.DefaultMockChunks)
	}
}
