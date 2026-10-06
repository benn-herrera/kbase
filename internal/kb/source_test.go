package kb

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// TestMigratedSourceReads: a migrated source reads the bytes the migration
// produced in place of the disk's, finds a file the disk does not hold, and
// reads an obsolete path as absent, in listings too; Put is read back, and
// Saved makes the source current.
func TestMigratedSourceReads(t *testing.T) {
	repo := t.TempDir()
	root := filepath.Join(repo, KBDir)
	for rel, text := range map[string]string{
		"kb-root/a.md": "<!-- old form -->\n", "kb-root/.index/claims.jsonl": "{}\n", "kb-build-node-pass.json": "{}\n", "kb-root/b.md": "untouched\n",
	} {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	src := MigratedSource(root, UnstampedFormatVersion, map[string][]byte{
		"kb-root/a.md": []byte("---\nkind: leaf\n---\n"), "kb-root/.index/claims.yaml": []byte("--- {}\n"), "kb-build-node-pass.yaml": []byte("about: x\n"),
	}, []string{"kb-root/.index/claims.jsonl", "kb-build-node-pass.json"})
	for p, want := range map[string]string{
		filepath.Join(root, "a.md"): "---\nkind: leaf\n---\n", filepath.Join(root, "b.md"): "untouched\n",
		filepath.Join(root, IndexDir, "claims.yaml"): "--- {}\n", filepath.Join(repo, "kb-build-node-pass.yaml"): "about: x\n",
	} {
		if got, err := src.ReadText(p); err != nil || got != want {
			t.Errorf("ReadText(%s) = %q, %v; want %q", p, got, err, want)
		}
	}
	for _, p := range []string{filepath.Join(root, IndexDir, "claims.jsonl"), filepath.Join(repo, "kb-build-node-pass.json")} {
		if _, err := src.ReadFile(p); !ErrNotExist(err) || src.IsFile(p) {
			t.Errorf("%s reads as present (%v)", p, err)
		}
	}
	files, err := src.DirFiles(filepath.Join(root, IndexDir))
	if err != nil || !slices.Equal(files, []string{filepath.Join(root, IndexDir, "claims.yaml")}) {
		t.Errorf("DirFiles(.index) = %q, %v", files, err)
	}
	if docs, err := Documents(src); err != nil || !slices.Equal(docs, []string{"a.md", "b.md"}) {
		t.Errorf("Documents = %q, %v", docs, err)
	}
	if got := src.MigratedFiles(); !slices.Equal(got, []string{"kb-build-node-pass.yaml", "kb-root/.index/claims.yaml", "kb-root/a.md"}) {
		t.Errorf("MigratedFiles = %q", got)
	}
	src.Put(filepath.Join(root, "b.md"), []byte("put\n"))
	if got, _ := src.ReadText(filepath.Join(root, "b.md")); got != "put\n" {
		t.Errorf("a Put file reads back as %q", got)
	}
	if !src.Migrated() || src.Version() != UnstampedFormatVersion {
		t.Errorf("Migrated %t at %s", src.Migrated(), src.Version())
	}
	src.Saved()
	if src.Migrated() || src.Version() != FormatVersion || len(src.Obsolete()) != 0 {
		t.Errorf("after Saved: migrated %t at %s, obsolete %q", src.Migrated(), src.Version(), src.Obsolete())
	}
}

// TestTreeLinksBelowTheFrontmatter: the up-link line is the first line after
// the frontmatter's closing fence, and no line of the frontmatter is read
// for links.
func TestTreeLinksBelowTheFrontmatter(t *testing.T) {
	for _, tc := range []struct {
		name, text, parent string
		hasParent          bool
		children           []string
	}{
		{"frontmatter then up-link", "---\nkind: index\n---\n[↑ Up](../index.md)\n\n- [A](a.md)\n", "index.md", true, []string{"v/a.md"}},
		{"no frontmatter", "[↑ Up](../index.md)\n\n- [A](a.md)\n", "index.md", true, []string{"v/a.md"}},
		{"up-link not first after the frontmatter", "---\nkind: index\n---\n\n[↑ Up](../index.md)\n", "", false, []string{"index.md"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			parent, ok, children := TreeLinks("v/index.md", tc.text)
			if parent != tc.parent || ok != tc.hasParent || !slices.Equal(children, tc.children) {
				t.Errorf("TreeLinks = %q, %t, %q; want %q, %t, %q", parent, ok, children, tc.parent, tc.hasParent, tc.children)
			}
		})
	}
}
