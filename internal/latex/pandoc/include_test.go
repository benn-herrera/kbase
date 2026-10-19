package pandoc_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kbase/internal/latex/pandoc"
	"kbase/internal/log"
)

// writeFile writes body at path, making its parent directories.
func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestReadLaTeXRefusesAnIncludeOutsideTheVolume: an \input or \include pandoc
// would inline from outside the volume root's directory is refused before it
// runs, naming the include and the file that names it, along every way out:
// an absolute path, .., a second-level include, and a symlink inside the
// corpus pointing out of it.
func TestReadLaTeXRefusesAnIncludeOutsideTheVolume(t *testing.T) {
	root := t.TempDir()
	volume := filepath.Join(root, "volume")
	writeFile(t, filepath.Join(root, "secret.tex"), "Outside the volume.\n")
	writeFile(t, filepath.Join(volume, "sub", "first.tex"), "\\input{../secret.tex}\n")
	if err := os.Symlink(filepath.Join(root, "secret.tex"), filepath.Join(volume, "link.tex")); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, source, wantTarget, wantOrigin string
	}{
		{"absolute", "\\input{" + filepath.Join(root, "secret.tex") + "}\n", filepath.Join(root, "secret.tex"), ""},
		{"parent", "\\input{../secret.tex}\n", "../secret.tex", ""},
		{"parent without an extension", "\\input{../secret}\n", "../secret", ""},
		{"include command", "\\include{../secret.tex}\n", "../secret.tex", ""},
		{"below a first-level include", "\\input{sub/first.tex}\n", "../secret.tex", "sub/first.tex"},
		{"through a symlink", "\\input{link.tex}\n", "link.tex", ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := pandoc.ReadLaTeX(context.Background(), log.Discard(), volume, c.source, nil)
			var escape pandoc.EscapeError
			if !errors.As(err, &escape) {
				t.Fatalf("read of %q = %v, want an include escape refused", c.source, err)
			}
			if escape.Target != c.wantTarget || escape.Origin != c.wantOrigin {
				t.Errorf("escape = %+v, want target %q named at %q", escape, c.wantTarget, c.wantOrigin)
			}
			if !strings.Contains(err.Error(), "outside the volume's directory") {
				t.Errorf("refusal = %q, want it naming the volume's directory", err)
			}
		})
	}
}

// TestReadLaTeXReadsAnIncludeInsideTheVolume: a sibling file, a subdirectory
// file named without its extension, and an include commented out are read as
// pandoc reads them.
func TestReadLaTeXReadsAnIncludeInsideTheVolume(t *testing.T) {
	requirePandoc(t)
	volume := t.TempDir()
	writeFile(t, filepath.Join(volume, "sib.tex"), "SIBLING\n")
	writeFile(t, filepath.Join(volume, "sub", "deep.tex"), "DEEP\n")
	source := "\\input{sib.tex}\n\\input{sub/deep}\n% \\input{../commented.tex}\n"
	ast, err := pandoc.ReadLaTeX(context.Background(), log.Discard(), volume, source, nil)
	if err != nil {
		t.Fatalf("read of an include inside the volume = %v, want it", err)
	}
	for _, want := range []string{"SIBLING", "DEEP"} {
		if !strings.Contains(string(ast), want) {
			t.Errorf("the AST does not carry %q:\n%s", want, ast)
		}
	}
}

// TestReadLaTeXLeavesAMissingIncludeToPandoc: a target that resolves inside
// the volume and names no file is the reader's own warning, which is refused
// as an unloadable include and not as an escape.
func TestReadLaTeXLeavesAMissingIncludeToPandoc(t *testing.T) {
	requirePandoc(t)
	_, err := pandoc.ReadLaTeX(context.Background(), log.Discard(), t.TempDir(), "\\input{absent.tex}\n", nil)
	var include pandoc.IncludeError
	var escape pandoc.EscapeError
	if !errors.As(err, &include) || errors.As(err, &escape) {
		t.Fatalf("read of a missing include = %v, want an unloadable include", err)
	}
}
