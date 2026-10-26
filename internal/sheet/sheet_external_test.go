package sheet_test

import (
	"bytes"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/buildrecords"
	"kbase/internal/index"
	"kbase/internal/kb"
	"kbase/internal/kbload"
	"kbase/internal/sheet"
)

// adjagentFixture is the claim-sheet fixture: a KB, an unmarked build record
// beside it that the sheets do not read, and the DOT the sheets compose to.
const adjagentFixture = "testdata/adjagent-sheet"

func openAdjagentFixture(t *testing.T) *kb.Source {
	t.Helper()
	src, err := kbload.Open(filepath.Join(adjagentFixture, "kb-root"))
	if err != nil {
		t.Fatal(err)
	}
	return src
}

// derivedRows is the rows index.Derive yields for src.
func derivedRows(t *testing.T, src *kb.Source) index.Rows {
	t.Helper()
	rows, err := index.Derive(src)
	if err != nil {
		t.Fatal(err)
	}
	return rows
}

// renderDerived is Render over src's derived rows.
func renderDerived(t *testing.T, src *kb.Source) ([]sheet.Sheet, error) {
	t.Helper()
	rows := derivedRows(t, src)
	return sheet.Render(src, rows.Nodes, rows.Edges, rows.Cites)
}

// composeDerived is every sheet src's derived rows compose to.
func composeDerived(t *testing.T, src *kb.Source) []sheet.Composed {
	t.Helper()
	rows := derivedRows(t, src)
	sheets, err := sheet.Compose(src, rows.Nodes, rows.Edges, rows.Cites)
	if err != nil {
		t.Fatal(err)
	}
	return sheets
}

// fixtureFiles is every file under dir, by slash path relative to it.
func fixtureFiles(t *testing.T, dir string) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		out[filepath.ToSlash(rel)] = data
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// TestAdjagentSheetGoldens: kbase draws the fixture as the full sheet,
// the digest and the sheets of the two volumes holding nodes — none for
// gamma, which holds none — and its sheets are the goldens' DOT, byte for byte.
func TestAdjagentSheetGoldens(t *testing.T) {
	drawn := map[string]string{}
	var paths []string
	for _, s := range composeDerived(t, openAdjagentFixture(t)) {
		drawn[s.Path] = s.DOT(false)
		paths = append(paths, s.Path)
	}
	if want := []string{kb.ClaimGraphFile, kb.ClaimGraphDigestFile, "alpha/" + kb.ClaimGraphFile, "beta/" + kb.ClaimGraphFile}; !slices.Equal(paths, want) {
		t.Errorf("sheets %q, want %q", paths, want)
	}
	for golden, path := range map[string]string{"claim-graph.dot": kb.ClaimGraphFile, "claim-graph-digest.dot": kb.ClaimGraphDigestFile,
		"alpha-claim-graph.dot": "alpha/" + kb.ClaimGraphFile} {
		data, err := os.ReadFile(filepath.Join(adjagentFixture, golden))
		if err != nil {
			t.Fatal(err)
		}
		want := string(data)
		got, ok := drawn[path]
		if !ok {
			t.Errorf("no sheet %s", path)
			continue
		}
		if got != want {
			gl, wl := strings.Split(got, "\n"), strings.Split(want, "\n")
			for i := range max(len(gl), len(wl)) {
				if i >= len(gl) || i >= len(wl) || gl[i] != wl[i] {
					t.Errorf("%s differs from %s at line %d:\ngot:  %s\nwant: %s", path, golden, i+1, at(gl, i), at(wl, i))
					break
				}
			}
		}
	}
}

func at(lines []string, i int) string {
	if i < len(lines) {
		return lines[i]
	}
	return "(none)"
}

// TestLadderThroughFakeDot drives renderSVG over every sheet the emitter
// composes for the adjagent fixture, with a dot stand-in that refuses any DOT
// lacking splines=line: the first rung fails and the emitter's retry DOT
// renders; where the retry fails too, the first error comes back.
func TestLadderThroughFakeDot(t *testing.T) {
	sheets := composeDerived(t, openAdjagentFixture(t))
	dir := t.TempDir()
	fake := filepath.Join(dir, "dot")
	script := "#!/bin/sh\ninput=$(cat)\nif printf '%s' \"$input\" | grep -q 'splines=line'; then echo '<svg/>'; exit 0; fi\necho 'routesplines failed' >&2\nexit 1\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	for _, s := range sheets {
		if svg, err := sheet.RenderSVG(s.DOT); err != nil || string(svg) != "<svg/>\n" {
			t.Errorf("%s: renderSVG with fallback = %q, %v; want <svg/>", s.Path, svg, err)
		}
		unlined := func(bool) string { return s.DOT(false) }
		if _, err := sheet.RenderSVG(unlined); err == nil || !strings.Contains(err.Error(), "routesplines failed") {
			t.Errorf("%s: renderSVG where both rungs fail = %v, want the first error", s.Path, err)
		}
	}
}

// copiedAdjagentFixture is a writable copy of the adjagent fixture's
// repository.
func copiedAdjagentFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for rel, data := range fixtureFiles(t, adjagentFixture) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// composedDOT is every sheet the KB at root composes to, by path.
func composedDOT(t *testing.T, root string) map[string]string {
	t.Helper()
	src, err := kbload.Open(filepath.Join(root, kb.KBDir))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]string{}
	for _, s := range composeDerived(t, src) {
		out[s.Path] = s.DOT(false)
	}
	return out
}

// TestBuildRecordsDrawNothing: the build records beside kb-root move no
// sheet — neither the fixture's unmarked record, whose "points" answer the
// goldens draw as the one supports stroke, nor a cull record linking a drawn
// supports pair.
func TestBuildRecordsDrawNothing(t *testing.T) {
	root := copiedAdjagentFixture(t)
	without := composedDOT(t, root)
	record := "links:\n  - a: clm-aaa001\n    b: clm-bbb001\n    harvest: adjacent\n"
	if err := os.WriteFile(filepath.Join(root, buildrecords.CullFile), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	if with := composedDOT(t, root); !maps.Equal(with, without) {
		t.Error("a cull record moved the sheets")
	}
}

func TestRenderWithoutDot(t *testing.T) {
	src := openAdjagentFixture(t)
	t.Setenv("PATH", "")
	if _, err := renderDerived(t, src); !errors.Is(err, sheet.ErrNoDot) {
		t.Errorf("Render with no PATH = %v, want ErrNoDot", err)
	}
}

func TestRenderDraws(t *testing.T) {
	sheets, err := renderDerived(t, openAdjagentFixture(t))
	if errors.Is(err, sheet.ErrNoDot) {
		t.Skip("no Graphviz dot on PATH")
	}
	if err != nil {
		t.Fatal(err)
	}
	if len(sheets) == 0 {
		t.Fatal("Render drew no sheet")
	}
	for _, s := range sheets {
		if !bytes.Contains(s.SVG, []byte(`<svg width="100%"`)) {
			t.Errorf("%s: the root is not fitted to the viewer", s.Path)
		}
	}
}
