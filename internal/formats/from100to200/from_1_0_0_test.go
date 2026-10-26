package from100to200_test

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"kbase/internal/formats"
	"kbase/internal/formats/format100"
	"kbase/internal/formats/format200"
)

const (
	register100Dir = "../testdata/register-1.0.0"
	register200Dir = "../testdata/register-2.0.0"
	badToken100Dir = "../testdata/bad-token-1.0.0"
	badToken200Dir = "../testdata/bad-token-2.0.0"

	entryPointPath = "kb-root/entry-point.md"

	// gotDir is where a differing conversion is written, under its golden's
	// name, so a golden is written by copying it in.
	gotDir = "../../../test-data/transient/unit_tests/formats/got"
)

// writeConverted writes files under gotDir/name, replacing a tree that stood
// there, and is where a golden is read from.
func writeConverted(t *testing.T, name string, files formats.Files) string {
	t.Helper()
	dir := filepath.Join(gotDir, name)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	for p, b := range files {
		target := filepath.Join(dir, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, b, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// readTree is every file under dir by slash path relative to dir, prefixed.
func readTree(t *testing.T, dir, prefix string, into formats.Files) {
	t.Helper()
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		into[path.Join(prefix, filepath.ToSlash(rel))] = b
		return err
	})
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
}

// TestConvert100To200Goldens: the register fixture converts to its 2.0.0
// golden file for file and byte for byte, its legacy declaration document the
// one path it makes obsolete, and its entry point left stamped 1.0.0: a
// conversion writes no stamp.
func TestConvert100To200Goldens(t *testing.T) {
	got, obsolete, _, err := formats.Chain(format100.Version100, format200.Version200, fixtureTree(t, register100Dir))
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	if want := []string{"kb-root/" + format100.Register100Name, format100.InvariantsPath}; !slices.Equal(obsolete, want) {
		t.Errorf("obsolete %q, want %q", obsolete, want)
	}
	compareFiles(t, "register-2.0.0", got, fixtureTree(t, register200Dir))
	fm, err := format200.ParseFrontmatter(string(got[entryPointPath]))
	if stamp := fm.Str("kb-format"); err != nil || stamp != format100.Version100 {
		t.Errorf("the converted entry point is stamped %q (%v), want %s", stamp, err, format100.Version100)
	}
}

// TestConvert100To200IsAFixedPoint: the 2.0.0 golden is what the converter
// writes, so converting it again changes no byte and makes nothing obsolete —
// a conversion that stopped between two writes leaves a KB its next load
// reads.
func TestConvert100To200IsAFixedPoint(t *testing.T) {
	want := fixtureTree(t, register200Dir)
	got, obsolete, _, err := formats.Chain(format100.Version100, format200.Version200, want)
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	if len(obsolete) != 0 {
		t.Errorf("converting 2.0.0 bytes made %q obsolete, want none", obsolete)
	}
	compareFiles(t, "register-2.0.0.converted-again", got, want)
}

// TestConvert100To200RefusesContentBesideTheDeclarations: an invariants.md
// holding prose outside every declaration is refused by path, line and
// content, and nothing converts.
func TestConvert100To200RefusesContentBesideTheDeclarations(t *testing.T) {
	files := fixtureTree(t, register100Dir)
	files[format100.InvariantsPath] = []byte("# Invariants\n\n## Notes\n\nRead this first.\n\n### INVARIANT-S3: A labelling convention\n\nBody.\n")
	got, _, _, err := formats.Chain(format100.Version100, format200.Version200, files)
	var surplus *formats.ConversionError
	if !errors.As(err, &surplus) || surplus.Path != format100.InvariantsPath || surplus.Line != 5 || surplus.Text != "Read this first." {
		t.Fatalf("Chain = %v, want a ConversionError at %s line 5", err, format100.InvariantsPath)
	}
	if got != nil {
		t.Errorf("the refused conversion produced %d files, want none", len(got))
	}
}

// TestConvert100To200RefusesProseAfterAnAxiomBullet: prose trailing an axiom
// bullet stands outside the bullet's own declaration, so it is refused by
// path, line and content rather than absorbed into a declaration, and nothing
// converts.
func TestConvert100To200RefusesProseAfterAnAxiomBullet(t *testing.T) {
	files := fixtureTree(t, register100Dir)
	files[format100.InvariantsPath] = []byte("# Invariants\n\n### INVARIANT-S3: A labelling convention\n\nLabelled as authored.\n\n- Axiom 1: **Substrate Topology** — the vacuum is a chiral lattice.\n\nProse that follows the axiom bullet.\n")
	got, _, _, err := formats.Chain(format100.Version100, format200.Version200, files)
	var surplus *formats.ConversionError
	if !errors.As(err, &surplus) || surplus.Path != format100.InvariantsPath || surplus.Line != 9 || surplus.Text != "Prose that follows the axiom bullet." {
		t.Fatalf("Chain = %v, want a ConversionError at %s line 9", err, format100.InvariantsPath)
	}
	if got != nil {
		t.Errorf("the refused conversion produced %d files, want none", len(got))
	}
}

// TestConvert100To200ConvertsHeadingsAroundDeclarations: section headings
// before, between and after the declarations are structure and convert, and
// the axioms.md holds the declarations alone.
func TestConvert100To200ConvertsHeadingsAroundDeclarations(t *testing.T) {
	files := fixtureTree(t, register100Dir)
	files[format100.InvariantsPath] = []byte("# Invariants\n\n## Structural Conventions\n\n### INVARIANT-S3: A labelling convention\n\nLabels are written as authored.\n\n## Topology\n\n- Axiom 1: **Substrate Topology** — the vacuum is a chiral lattice.\n\n### Reference\n")
	got, obsolete, _, err := formats.Chain(format100.Version100, format200.Version200, files)
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	want := "### Axiom S3: A labelling convention\n\nLabels are written as authored.\n\n### Axiom 1: Substrate Topology\n\nthe vacuum is a chiral lattice.\n"
	if string(got[format200.AxiomsPath]) != want {
		t.Errorf("%s = %q, want %q", format200.AxiomsPath, got[format200.AxiomsPath], want)
	}
	if !slices.Contains(obsolete, format100.InvariantsPath) {
		t.Errorf("obsolete %q, want %s among them", obsolete, format100.InvariantsPath)
	}
}

// TestConvert100To200LeavesADeclarationlessDocumentAlone: an invariants.md
// holding no declaration converts nothing, invents no axioms.md and is not
// made obsolete.
func TestConvert100To200LeavesADeclarationlessDocumentAlone(t *testing.T) {
	doc := "# Notes\n\nNothing here declares an axiom.\n"
	files := fixtureTree(t, register100Dir)
	files[format100.InvariantsPath] = []byte(doc)
	got, obsolete, _, err := formats.Chain(format100.Version100, format200.Version200, files)
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	if _, ok := got[format200.AxiomsPath]; ok {
		t.Errorf("the conversion invented %s, want none", format200.AxiomsPath)
	}
	if slices.Contains(obsolete, format100.InvariantsPath) {
		t.Errorf("the conversion made %s obsolete, want it left where it stands", format100.InvariantsPath)
	}
	if string(got[format100.InvariantsPath]) != doc {
		t.Errorf("%s = %q, want it as written", format100.InvariantsPath, got[format100.InvariantsPath])
	}
}

// TestConvert100To200RewritesLinksToTheRenamedRegister: a Markdown link target
// naming the register follows the file's rename, its anchor, brackets and
// title kept; a different file and prose outside a link stand.
func TestConvert100To200RewritesLinksToTheRenamedRegister(t *testing.T) {
	tests := []struct{ name, in, want string }{
		{"inline same directory", "[r](claim-quality.md)", "[r](claim-rigor.md)"},
		{"inline parent directory", "[r](../part2/claim-quality.md)", "[r](../part2/claim-rigor.md)"},
		{"anchor", "[r](../part2/claim-quality.md#clm-a)", "[r](../part2/claim-rigor.md#clm-a)"},
		{"reference style", "see [r]\n\n[r]: ../part2/claim-quality.md#clm-a", "see [r]\n\n[r]: ../part2/claim-rigor.md#clm-a"},
		{"title", `[r](claim-quality.md "the register")`, `[r](claim-rigor.md "the register")`},
		{"angle brackets", "[r](<../part2/claim-quality.md#clm-a>)", "[r](<../part2/claim-rigor.md#clm-a>)"},
		{"different file", "[r](old-claim-quality.md)", "[r](old-claim-quality.md)"},
		{"prose naming the file", "The file claim-quality.md was renamed.", "The file claim-quality.md was renamed."},
	}
	const doc = "kb-root/notes.md"
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files := fixtureTree(t, register100Dir)
			files[doc] = []byte(tt.in + "\n")
			got, _, _, err := formats.Chain(format100.Version100, format200.Version200, files)
			if err != nil {
				t.Fatalf("Chain: %v", err)
			}
			if string(got[doc]) != tt.want+"\n" {
				t.Errorf("%s = %q, want %q", doc, got[doc], tt.want+"\n")
			}
		})
	}
}

// compareFiles: got and want hold the same paths, and each path the same
// bytes. A difference is written under gotDir/name, so a golden is written by
// copying it in.
func compareFiles(t *testing.T, name string, got, want formats.Files) {
	t.Helper()
	differs := false
	if paths, wantPaths := slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)); !slices.Equal(paths, wantPaths) {
		t.Errorf("converted paths %q, goldens %q", paths, wantPaths)
		differs = true
	}
	for p, w := range want {
		if !bytes.Equal(got[p], w) {
			t.Errorf("%s differs from its golden:\n%s\nwant\n%s", p, got[p], w)
			differs = true
		}
	}
	if differs {
		t.Errorf("got written to %s", writeConverted(t, name, got))
	}
}

func fixtureTree(t *testing.T, dir string) formats.Files {
	t.Helper()
	files := formats.Files{}
	readTree(t, dir, "", files)
	return files
}

// TestConvert100To200Edges: the converted register loads through the 2.0.0 Discover
// with every 1.0.0 edge in its 2.0.0 relation and each carried token value —
// the recorded strength, the applicability turned bearing, a split bullet's
// pending strengths and the lifts a support and an experiment author. Every
// claim's premise list is now its analysis's, each claim lifting off the
// analysis minted for it at full bearing, and nothing rests on a claim.
func TestConvert100To200Edges(t *testing.T) {
	got, _, _, err := formats.Chain(format100.Version100, format200.Version200, fixtureTree(t, register100Dir))
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	st, err := discover(t, got)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	edges := stateEdges(st)
	want := []string{
		"analysis ana-aaaaaa rigor 0.8",
		"analysis ana-aaaaab rigor 0.85",
		"analysis ana-bbbbbb rigor pending",
		"analysis ana-bbbbbc rigor 0.9",
		"analysis ana-cccccc rigor 0.6",
		"axiom axm-1 Substrate Topology",
		"axiom axm-S3 A labelling convention",
		"claim clm-aaaaaa rigor 0.85 strength 0.62",
		"claim clm-bbbbbb rigor 0.9 strength 0.9",
		"claim clm-cccccc rigor 0.6 strength 0.6",
		"experiment exp-aaaaaa rigor 1 lifts clm-aaaaaa 0.8, clm-bbbbbb pending",
		"experiment exp-bbbbbb rigor 1",
		"founds axm-1 ana-cccccc 1 ctx the substrate assumption",
		"founds axm-S3 ana-aaaaab 1 ctx a labelling convention",
		"founds axm-S3 ana-bbbbbc 0.5",
		"founds wrk-smith2020 ana-aaaaab 0.8 ctx Theorem 3",
		"founds wrk-smith2020 ana-bbbbbc 0.5",
		"founds wrk-smith2020 ana-cccccc 1",
		"lifts ana-aaaaaa clm-bbbbbb 0.5",
		"lifts ana-aaaaaa clm-cccccc pending",
		"lifts ana-aaaaab clm-aaaaaa 1",
		"lifts ana-bbbbbb clm-cccccc 0.7",
		"lifts ana-bbbbbc clm-bbbbbb 1",
		"lifts ana-cccccc clm-cccccc 1",
		"lifts exp-aaaaaa clm-aaaaaa 0.8",
		"lifts exp-aaaaaa clm-bbbbbb pending",
		"supports clm-aaaaaa ana-aaaaaa 1",
		"supports clm-aaaaaa ana-bbbbbc 1 recorded 0.85",
		"supports clm-bbbbbb ana-aaaaab 1 recorded 0.9 ctx builds directly on the anchor claim",
		"supports clm-cccccc ana-bbbbbc 0.5",
		"supports clm-cccccc ana-bbbbbc 1",
		"work wrk-smith2020 rigor 0.9",
	}
	if !slices.Equal(edges, want) {
		t.Errorf("edges\n %s\nwant\n %s", strings.Join(edges, "\n "), strings.Join(want, "\n "))
	}
}

// stateEdges is every authored value and edge the fixture carries, in a
// stable textual form.
func stateEdges(st format200.State) []string {
	var out []string
	for _, w := range st.Works {
		out = append(out, fmt.Sprintf("work %s rigor %s", w.ID, number(w.Rigor)))
	}
	for _, a := range st.Axioms {
		out = append(out, fmt.Sprintf("axiom %s %s", a.ID, a.Title))
	}
	for _, s := range st.Analyses {
		out = append(out, fmt.Sprintf("analysis %s rigor %s", s.ID, number(s.Rigor)))
	}
	for _, c := range st.ClaimEntries {
		out = append(out, fmt.Sprintf("claim %s rigor %s strength %s", c.ID, number(c.Rigor), number(c.Strength)))
		for _, e := range c.Premises {
			out = append(out, edgeText(e))
		}
	}
	for _, s := range st.Analyses {
		for _, e := range s.Premises {
			out = append(out, edgeText(e))
		}
		for _, e := range s.Lifts {
			out = append(out, edgeText(e))
		}
		for _, e := range s.Refutes {
			out = append(out, edgeText(e))
		}
	}
	for _, x := range st.Experiments {
		var lifts []string
		for _, e := range x.Lifts {
			lifts = append(lifts, e.Target+" "+bearingText(e.Bearing))
		}
		line := fmt.Sprintf("experiment %s rigor %s", x.ID, number(x.Rigor))
		if len(lifts) > 0 {
			line += " lifts " + strings.Join(lifts, ", ")
		}
		out = append(out, line)
		for _, e := range x.Lifts {
			out = append(out, edgeText(e))
		}
	}
	slices.Sort(out)
	return out
}

func edgeText(e format200.Edge) string {
	text := fmt.Sprintf("%s %s %s %s", e.Relation, e.Source, e.Target, bearingText(e.Bearing))
	if e.SourceStrengthRecorded != nil {
		text += fmt.Sprintf(" recorded %s", number(e.SourceStrengthRecorded))
	}
	if e.Context != nil {
		text += " ctx " + *e.Context
	}
	return text
}

func bearingText(b format200.Bearing) string {
	if b.Pending {
		return "pending"
	}
	return number(&b.Value)
}

func number(v *float64) string {
	if v == nil {
		return "pending"
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%g", *v), "0"), ".")
}

// discover loads the converted files as a 2.0.0 KB's authoring layer.
func discover(t *testing.T, files formats.Files) (format200.State, error) {
	t.Helper()
	return format200.Discover(files, func(string, int, string) {})
}

// TestConvert100To200CarriesAnUnreadableBearing: the value the converter
// cannot judge is carried as text, and the 2.0.0 reader refuses it by field
// and line — the converter never repairs what it does not understand.
func TestConvert100To200CarriesAnUnreadableBearing(t *testing.T) {
	got, _, _, err := formats.Chain(format100.Version100, format200.Version200, fixtureTree(t, badToken100Dir))
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	compareFiles(t, "bad-token-2.0.0", got, fixtureTree(t, badToken200Dir))
	file := format200.KBDir + "/" + format200.RegisterFile
	if !strings.Contains(string(got[file]), "(bearing sometimes)") {
		t.Errorf("the converted bullet does not carry the token as written:\n%s", got[file])
	}
	_, err = discover(t, got)
	var ann *format200.AnnotationError
	if !errors.As(err, &ann) || len(ann.Faults) != 1 {
		t.Fatalf("Discover = %v, want one annotation fault", err)
	}
	f := ann.Faults[0]
	if f.Field != format200.ListRestsOn || f.Allowed == "" || f.Line == 0 || f.Path != badTokenPath() {
		t.Errorf("fault %+v, want rests-on by path, line and allowed set", f)
	}
}

// badTokenPath is the fixture register's path as the readers report it:
// kb-root-relative.
func badTokenPath() string { return format200.RegisterFile }
