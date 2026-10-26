package write

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/kb"
)

// requireReferenceFixture is the reference's fixture of that name, committed
// under testdata/ so these tests run without the reference's own tree.
func requireReferenceFixture(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join("testdata", name)
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("fixture %s is not committed: %v", name, err)
	}
	return dir
}

func f64(v float64) *float64 { return &v }

// goldenShapes are the render specimens; the name is the golden's file stem.
var goldenShapes = map[string]func() string{
	"markers": func() string {
		return strings.Join([]string{kb.StatementMarker("clm-aa1111"), kb.StatementMarker("ana-ee5555"), kb.StatementMarker("exp-ff6666")}, "\n")
	},
	"citation": func() string {
		return renderCitation("the weakest link in the dependency cone", "part3/claim-rigor.md", "regime-conservation-laws")
	},
}

// TestRenderGoldens byte-compares every rendered shape against the committed
// golden for it; each golden carries the one trailing newline a text file ends
// with.
func TestRenderGoldens(t *testing.T) {
	dir := requireReferenceFixture(t, "writeapi-render-golden")
	onDisk, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	var stems []string
	for _, p := range onDisk {
		if stem := strings.TrimSuffix(filepath.Base(p), ".md"); stem != "README" {
			stems = append(stems, stem)
		}
	}
	var shapes []string
	for name := range goldenShapes {
		shapes = append(shapes, name)
	}
	slices.Sort(stems)
	slices.Sort(shapes)
	if !slices.Equal(stems, shapes) {
		t.Fatalf("golden set %v and specimen table %v disagree", stems, shapes)
	}
	for _, name := range shapes {
		t.Run(name, func(t *testing.T) {
			golden, err := os.ReadFile(filepath.Join(dir, name+".md"))
			if err != nil {
				t.Fatal(err)
			}
			if want, got := string(golden), goldenShapes[name]()+"\n"; got != want {
				t.Errorf("render differs from the golden\n--- golden\n%s--- kbase\n%s", want, got)
			}
		})
	}
}
