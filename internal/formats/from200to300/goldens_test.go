package from200to300

import (
	"bytes"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format200"
)

const (
	testdata = "../testdata"
	// gotDir is where a differing conversion is written, under its golden's
	// name, so a golden is written by copying it in.
	gotDir = "../../../test-data/transient/unit_tests/formats/got"
)

// readTree is every file under dir by slash path relative to it.
func readTree(t *testing.T, dir string) common.Files {
	t.Helper()
	files := common.Files{}
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

// TestGoldenPairs: each 2.0.x fixture converts to its 3.0.0 golden tree file
// for file and byte for byte, with the obsolete paths and the warnings the
// golden's conversion record beside it names.
func TestGoldenPairs(t *testing.T) {
	for _, pair := range []struct{ in, out string }{
		{"register-2.0.0", "register-3.0.0"},
		{"adjagent-sheet-2.0.1", "adjagent-sheet-3.0.0"},
	} {
		t.Run(pair.in, func(t *testing.T) {
			got, obsolete, warnings, err := Convert200To300(readTree(t, path.Join(testdata, pair.in)))
			if err != nil {
				t.Fatalf("Convert200To300: %v", err)
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

// TestGoldenRefusal: the bad-token fixture is refused with the 2.0.x load's
// own faults, the golden that load's test pins.
func TestGoldenRefusal(t *testing.T) {
	_, _, _, err := Convert200To300(readTree(t, path.Join(testdata, "bad-token-2.0.0")))
	var ann *format200.AnnotationError
	if !errors.As(err, &ann) {
		t.Fatalf("err = %v, want the 2.0.x load's annotation refusal", err)
	}
	got, err := yaml.Marshal(ann.Faults)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(path.Join(testdata, "bad-token-2.0.0.faults.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("faults\n%s\nwant\n%s", got, want)
	}
}

// TestAConvertedKBConvertsToItself: a save that wrote every converted file
// and stopped before the stamp leaves a KB whose next conversion changes no
// byte, makes nothing obsolete and warns of nothing.
func TestAConvertedKBConvertsToItself(t *testing.T) {
	for _, golden := range []string{"register-3.0.0", "adjagent-sheet-3.0.0"} {
		t.Run(golden, func(t *testing.T) {
			want := readTree(t, path.Join(testdata, golden))
			got, obsolete, warnings, err := Convert200To300(want)
			if err != nil {
				t.Fatalf("Convert200To300: %v", err)
			}
			if len(obsolete) != 0 || len(warnings) != 0 || !maps.EqualFunc(got, want, bytes.Equal) {
				t.Errorf("re-converting %s: obsolete %q, warnings %+v, files equal %t", golden, obsolete, warnings, maps.EqualFunc(got, want, bytes.Equal))
			}
		})
	}
}

// TestAStoppedSaveIsTakenAsItStands: where the node records stand beside a
// leaf still carrying its 2.0.x frontmatter — a save stopped between the
// records and the leaves — the records are taken byte for byte, the leaf's
// frontmatter is removed without being read, no marker is placed, the 2.0.x
// index files are obsolete, and nothing is warned of.
func TestAStoppedSaveIsTakenAsItStands(t *testing.T) {
	files := readTree(t, path.Join(testdata, "register-2.0.0"))
	stood := readTree(t, path.Join(testdata, "register-3.0.0"))
	nodes := []byte(`--- {"kind": "claim", "id": "clm-aaaaaa", "title": "Only This"}` + "\n")
	files[nodesPath], files[edgesPath] = nodes, stood[edgesPath]
	files["kb-root/.index/claims.yaml"] = []byte("--- {}\n")
	got, obsolete, warnings, err := Convert200To300(files)
	if err != nil {
		t.Fatalf("Convert200To300: %v", err)
	}
	if !bytes.Equal(got[nodesPath], nodes) || !bytes.Equal(got[edgesPath], stood[edgesPath]) {
		t.Errorf("the records were rewritten:\n%s\n%s", got[nodesPath], got[edgesPath])
	}
	if !slices.Equal(obsolete, []string{"kb-root/.index/claims.yaml"}) || len(warnings) != 0 {
		t.Errorf("obsolete %q, warnings %+v; want the 2.0.x index file and none", obsolete, warnings)
	}
	leaf := string(files[leafPath])
	if want := leaf[format200.FrontmatterEnd(leaf):]; string(got[leafPath]) != want {
		t.Errorf("the leaf converted to\n%s\nwant its body as it stood, unmarked\n%s", got[leafPath], want)
	}
	if !bytes.Equal(got[entryPoint], stood[entryPoint]) {
		t.Errorf("the entry point converted to\n%s\nwant\n%s", got[entryPoint], stood[entryPoint])
	}
}
