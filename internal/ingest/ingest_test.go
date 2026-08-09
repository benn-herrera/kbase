package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// writeFile creates dir/name (parents included) with the given contents.
func writeFile(t *testing.T, root, name, body string) string {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", name, err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return p
}

// paths lists the ingested unit ids in order, which is the order tests assert
// against: sorted, slash-separated, relative to the root.
func paths(c Corpus) []string {
	out := make([]string, 0, len(c.Units))
	for _, u := range c.Units {
		out = append(out, u.Path)
	}
	return out
}

// TestWalkMarkdownSelection covers what the walk collects and what it leaves
// alone: nested Markdown in path order, a case-variant extension, and the
// three categories that are not documents (dot-directories, dot-files,
// non-Markdown files).
func TestWalkMarkdownSelection(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "index.md", "# Index\n")
	writeFile(t, root, "guide/setup.md", "# Setup\n")
	writeFile(t, root, "guide/deep/nested.MD", "# Nested\n")
	writeFile(t, root, "notes.txt", "not markdown\n")
	writeFile(t, root, ".hidden/secret.md", "# Hidden\n")
	writeFile(t, root, ".dotfile.md", "# Dotfile\n")
	writeFile(t, root, "guide/.draft.md", "# Draft\n")

	c, err := WalkMarkdown(root)
	if err != nil {
		t.Fatalf("WalkMarkdown: %v", err)
	}
	want := []string{"guide/deep/nested.MD", "guide/setup.md", "index.md"}
	if got := paths(c); !slices.Equal(got, want) {
		t.Errorf("units: got %v, want %v", got, want)
	}
	if c.Root != root {
		t.Errorf("Root = %q, want %q", c.Root, root)
	}
	if !c.Has("guide/setup.md") || c.Has("notes.txt") {
		t.Error("Has must answer for ingested paths only")
	}
}

// TestWalkMarkdownCustody: the bytes under custody are the file's bytes,
// unnormalized, and the digest is over exactly those bytes. CRLF and a
// missing trailing newline are the cases a "helpful" reader would silently
// fix — fixing them would invalidate every byte offset the survey records.
func TestWalkMarkdownCustody(t *testing.T) {
	const body = "# Title\r\n\r\ntext without trailing newline"
	root := t.TempDir()
	writeFile(t, root, "doc.md", body)

	c, err := WalkMarkdown(root)
	if err != nil {
		t.Fatalf("WalkMarkdown: %v", err)
	}
	u := c.Units[0]
	if string(u.Bytes) != body {
		t.Errorf("custody bytes = %q, want %q", u.Bytes, body)
	}
	sum := sha256.Sum256([]byte(body))
	if want := hex.EncodeToString(sum[:]); u.SHA256 != want {
		t.Errorf("SHA256 = %q, want %q", u.SHA256, want)
	}
}

// TestWalkMarkdownSymlinks pins the two halves of the symlink policy: a file
// link is content and is followed; a directory link is refused loudly rather
// than walked into.
func TestWalkMarkdownSymlinks(t *testing.T) {
	t.Run("file link is followed", func(t *testing.T) {
		outside := t.TempDir()
		target := writeFile(t, outside, "external.md", "# External\n")
		root := t.TempDir()
		writeFile(t, root, "index.md", "# Index\n")
		if err := os.Symlink(target, filepath.Join(root, "linked.md")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		c, err := WalkMarkdown(root)
		if err != nil {
			t.Fatalf("WalkMarkdown: %v", err)
		}
		if got, want := paths(c), []string{"index.md", "linked.md"}; !slices.Equal(got, want) {
			t.Errorf("units: got %v, want %v", got, want)
		}
		u, _ := c.Unit("linked.md")
		if string(u.Bytes) != "# External\n" {
			t.Errorf("linked unit bytes = %q, want the link target's content", u.Bytes)
		}
	})

	t.Run("directory link is refused", func(t *testing.T) {
		outside := t.TempDir()
		writeFile(t, outside, "external.md", "# External\n")
		root := t.TempDir()
		writeFile(t, root, "index.md", "# Index\n")
		if err := os.Symlink(outside, filepath.Join(root, "elsewhere")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		_, err := WalkMarkdown(root)
		if err == nil {
			t.Fatal("expected a symlinked-directory refusal, got nil")
		}
		if !strings.Contains(err.Error(), "symlink to a directory") {
			t.Errorf("error = %v, want it to name the symlinked directory policy", err)
		}
	})

	t.Run("broken link fails loudly", func(t *testing.T) {
		root := t.TempDir()
		writeFile(t, root, "index.md", "# Index\n")
		if err := os.Symlink(filepath.Join(root, "gone.md"), filepath.Join(root, "dangling.md")); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}

		_, err := WalkMarkdown(root)
		if err == nil {
			t.Fatal("expected a dangling-symlink failure, got nil")
		}
		if !strings.Contains(err.Error(), "dangling.md") {
			t.Errorf("error = %v, want it to name the offending path", err)
		}
	})
}

// TestWalkMarkdownRootFailures: a mistyped or non-directory root is a loud
// failure, and so is a directory holding no Markdown at all.
func TestWalkMarkdownRootFailures(t *testing.T) {
	root := t.TempDir()
	file := writeFile(t, root, "doc.md", "# Doc\n")

	for _, tc := range []struct {
		name, root, want string
	}{
		{"missing root", filepath.Join(root, "nope"), "corpus root"},
		{"root is a file", file, "is not a directory"},
		{"no markdown", t.TempDir(), "no .md files"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := WalkMarkdown(tc.root)
			if err == nil {
				t.Fatalf("expected an error containing %q, got nil", tc.want)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

// TestNewOrdersAndRejects: New is the single constructor, so ordering and the
// two malformed-input refusals are pinned here rather than through the walk.
func TestNewOrdersAndRejects(t *testing.T) {
	c, err := New("/corpus", []Unit{
		{Path: "b.md", Bytes: []byte("b")},
		{Path: "a/deep.md", Bytes: []byte("d")},
		{Path: "a.md", Bytes: []byte("a")},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got, want := paths(c), []string{"a.md", "a/deep.md", "b.md"}; !slices.Equal(got, want) {
		t.Errorf("units: got %v, want %v (sorted by path)", got, want)
	}

	if _, err := New("/corpus", []Unit{{Path: "x.md"}, {Path: "x.md"}}); err == nil {
		t.Error("duplicate paths must be refused")
	}
	if _, err := New("/corpus", []Unit{{Bytes: []byte("x")}}); err == nil {
		t.Error("a unit without a path must be refused")
	}
}

// TestContentHashIdentity: the corpus hash is the source identity in the
// provenance stamp, so it must be stable for identical input, insensitive to
// the order units arrive in, and sensitive to a rename that moves no bytes.
func TestContentHashIdentity(t *testing.T) {
	base := []Unit{
		{Path: "a.md", Bytes: []byte("alpha")},
		{Path: "b.md", Bytes: []byte("beta")},
	}
	shuffled := []Unit{base[1], base[0]}
	renamed := []Unit{{Path: "a.md", Bytes: []byte("alpha")}, {Path: "c.md", Bytes: []byte("beta")}}
	edited := []Unit{{Path: "a.md", Bytes: []byte("alpha!")}, {Path: "b.md", Bytes: []byte("beta")}}

	hash := func(us []Unit) string {
		t.Helper()
		c, err := New("/corpus", us)
		if err != nil {
			t.Fatalf("New: %v", err)
		}
		return c.ContentHash
	}

	want := hash(base)
	if got := hash(shuffled); got != want {
		t.Error("content hash must not depend on input order")
	}
	if got := hash(renamed); got == want {
		t.Error("content hash must change when a file is renamed")
	}
	if got := hash(edited); got == want {
		t.Error("content hash must change when a file's bytes change")
	}
}
