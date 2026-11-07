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
	surfaceFrames  = flag.String("surface.frames", "", "the frames kbase mcp answered, one per line")
	surfaceLocated = flag.String("surface.located", "", "the frames a kbase mcp with a default project answered, tools/list among them")
	surfaceOut     = flag.String("surface.out", "", "the file the served surface's counts are written to")
)

// TestMeasureMCPSurface writes the served surface's size from the frames a
// server answered to initialize, tools/list and resources/list: the tools
// listed, the tools/list result's JSON length, the same for the tools/list a
// server with a default project answered, every input schema's JSON length
// summed, and the instructions' length; then, under per-tool, each tool's
// entry in the tools/list result, by name; each length in characters. It is a
// measurement, not a gate: it fails only where one of those requests has no
// result among the frames or a count is not positive.
func TestMeasureMCPSurface(t *testing.T) {
	if *surfaceFrames == "" || *surfaceLocated == "" || *surfaceOut == "" {
		t.Skip("no frames named; the measure-mcp-surface recipe supplies them")
	}
	byKey, toolsResult := resultsOf(t, *surfaceFrames)
	_, locatedTools := resultsOf(t, *surfaceLocated)
	if locatedTools == nil {
		t.Fatalf("no frame in %s answers with tools: the server did not answer tools/list", *surfaceLocated)
	}
	var instructions string
	var entries []json.RawMessage
	for key, into := range map[string]any{"instructions": &instructions, "tools": &entries, "resources": &[]any{}} {
		raw, ok := byKey[key]
		if !ok {
			t.Fatalf("no frame answers with %s: the server did not answer the request", key)
		}
		if err := json.Unmarshal(raw, into); err != nil {
			t.Fatalf("decoding %s: %v", key, err)
		}
	}
	type count struct {
		key string
		n   int
	}
	schemaChars := 0
	var perTool []count
	for _, entry := range entries {
		var tool struct {
			Name        string          `json:"name"`
			InputSchema json.RawMessage `json:"inputSchema"`
		}
		if err := json.Unmarshal(entry, &tool); err != nil {
			t.Fatalf("decoding a tool: %v\n%s", err, entry)
		}
		schemaChars += utf8.RuneCount(tool.InputSchema)
		perTool = append(perTool, count{tool.Name, utf8.RuneCount(entry)})
	}
	var out strings.Builder
	write := func(indent string, counts []count) {
		for _, c := range counts {
			if c.n <= 0 {
				t.Errorf("%s is %d, want a positive count", c.key, c.n)
			}
			fmt.Fprintf(&out, "%s%s: %d\n", indent, c.key, c.n)
		}
	}
	write("", []count{
		{"tools", len(entries)},
		{"tools-list-chars", utf8.RuneCount(toolsResult)},
		{"tools-list-chars-located", utf8.RuneCount(locatedTools)},
		{"input-schema-chars", schemaChars},
		{"instructions-chars", utf8.RuneCountInString(instructions)},
	})
	out.WriteString("per-tool:\n")
	write("  ", perTool)
	if err := os.WriteFile(*surfaceOut, []byte(out.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// resultsOf is each result in the frames file path holds, by the key its
// method answers with — instructions, tools or resources — so the frames' ids
// and order are the recipe's to choose; and the whole tools/list result, nil
// where none answers.
func resultsOf(t *testing.T, path string) (map[string]json.RawMessage, json.RawMessage) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
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
	return byKey, toolsResult
}
