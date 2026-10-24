package from_1_0_0_to_2_0_0_test

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

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/migrate"
	"kbase/internal/migrate/format_1_0_0"
	"kbase/internal/migrate/format_2_0_0"
)

const (
	register100Dir = "../testdata/register-1.0.0"
	register200Dir = "../testdata/register-2.0.0"
	badToken100Dir = "../testdata/bad-token-1.0.0"
	badToken200Dir = "../testdata/bad-token-2.0.0"

	entryPointPath = "kb-root/entry-point.md"
)

// readTree is every file under dir by slash path relative to dir, prefixed.
func readTree(t *testing.T, dir, prefix string, into migrate.Files) {
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
	got, obsolete, err := migrate.Chain(format_1_0_0.Version100, format_2_0_0.Version200, fixtureTree(t, register100Dir))
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	if want := []string{"kb-root/" + format_1_0_0.Register100Name, format_1_0_0.InvariantsPath}; !slices.Equal(obsolete, want) {
		t.Errorf("obsolete %q, want %q", obsolete, want)
	}
	compareFiles(t, got, fixtureTree(t, register200Dir))
	if stamp, err := kb.FormatStamp(string(got[entryPointPath])); err != nil || stamp != format_1_0_0.Version100 {
		t.Errorf("the converted entry point is stamped %q (%v), want %s", stamp, err, format_1_0_0.Version100)
	}
}

// TestConvert100To200IsAFixedPoint: the 2.0.0 golden is what the converter
// writes, so converting it again changes no byte and makes nothing obsolete —
// a conversion that stopped between two writes leaves a KB its next load
// reads.
func TestConvert100To200IsAFixedPoint(t *testing.T) {
	want := fixtureTree(t, register200Dir)
	got, obsolete, err := migrate.Chain(format_1_0_0.Version100, format_2_0_0.Version200, want)
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	if len(obsolete) != 0 {
		t.Errorf("converting 2.0.0 bytes made %q obsolete, want none", obsolete)
	}
	compareFiles(t, got, want)
}

// TestConvert100To200RefusesContentBesideTheDeclarations: an invariants.md
// holding prose outside every declaration is refused by path, line and
// content, and nothing converts.
func TestConvert100To200RefusesContentBesideTheDeclarations(t *testing.T) {
	files := fixtureTree(t, register100Dir)
	files[format_1_0_0.InvariantsPath] = []byte("# Invariants\n\n## Notes\n\nRead this first.\n\n### INVARIANT-S3: A labelling convention\n\nBody.\n")
	got, _, err := migrate.Chain(format_1_0_0.Version100, format_2_0_0.Version200, files)
	var surplus *migrate.ConversionError
	if !errors.As(err, &surplus) || surplus.Path != format_1_0_0.InvariantsPath || surplus.Line != 5 || surplus.Text != "Read this first." {
		t.Fatalf("Chain = %v, want a ConversionError at %s line 5", err, format_1_0_0.InvariantsPath)
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
	files[format_1_0_0.InvariantsPath] = []byte("# Invariants\n\n### INVARIANT-S3: A labelling convention\n\nLabelled as authored.\n\n- Axiom 1: **Substrate Topology** — the vacuum is a chiral lattice.\n\nProse that follows the axiom bullet.\n")
	got, _, err := migrate.Chain(format_1_0_0.Version100, format_2_0_0.Version200, files)
	var surplus *migrate.ConversionError
	if !errors.As(err, &surplus) || surplus.Path != format_1_0_0.InvariantsPath || surplus.Line != 9 || surplus.Text != "Prose that follows the axiom bullet." {
		t.Fatalf("Chain = %v, want a ConversionError at %s line 9", err, format_1_0_0.InvariantsPath)
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
	files[format_1_0_0.InvariantsPath] = []byte("# Invariants\n\n## Structural Conventions\n\n### INVARIANT-S3: A labelling convention\n\nLabels are written as authored.\n\n## Topology\n\n- Axiom 1: **Substrate Topology** — the vacuum is a chiral lattice.\n\n### Reference\n")
	got, obsolete, err := migrate.Chain(format_1_0_0.Version100, format_2_0_0.Version200, files)
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	want := "### Axiom S3: A labelling convention\n\nLabels are written as authored.\n\n### Axiom 1: Substrate Topology\n\nthe vacuum is a chiral lattice.\n"
	if string(got[format_2_0_0.AxiomsPath]) != want {
		t.Errorf("%s = %q, want %q", format_2_0_0.AxiomsPath, got[format_2_0_0.AxiomsPath], want)
	}
	if !slices.Contains(obsolete, format_1_0_0.InvariantsPath) {
		t.Errorf("obsolete %q, want %s among them", obsolete, format_1_0_0.InvariantsPath)
	}
}

// TestConvert100To200LeavesADeclarationlessDocumentAlone: an invariants.md
// holding no declaration converts nothing, invents no axioms.md and is not
// made obsolete.
func TestConvert100To200LeavesADeclarationlessDocumentAlone(t *testing.T) {
	doc := "# Notes\n\nNothing here declares an axiom.\n"
	files := fixtureTree(t, register100Dir)
	files[format_1_0_0.InvariantsPath] = []byte(doc)
	got, obsolete, err := migrate.Chain(format_1_0_0.Version100, format_2_0_0.Version200, files)
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	if _, ok := got[format_2_0_0.AxiomsPath]; ok {
		t.Errorf("the conversion invented %s, want none", format_2_0_0.AxiomsPath)
	}
	if slices.Contains(obsolete, format_1_0_0.InvariantsPath) {
		t.Errorf("the conversion made %s obsolete, want it left where it stands", format_1_0_0.InvariantsPath)
	}
	if string(got[format_1_0_0.InvariantsPath]) != doc {
		t.Errorf("%s = %q, want it as written", format_1_0_0.InvariantsPath, got[format_1_0_0.InvariantsPath])
	}
}

// compareFiles: got and want hold the same paths, and each path the same
// bytes.
func compareFiles(t *testing.T, got, want migrate.Files) {
	t.Helper()
	if paths, wantPaths := slices.Sorted(maps.Keys(got)), slices.Sorted(maps.Keys(want)); !slices.Equal(paths, wantPaths) {
		t.Fatalf("converted paths %q, goldens %q", paths, wantPaths)
	}
	for p, w := range want {
		if !bytes.Equal(got[p], w) {
			t.Errorf("%s differs from its golden:\n%s\nwant\n%s", p, got[p], w)
		}
	}
}

func fixtureTree(t *testing.T, dir string) migrate.Files {
	t.Helper()
	files := migrate.Files{}
	readTree(t, dir, "", files)
	return files
}

// TestConvert100To200Edges: the converted register loads through kb.Discover
// with every 1.0.0 edge in its 2.0.0 relation and each carried token value —
// the recorded strength, the applicability turned bearing, a split bullet's
// pending strengths and the lifts a support and an experiment author.
func TestConvert100To200Edges(t *testing.T) {
	got, _, err := migrate.Chain(format_1_0_0.Version100, format_2_0_0.Version200, fixtureTree(t, register100Dir))
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
		"analysis ana-bbbbbb rigor pending",
		"axiom axm-1 Substrate Topology",
		"axiom axm-S3 A labelling convention",
		"claim clm-aaaaaa rigor 0.85 strength 0.62",
		"claim clm-bbbbbb rigor 0.9 strength 0.9",
		"claim clm-cccccc rigor 0.6 strength 0.6",
		"experiment exp-aaaaaa rigor 1 lifts clm-aaaaaa 0.8, clm-bbbbbb pending",
		"experiment exp-bbbbbb rigor 1",
		"founds axm-1 clm-cccccc 1 ctx the substrate assumption",
		"founds axm-S3 clm-aaaaaa 1 ctx a labelling convention",
		"founds axm-S3 clm-bbbbbb 0.5",
		"founds wrk-smith2020 clm-aaaaaa 0.8 ctx Theorem 3",
		"founds wrk-smith2020 clm-bbbbbb 0.5",
		"founds wrk-smith2020 clm-cccccc 1",
		"lifts ana-aaaaaa clm-bbbbbb 0.5",
		"lifts ana-aaaaaa clm-cccccc pending",
		"lifts ana-bbbbbb clm-cccccc 0.7",
		"lifts exp-aaaaaa clm-aaaaaa 0.8",
		"lifts exp-aaaaaa clm-bbbbbb pending",
		"supports clm-aaaaaa ana-aaaaaa 1",
		"supports clm-aaaaaa clm-bbbbbb 1 recorded 0.85",
		"supports clm-bbbbbb clm-aaaaaa 1 recorded 0.9 ctx builds directly on the anchor claim",
		"supports clm-cccccc clm-bbbbbb 0.5",
		"supports clm-cccccc clm-bbbbbb 1",
		"work wrk-smith2020 rigor 0.9",
	}
	if !slices.Equal(edges, want) {
		t.Errorf("edges\n %s\nwant\n %s", strings.Join(edges, "\n "), strings.Join(want, "\n "))
	}
}

// stateEdges is every authored value and edge the fixture carries, in a
// stable textual form.
func stateEdges(st kb.State) []string {
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

func edgeText(e kb.Edge) string {
	text := fmt.Sprintf("%s %s %s %s", e.Relation, e.Source, e.Target, bearingText(e.Bearing))
	if e.SourceStrengthRecorded != nil {
		text += fmt.Sprintf(" recorded %s", number(e.SourceStrengthRecorded))
	}
	if e.Context != nil {
		text += " ctx " + *e.Context
	}
	return text
}

func bearingText(b kb.Bearing) string {
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

// discover loads the converted files as a KB's authoring layer.
// A test helper does not cross packages: format_2_0_0's tests keep their own copy.
func discover(t *testing.T, files migrate.Files) (kb.State, error) {
	t.Helper()
	repo := t.TempDir()
	kbRoot := filepath.Join(repo, kb.KBDir)
	if err := os.MkdirAll(kbRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	return kb.Discover(kb.MigratedSource(kbRoot, format_1_0_0.Version100, files, nil), log.Discard())
}

// TestConvert100To200CarriesAnUnreadableBearing: the value the converter
// cannot judge is carried as text, and the 2.0.0 reader refuses it by field
// and line — the converter never repairs what it does not understand.
func TestConvert100To200CarriesAnUnreadableBearing(t *testing.T) {
	got, _, err := migrate.Chain(format_1_0_0.Version100, format_2_0_0.Version200, fixtureTree(t, badToken100Dir))
	if err != nil {
		t.Fatalf("Chain: %v", err)
	}
	want := fixtureTree(t, badToken200Dir)
	for p, w := range want {
		if !bytes.Equal(got[p], w) {
			t.Errorf("%s converts to\n%s\nwant\n%s", p, got[p], w)
		}
	}
	file := kb.KBDir + "/" + kb.RegisterFile
	if !strings.Contains(string(got[file]), "(bearing sometimes)") {
		t.Errorf("the converted bullet does not carry the token as written:\n%s", got[file])
	}
	_, err = discover(t, got)
	var ann *kb.AnnotationError
	if !errors.As(err, &ann) || len(ann.Faults) != 1 {
		t.Fatalf("Discover = %v, want one annotation fault", err)
	}
	f := ann.Faults[0]
	if f.Field != kb.ListRestsOn || f.Allowed == "" || f.Line == 0 || f.Path != badTokenPath() {
		t.Errorf("fault %+v, want rests-on by path, line and allowed set", f)
	}
}

// badTokenPath is the fixture register's path as the readers report it:
// kb-root-relative.
func badTokenPath() string { return kb.RegisterFile }
