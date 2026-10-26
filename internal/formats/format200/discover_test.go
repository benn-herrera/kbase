package format200_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/formats/common"
	"kbase/internal/formats/format200"
)

const (
	register200Dir = "../testdata/register-2.0.0"
	badToken200Dir = "../testdata/bad-token-2.0.0"

	registerGolden = "../testdata/register-2.0.0.state.yaml"
	badTokenGolden = "../testdata/bad-token-2.0.0.faults.yaml"

	gotDir = "../../../test-data/transient/unit_tests/formats/got"
)

func drop(string, int, string) {}

func readTree(t *testing.T, dir, prefix string, into common.Files) {
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

func fixtureTree(t *testing.T, dir string) common.Files {
	t.Helper()
	files := common.Files{}
	readTree(t, dir, "", files)
	return files
}

func marshal(t *testing.T, v any) []byte {
	t.Helper()
	b, err := yaml.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling %T: %v", v, err)
	}
	return b
}

// checkGolden compares got with the golden file; on a difference it writes
// got under the golden's name to gotDir, so a golden is written by copying it in.
func checkGolden(t *testing.T, golden string, got []byte) {
	t.Helper()
	want, err := os.ReadFile(golden)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	if err == nil && bytes.Equal(got, want) {
		return
	}
	if err := os.MkdirAll(gotDir, 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(gotDir, filepath.Base(golden))
	if err := os.WriteFile(out, got, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Errorf("%s differs from %s; got written to %s", filepath.Base(golden), golden, out)
}

func copyFaults(t *testing.T, err error) []format200.AnnotationFault {
	t.Helper()
	var ae *format200.AnnotationError
	if !errors.As(err, &ae) {
		t.Fatalf("copy refused with %T (%v), want *AnnotationError", err, err)
	}
	return ae.Faults
}

// TestDiscoverRegisterGolden: the register fixture parses to its golden State.
func TestDiscoverRegisterGolden(t *testing.T) {
	st, err := format200.Discover(fixtureTree(t, register200Dir), drop)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	checkGolden(t, registerGolden, marshal(t, st))
}

// TestDiscoverBadTokenGolden: the bad-token fixture is refused with its golden faults.
func TestDiscoverBadTokenGolden(t *testing.T) {
	_, err := format200.Discover(fixtureTree(t, badToken200Dir), drop)
	checkGolden(t, badTokenGolden, marshal(t, copyFaults(t, err)))
}
