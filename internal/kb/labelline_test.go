package kb

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestLabelLine: testdata/label-lines.jsonl is each line and the name kb_tools'
// LABEL_LINE_RE reads from it, null for none. The claim graph's test reads
// the same file.
func TestLabelLine(t *testing.T) {
	data, err := os.ReadFile("testdata/label-lines.jsonl")
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
		name, ok := LabelLine(c.Line)
		if ok != (c.Name != nil) || ok && name != *c.Name {
			t.Errorf("LabelLine(%q) = %q, %t; want %v", c.Line, name, ok, l)
		}
	}
}
