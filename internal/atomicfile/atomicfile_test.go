package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// TestWriteInRefusesASymlink: a document that is a symbolic link is not
// written through, and the file the link names keeps its bytes.
func TestWriteInRefusesASymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "claims.yaml")
	writeFile(t, target, "old\n")
	link := filepath.Join(root, "claims.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	err := WriteIn(root, link, []byte("new\n"))
	var refused *LinkError
	if !errors.As(err, &refused) || !strings.Contains(refused.Detail, "symbolic link") || !strings.Contains(refused.Detail, link) {
		t.Fatalf("WriteIn over a symlink = %v, want a link refusal naming %s", err, link)
	}
	if got := readFile(t, target); got != "old\n" {
		t.Errorf("the file the link names = %q, want it untouched", got)
	}
}

// TestWriteInRefusesATargetOutsideRoot: a document below a directory that is a
// symbolic link out of the tree is refused, and the file it would have
// replaced keeps its bytes.
func TestWriteInRefusesATargetOutsideRoot(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "claims.yaml")
	writeFile(t, target, "old\n")
	if err := os.Symlink(outside, filepath.Join(root, "sub")); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "sub", "claims.yaml")

	err := WriteIn(root, path, []byte("new\n"))
	var refused *LinkError
	if !errors.As(err, &refused) || !strings.Contains(refused.Detail, "outside") || !strings.Contains(refused.Detail, path) {
		t.Fatalf("WriteIn through a symlinked directory = %v, want a link refusal naming %s", err, path)
	}
	if got := readFile(t, target); got != "old\n" {
		t.Errorf("%s = %q, want it untouched", target, got)
	}
	if entries, err := os.ReadDir(outside); err != nil || len(entries) != 1 {
		t.Errorf("the outside directory holds %v (%v), want only the target", entries, err)
	}
}

// TestWriteInReplacesAFileInsideRoot: a document that stays where it stands is
// written, leaving no temp behind.
func TestWriteInReplacesAFileInsideRoot(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".index", "claims.yaml")
	if err := os.Mkdir(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "old\n")

	if err := WriteIn(root, path, []byte("new\n")); err != nil {
		t.Fatalf("WriteIn inside root: %v", err)
	}
	if got := readFile(t, path); got != "new\n" {
		t.Errorf("%s = %q, want the written bytes", path, got)
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "claims.yaml" {
		t.Errorf("the directory holds %v, want only the document", entries)
	}
}
