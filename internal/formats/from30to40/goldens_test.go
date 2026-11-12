package from30to40

import (
	"bytes"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format30"
)

const (
	testdata = "../testdata"
	// gotDir is where a differing conversion is written, under its golden's
	// name, so a golden is written by copying it in.
	gotDir = "../../../test-data/transient/unit_tests/formats/got"
)

// readTree is every file under dir by slash path relative to it; none where
// dir does not stand, as a golden not yet produced.
func readTree(t *testing.T, dir string) common.Files {
	t.Helper()
	files := common.Files{}
	if _, err := os.Stat(dir); errors.Is(err, fs.ErrNotExist) {
		return files
	}
	err := filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		b, err := os.ReadFile(p)
		files[filepath.ToSlash(rel)] = b
		return err
	})
	if err != nil {
		t.Fatalf("reading %s: %v", dir, err)
	}
	return files
}

// writeGot writes files under gotDir/name, replacing a tree that stood there;
// name "" writes them directly under gotDir.
func writeGot(t *testing.T, name string, files common.Files) string {
	t.Helper()
	dir := filepath.Join(gotDir, name)
	if name != "" {
		if err := os.RemoveAll(dir); err != nil {
			t.Fatal(err)
		}
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

// conversionRecord is what a conversion reports beside its files.
type conversionRecord struct {
	Obsolete []string         `yaml:"obsolete"`
	Warnings []common.Warning `yaml:"warnings"`
}

// TestGoldenPairs: each 3.0.0 tree, from20to30's frozen output, converts to
// its 4.0.0 golden tree file for file and byte for byte, with the obsolete
// paths and the warnings the golden's conversion record beside it names.
func TestGoldenPairs(t *testing.T) {
	for _, pair := range []struct{ in, out string }{
		{"register-3.0.0", "register-4.0.0"},
		{"devpro-sheet-3.0.0", "devpro-sheet-4.0.0"},
		{"markers-3.0.0", "markers-4.0.0"},
		{"citations-3.0.0", "citations-4.0.0"},
	} {
		t.Run(pair.in, func(t *testing.T) {
			got, obsolete, warnings, err := Convert(readTree(t, path.Join(testdata, pair.in)))
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if want := readTree(t, path.Join(testdata, pair.out)); !maps.EqualFunc(got, want, bytes.Equal) {
				t.Errorf("the conversion of %s differs from %s; got written to %s", pair.in, pair.out, writeGot(t, pair.out, got))
			}
			record, err := yaml.Marshal(conversionRecord{Obsolete: obsolete, Warnings: warnings})
			if err != nil {
				t.Fatal(err)
			}
			name := pair.out + ".conversion.yaml"
			if want, err := os.ReadFile(path.Join(testdata, name)); err != nil || !bytes.Equal(record, want) {
				t.Errorf("%s: %v; got written to %s", name, err, writeGot(t, "", common.Files{name: record}))
			}
		})
	}
}

// TestAConvertedKBConvertsToItself: a save that wrote the converted files and
// stopped before the stamp leaves a KB whose next conversion changes no byte,
// makes nothing obsolete, and warns again only of each citation it still
// cannot resolve, exactly as its first conversion did.
func TestAConvertedKBConvertsToItself(t *testing.T) {
	for _, golden := range []string{"register-4.0.0", "devpro-sheet-4.0.0", "markers-4.0.0", "citations-4.0.0"} {
		t.Run(golden, func(t *testing.T) {
			want := readTree(t, path.Join(testdata, golden))
			got, obsolete, warnings, err := Convert(want)
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if len(obsolete) != 0 || !maps.EqualFunc(got, want, bytes.Equal) {
				t.Errorf("re-converting %s: obsolete %q, files equal %t", golden, obsolete, maps.EqualFunc(got, want, bytes.Equal))
			}
			var first conversionRecord
			data, err := os.ReadFile(path.Join(testdata, golden+".conversion.yaml"))
			if err != nil {
				t.Fatal(err)
			}
			if err := yaml.Unmarshal(data, &first); err != nil {
				t.Fatal(err)
			}
			// site is what a warning reports beyond its sentence, which quotes
			// the link as it stands.
			site := func(ws []common.Warning) []common.Warning {
				var out []common.Warning
				for _, w := range ws {
					if w.List == common.ListUnresolvedCitations {
						out = append(out, common.Warning{Path: w.Path, Line: w.Line, List: w.List, ID: w.ID})
					}
				}
				return out
			}
			if again, unresolved := site(warnings), site(first.Warnings); len(again) != len(warnings) || !slices.Equal(again, unresolved) {
				t.Errorf("re-converting %s warns %+v, want the first conversion's unresolved citations %+v", golden, warnings, unresolved)
			}
		})
	}
}

// TestAStoppedSaveIsTakenAsItStands: where the edge file stands converted
// beside a 3.0.0 node file — a save stopped between the two — the edge file is
// taken byte for byte, the node file converted, and the markers' move reads
// the converted edge file's lifts.
func TestAStoppedSaveIsTakenAsItStands(t *testing.T) {
	for _, pair := range []struct{ in, out string }{{"register-3.0.0", "register-4.0.0"}, {"markers-3.0.0", "markers-4.0.0"}} {
		t.Run(pair.in, func(t *testing.T) {
			files := readTree(t, path.Join(testdata, pair.in))
			want := readTree(t, path.Join(testdata, pair.out))
			files[edgesPath] = want[edgesPath]
			got, _, _, err := Convert(files)
			if err != nil {
				t.Fatalf("Convert: %v", err)
			}
			if !maps.EqualFunc(got, want, bytes.Equal) {
				t.Errorf("the half-saved tree converts to a tree other than %s; got written to %s", pair.out, writeGot(t, pair.out+"-stopped", got))
			}
		})
	}
}

// TestDroppedFieldsAreReported: each claim's and analysis's no-edge text and
// each claim's strengthen-by items are reported under their list with the
// node's id and the text, in node order, and the converted records carry
// neither field; an experiment's pending status is planned.
func TestDroppedFieldsAreReported(t *testing.T) {
	nodes := `- id: "ana-aaaaaa"
  title: "A"
  rigor: 0.5
  manually-scored: true
  rationale: ""
  no-edge: "clm-bbbbbb is cited as contrast"
- id: "clm-aaaaaa"
  title: "C"
  rigor: "*pending*"
  manually-scored: false
  rationale: "R"
  no-edge: ""
  strengthen-by:
    - "Tighten the constant."
    - "Cite exp-aaaaaa's rerun."
- id: "exp-aaaaaa"
  title: "E"
  status: "pending"
  rigor: 1.0
  manually-scored: false
`
	got, _, warnings, err := Convert(common.Files{nodesPath: []byte(nodes), edgesPath: []byte("[]\n")})
	if err != nil {
		t.Fatalf("Convert: %v", err)
	}
	type reported struct{ list, id, text string }
	var lists []reported
	for _, w := range warnings {
		if w.Path != nodesPath || w.Detail == "" {
			t.Errorf("warning %+v names no record file or detail", w)
		}
		lists = append(lists, reported{w.List, w.ID, w.Text})
	}
	if want := []reported{
		{common.ListNoEdge, "ana-aaaaaa", "clm-bbbbbb is cited as contrast"},
		{common.ListStrengthenBy, "clm-aaaaaa", "Tighten the constant.; Cite exp-aaaaaa's rerun."},
	}; !reflect.DeepEqual(lists, want) {
		t.Errorf("reported %+v, want %+v", lists, want)
	}
	zeros := `"0000000000000000000000000000000000000000000000000000000000000000"`
	want := `- id: "ana-aaaaaa"
  title: "A"
  rigor: 0.5
  manually-scored: true
  rationale: ""
  judged-hash: ` + zeros + `
- id: "clm-aaaaaa"
  title: "C"
  rigor: "*pending*"
  manually-scored: false
  rationale: "R"
  judged-hash: ` + zeros + `
- id: "exp-aaaaaa"
  title: "E"
  status: "planned"
  rigor: 1.0
  manually-scored: false
  judged-hash: ` + zeros + `
`
	if string(got[nodesPath]) != want {
		t.Errorf("the node file converts to\n%s\nwant\n%s", got[nodesPath], want)
	}
}

// TestARefusedRecordFileIsRefused: a 3.0.0 record file the 3.0.0 load refuses
// is refused with that load's own faults, one per fault.
func TestARefusedRecordFileIsRefused(t *testing.T) {
	nodes := "- id: \"clm-aaaaaa\"\n  title: \"\"\n  rigor: 2\n"
	_, _, _, err := Convert(common.Files{nodesPath: []byte(nodes), edgesPath: []byte("[]\n")})
	var load *format30.GraphLoadError
	if !errors.As(err, &load) {
		t.Fatalf("err = %v, want the 3.0.0 load's refusal", err)
	}
	var keys []string
	for _, f := range load.Faults {
		if f.Check != format30.CheckMetadata || f.Path != format30.NodesFile {
			t.Errorf("fault %+v, want a metadata fault on %s", f, format30.NodesFile)
		}
		keys = append(keys, f.Key)
	}
	if want := []string{"manually-scored", "rationale", "no-edge", "strengthen-by", "title"}; !reflect.DeepEqual(keys, want) {
		t.Errorf("faults name %q, want %q", keys, want)
	}
}
