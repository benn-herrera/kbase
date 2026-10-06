package claimgraph

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestReadBlocksReadsLabelLines: a line of kb's label-line fixture opens a
// block of the name the fixture gives it, and no block where it gives none.
func TestReadBlocksReadsLabelLines(t *testing.T) {
	data, err := os.ReadFile("../kb/testdata/label-lines.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		var c struct {
			Line string
			Name *string
		}
		if err := json.Unmarshal([]byte(l), &c); err != nil {
			t.Fatal(err)
		}
		var blocks []Block
		if c.Name != nil {
			blocks = []Block{{Environment: *c.Name}}
		}
		doc := Document{Path: "leaf.md", Text: c.Line + "\n>\n> The claim.\n"}
		if err := readBlocks(doc, blocks); err != nil {
			t.Errorf("%q: %v", c.Line, err)
		}
	}
}
