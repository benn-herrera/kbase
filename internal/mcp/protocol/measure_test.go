package protocol

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

var (
	surfaceFrames = flag.String("surface.frames", "", "the frames kbase mcp answered, one per line")
	surfaceOut    = flag.String("surface.out", "", "the file the served surface's counts are written to")
)

// TestMeasureMCPSurface writes the served surface's size from the frames a
// server answered to initialize, tools/list and resources/list: the tools
// listed, the tools/list result's JSON length, every input schema's JSON
// length summed, and the instructions' length, each length in characters. It
// is a measurement, not a gate: it fails only where one of those requests has
// no result among the frames or a count is not positive.
func TestMeasureMCPSurface(t *testing.T) {
	if *surfaceFrames == "" || *surfaceOut == "" {
		t.Skip("no frames named; the measure-mcp-surface recipe supplies them")
	}
	data, err := os.ReadFile(*surfaceFrames)
	if err != nil {
		t.Fatal(err)
	}
	// Each result is told by the key its method answers with, so the frames'
	// ids and order are the recipe's to choose.
	byKey := map[string]json.RawMessage{}
	var toolsResult json.RawMessage
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var frame struct {
			Result json.RawMessage `json:"result"`
		}
		var result map[string]json.RawMessage
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("a frame that is no JSON-RPC message: %v\n%s", err, line)
		}
		if frame.Result == nil {
			continue
		}
		if err := json.Unmarshal(frame.Result, &result); err != nil {
			t.Fatalf("a result that is no object: %v\n%s", err, line)
		}
		for _, key := range []string{"instructions", "tools", "resources"} {
			if raw, ok := result[key]; ok {
				byKey[key] = raw
			}
		}
		if _, ok := result["tools"]; ok {
			toolsResult = frame.Result
		}
	}
	var instructions string
	var tools []struct {
		InputSchema json.RawMessage `json:"inputSchema"`
	}
	for key, into := range map[string]any{"instructions": &instructions, "tools": &tools, "resources": &[]any{}} {
		raw, ok := byKey[key]
		if !ok {
			t.Fatalf("no frame answers with %s: the server did not answer the request", key)
		}
		if err := json.Unmarshal(raw, into); err != nil {
			t.Fatalf("decoding %s: %v", key, err)
		}
	}
	schemaChars := 0
	for _, tool := range tools {
		schemaChars += utf8.RuneCount(tool.InputSchema)
	}
	var out strings.Builder
	for _, count := range []struct {
		key string
		n   int
	}{
		{"tools", len(tools)},
		{"tools-list-chars", utf8.RuneCount(toolsResult)},
		{"input-schema-chars", schemaChars},
		{"instructions-chars", utf8.RuneCountInString(instructions)},
	} {
		if count.n <= 0 {
			t.Errorf("%s is %d, want a positive count", count.key, count.n)
		}
		fmt.Fprintf(&out, "%s: %d\n", count.key, count.n)
	}
	if err := os.WriteFile(*surfaceOut, []byte(out.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}
