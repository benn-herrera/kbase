package main

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/mcp"
)

var (
	helpFrames = flag.String("help.frames", "", "the frames kbase mcp answered to one resources/read per help resource")
	helpDir    = flag.String("help.dir", "", "the directory holding <command>.txt, what `kbase <command> --help` printed")
)

// TestHelpResourcesAreTheBinarysHelp checks each help resource the binary
// served against the file of what the binary printed for that command's
// --help. It skips unless the test-integration-mcp recipe names both.
func TestHelpResourcesAreTheBinarysHelp(t *testing.T) {
	if *helpFrames == "" || *helpDir == "" {
		t.Skip("no frames named; the test-integration-mcp recipe supplies them")
	}
	data, err := os.ReadFile(*helpFrames)
	if err != nil {
		t.Fatal(err)
	}
	served := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var frame struct {
			Result struct {
				Contents []struct {
					URI  string `json:"uri"`
					Text string `json:"text"`
				} `json:"contents"`
			} `json:"result"`
		}
		if err := json.Unmarshal([]byte(line), &frame); err != nil {
			t.Fatalf("a frame that is no JSON-RPC message: %v\n%s", err, line)
		}
		if len(frame.Result.Contents) == 0 {
			continue // the initialize answer, or a failed read, which leaves its name unserved
		}
		c := frame.Result.Contents[0]
		served[strings.TrimPrefix(c.URI, mcp.HelpURIPrefix)] = c.Text
	}
	files, err := filepath.Glob(filepath.Join(*helpDir, "*.txt"))
	if err != nil {
		t.Fatal(err)
	}
	printed := map[string]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		printed[strings.TrimSuffix(filepath.Base(f), ".txt")] = string(b)
	}
	if len(served) == 0 {
		t.Fatal("no help resource was read")
	}
	for name, text := range served {
		want, ok := printed[name]
		switch {
		case !ok:
			t.Errorf("%s: a resource with no --help file", name)
		case text != want:
			t.Errorf("%s: the resource differs from `kbase %s --help`", name, name)
		}
	}
	for name := range printed {
		if _, ok := served[name]; !ok {
			t.Errorf("%s: a --help file with no resource frame", name)
		}
	}
}
