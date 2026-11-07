package config

import (
	"strings"
	"testing"
)

// TestModelHintsMarkdownStatesEveryRowAndNote: the guidance an MCP client reads
// at kbase://model-hints is the same rows the config template carries — each
// row's role, capability and example ids — and the notes that hold for
// every row, which the template states after them.
func TestModelHintsMarkdownStatesEveryRowAndNote(t *testing.T) {
	got := ModelHintsMarkdown()
	for _, h := range modelHints {
		for _, want := range append([]string{"`" + string(h.role) + "`", h.capability}, h.examples...) {
			if !strings.Contains(got, want) {
				t.Errorf("the guidance states no %q:\n%s", want, got)
			}
		}
	}
	for _, note := range []string{modelHintsShared, modelHintsVendor} {
		if !strings.Contains(got, note) {
			t.Errorf("the guidance states no %q:\n%s", note, got)
		}
	}
}
