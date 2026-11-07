package atomicfile

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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

// TestWriteInRefusesASymlink: a file that is a symbolic link is not
// written through, and the file the link names keeps its bytes.
func TestWriteInRefusesASymlink(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(t.TempDir(), "claims.yaml")
	writeFile(t, target, "old\n")
	link := filepath.Join(root, "claims.yaml")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	err := WriteIn(root, filepath.Join(root, "temp"), link, []byte("new\n"))
	var refused *LinkError
	if !errors.As(err, &refused) || !strings.Contains(refused.Detail, "symbolic link") || !strings.Contains(refused.Detail, link) {
		t.Fatalf("WriteIn over a symlink = %v, want a link refusal naming %s", err, link)
	}
	if got := readFile(t, target); got != "old\n" {
		t.Errorf("the file the link names = %q, want it untouched", got)
	}
}

// TestWriteInRefusesATargetOutsideRoot: a file below a directory that is a
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

	err := WriteIn(root, filepath.Join(root, "temp"), path, []byte("new\n"))
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

// TestWriteModeSetsTheModeExactly: a new file lands at the mode asked for,
// group and other write bits the usual umask strips included, and a replaced
// file takes the mode asked for in place of its own.
func TestWriteModeSetsTheModeExactly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix file modes")
	}
	dir := t.TempDir()
	for _, tc := range []struct {
		name     string
		existing os.FileMode // 0: no file stands
		mode     os.FileMode
	}{
		{"created", 0, 0o666},
		{"replaced", 0o600, 0o644},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(dir, tc.name)
			if tc.existing != 0 {
				writeFile(t, path, "old\n")
				if err := os.Chmod(path, tc.existing); err != nil {
					t.Fatal(err)
				}
			}
			if err := WriteMode(path, []byte("new\n"), tc.mode); err != nil {
				t.Fatalf("WriteMode: %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != tc.mode {
				t.Errorf("mode %#o, want %#o", got, tc.mode)
			}
			if got := readFile(t, path); got != "new\n" {
				t.Errorf("%s = %q, want the written bytes", path, got)
			}
		})
	}
}

// names is the entry names of dir.
func names(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}

// TestWriteInMakesItsTempInTheTempDir: a file inside root is written
// through a temp in the temp directory, which the write creates holding its
// ignore file and nothing else once the write is done; nothing is left
// beside the file.
func TestWriteInMakesItsTempInTheTempDir(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "volume", "claims.yaml")
	if err := os.Mkdir(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, "old\n")
	temp := filepath.Join(root, ".index", "kb-temp")

	if err := WriteIn(root, temp, path, []byte("new\n")); err != nil {
		t.Fatalf("WriteIn inside root: %v", err)
	}
	if got := readFile(t, path); got != "new\n" {
		t.Errorf("%s = %q, want the written bytes", path, got)
	}
	if got := names(t, filepath.Dir(path)); !slices.Equal(got, []string{"claims.yaml"}) {
		t.Errorf("the file's directory holds %v, want only the file", got)
	}
	if got := names(t, temp); !slices.Equal(got, []string{ignoreFile}) {
		t.Errorf("the temp directory holds %v, want only %s", got, ignoreFile)
	}
	if got := readFile(t, filepath.Join(temp, ignoreFile)); got != ignoreAll {
		t.Errorf("%s = %q, want %q", ignoreFile, got, ignoreAll)
	}
}

// TestWriteInRestoresTheIgnoreFile: an ignore file that no longer ignores
// everything is rewritten before the write makes its temp beside it.
func TestWriteInRestoresTheIgnoreFile(t *testing.T) {
	root := t.TempDir()
	temp := filepath.Join(root, "kb-temp")
	if err := os.Mkdir(temp, 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(temp, ignoreFile), "!keep\n")

	if err := WriteIn(root, temp, filepath.Join(root, "a.md"), []byte("new\n")); err != nil {
		t.Fatalf("WriteIn: %v", err)
	}
	if got := readFile(t, filepath.Join(temp, ignoreFile)); got != ignoreAll {
		t.Errorf("%s = %q, want it rewritten to %q", ignoreFile, got, ignoreAll)
	}
}
