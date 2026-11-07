package kb

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestBearing: a bearing is a number in [0, 1] or the pending literal, and
// each value reads and writes back as itself; zero is the judgement 0.0, never
// the absence of one.
func TestBearing(t *testing.T) {
	for _, tc := range []struct {
		in, text string
		ok       bool
	}{
		{"0", "0.0", true}, {"1", "1.0", true}, {"0.4", "0.4", true}, {PendingLiteral, PendingLiteral, true},
		{"1.5", "", false}, {"-0.1", "", false}, {"sometimes", "", false}, {"", "", false},
	} {
		b, ok := BearingFromValue(tc.in)
		if ok != tc.ok {
			t.Errorf("BearingFromValue(%q) ok = %t, want %t", tc.in, ok, tc.ok)
			continue
		}
		if !ok {
			continue
		}
		if got := BearingText(b); got != tc.text {
			t.Errorf("BearingText(%q) = %q, want %q", tc.in, got, tc.text)
		}
		again, ok := BearingFromValue(BearingText(b))
		if !ok || again != b {
			t.Errorf("(bearing %q) read back as %+v, %t; want %+v", BearingText(b), again, ok, b)
		}
	}
}

// TestDocumentsExcludesOnlyTheNamedClasses: every Markdown document under
// kb-root is discovered, those in session/ and tools/ and an invariants.md
// included, except one under .index/ and the summary page, axioms.md and the
// maintained-block documents, which are no leaves.
func TestDocumentsExcludesOnlyTheNamedClasses(t *testing.T) {
	root := filepath.Join(t.TempDir(), KBDir)
	for _, rel := range []string{
		"session/notes.md", "tools/claim-quality.md", InvariantsFile, "v/a.md",
		IndexDir + "/stray.md", RegisterFile, "v/" + RegisterFile, AxiomsFile,
		AgentsFile, AgentsRedirectFile, ConventionsFile, "README.md",
	} {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("# A document\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{InvariantsFile, "session/notes.md", "tools/claim-quality.md", "v/a.md"}
	if docs, err := Documents(OnDisk(root)); err != nil || !slices.Equal(docs, want) {
		t.Fatalf("Documents = %q, %v; want %q", docs, err, want)
	}
}

// TestDocumentsRefusesASymlinkedDocument: a document that is a symbolic link
// is refused, naming the path, rather than read through it; a tree of regular
// documents lists.
func TestDocumentsRefusesASymlinkedDocument(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, KBDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	const text = "# A document\n"
	outside := filepath.Join(t.TempDir(), "doc.md")
	if err := os.WriteFile(outside, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "doc.md")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	_, err := Documents(OnDisk(root))
	var sym *SymlinkError
	if !errors.As(err, &sym) || sym.Path != link {
		t.Fatalf("Documents over a symlinked document = %v, want a SymlinkError naming %s", err, link)
	}

	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(link, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	if docs, err := Documents(OnDisk(root)); err != nil || !slices.Equal(docs, []string{"doc.md"}) {
		t.Fatalf("Documents over a document that stands where it is = %q, %v; want doc.md", docs, err)
	}
}
