package pandoc_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/latex/pandoc"
	"kbase/internal/log"
)

// requirePandoc skips the calling test where pandoc is missing or out of range,
// the posture the sheet tests take without Graphviz.
func requirePandoc(t *testing.T) {
	t.Helper()
	if _, err := pandoc.Preflight(context.Background()); err != nil {
		t.Skipf("pandoc is unusable here: %v", err)
	}
}

// TestRunDefaultsResolvesBesideTheFile: a defaults file naming its input and
// output by bare relative paths converts the input beside it into the output
// beside it, whatever the caller's working directory.
func TestRunDefaultsResolvesBesideTheFile(t *testing.T) {
	requirePandoc(t)
	dir := t.TempDir()
	files := map[string]string{
		"in.md":         "Some *emphasis*.\n",
		"defaults.yaml": "from: gfm\nto: latex\ninput-files: [in.md]\noutput-file: out.tex\n",
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := pandoc.RunDefaults(context.Background(), log.Discard(), filepath.Join(dir, "defaults.yaml")); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(filepath.Join(dir, "out.tex"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), `\emph{emphasis}`) {
		t.Errorf("the output does not carry the converted input:\n%s", out)
	}
}
