package format_2_0_0_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"testing"

	"go.yaml.in/yaml/v3"

	"kbase/internal/kb"
	"kbase/internal/log"
	"kbase/internal/migrate/common"
	"kbase/internal/migrate/format_2_0_0"
)

const (
	register200Dir = "../testdata/register-2.0.0"
	badToken200Dir = "../testdata/bad-token-2.0.0"

	registerGolden = "../testdata/register-2.0.0.state.yaml"
	badTokenGolden = "../testdata/bad-token-2.0.0.faults.yaml"

	gotDir = "../../../test-data/transient/unit_tests/migrate/got"
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

// discover loads the files through kb.Discover; a test helper does not cross packages.
func discover(t *testing.T, files common.Files) (kb.State, error) {
	t.Helper()
	repo := t.TempDir()
	kbRoot := filepath.Join(repo, kb.KBDir)
	if err := os.MkdirAll(kbRoot, 0o755); err != nil {
		t.Fatal(err)
	}
	return kb.Discover(kb.MigratedSource(kbRoot, format_2_0_0.Version200, files, nil), log.Discard())
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

func copyFaults(t *testing.T, err error) []format_2_0_0.AnnotationFault {
	t.Helper()
	var ae *format_2_0_0.AnnotationError
	if !errors.As(err, &ae) {
		t.Fatalf("copy refused with %T (%v), want *AnnotationError", err, err)
	}
	return ae.Faults
}

func kbFaults(t *testing.T, err error) []kb.AnnotationFault {
	t.Helper()
	var ae *kb.AnnotationError
	if !errors.As(err, &ae) {
		t.Fatalf("kb.Discover refused with %T (%v), want *AnnotationError", err, err)
	}
	return ae.Faults
}

// TestDiscoverRegisterGolden: the register fixture parses to its golden State.
func TestDiscoverRegisterGolden(t *testing.T) {
	st, err := format_2_0_0.Discover(fixtureTree(t, register200Dir), drop)
	if err != nil {
		t.Fatalf("Discover: %v", err)
	}
	checkGolden(t, registerGolden, marshal(t, st))
}

// TestDiscoverBadTokenGolden: the bad-token fixture is refused with its golden faults.
func TestDiscoverBadTokenGolden(t *testing.T) {
	_, err := format_2_0_0.Discover(fixtureTree(t, badToken200Dir), drop)
	checkGolden(t, badTokenGolden, marshal(t, copyFaults(t, err)))
}

// TestDiscoverAgreesWithKB: over each fixture the copy and kb.Discover yield
// byte-equal marshals, of the state for the register and of the faults for the
// bad token.
func TestDiscoverAgreesWithKB(t *testing.T) {
	t.Run("register", func(t *testing.T) {
		files := fixtureTree(t, register200Dir)
		want, err := discover(t, files)
		if err != nil {
			t.Fatalf("kb.Discover: %v", err)
		}
		got, err := format_2_0_0.Discover(files, drop)
		if err != nil {
			t.Fatalf("copy Discover: %v", err)
		}
		if g, w := marshal(t, got), marshal(t, want); !bytes.Equal(g, w) {
			t.Errorf("copy state\n%s\nkb state\n%s", g, w)
		}
	})
	t.Run("bad-token", func(t *testing.T) {
		files := fixtureTree(t, badToken200Dir)
		_, kbErr := discover(t, files)
		_, copyErr := format_2_0_0.Discover(files, drop)
		g, w := marshal(t, copyFaults(t, copyErr)), marshal(t, kbFaults(t, kbErr))
		if !bytes.Equal(g, w) {
			t.Errorf("copy faults\n%s\nkb faults\n%s", g, w)
		}
	})
}
