package kb

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolvePath: the existing prefix is resolved through its symlinks, the
// missing rest kept as written; a path nothing of which resolves is as given.
func TestResolvePath(t *testing.T) {
	real, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(real, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(real, "link")
	if err := os.Symlink(filepath.Join(real, "dir"), link); err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ name, in, want string }{
		{"existing, through a link", link, filepath.Join(real, "dir")},
		{"missing tail under a link", filepath.Join(link, "a", "b.tex"), filepath.Join(real, "dir", "a", "b.tex")},
		{"uncleaned", link + "/./x/../y", filepath.Join(real, "dir", "y")},
		{"nothing below the root exists", "/no-such-root-kbase/a", "/no-such-root-kbase/a"},
		{"relative, nothing exists", "no-such-kbase-dir/a", "no-such-kbase-dir/a"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := ResolvePath(c.in); got != c.want {
				t.Errorf("ResolvePath(%q) = %q; want %q", c.in, got, c.want)
			}
		})
	}
}

// TestSheetDigestDrawn: two or more volumes hold nodes draws the digest;
// the kb-root-level nodes are not a volume (SPEC §3), so root-only claims
// draw the full sheet alone — the README's digest link would dangle.
func TestSheetDigestDrawn(t *testing.T) {
	for _, tc := range []struct {
		name  string
		paths []string
		want  bool
	}{
		{"one volume", []string{"vol/claim-quality.md"}, false},
		{"root bucket alone", []string{"claim-quality.md", "other.md"}, false},
		{"root and one volume", []string{"claim-quality.md", "vol/claim-quality.md"}, false},
		{"two volumes", []string{"one/claim-quality.md", "two/claim-quality.md"}, true},
		{"one volume plus root", []string{"one/claim-quality.md", "claim-quality.md"}, false},
	} {
		nodes := make([]NodeRow, len(tc.paths))
		for i, p := range tc.paths {
			nodes[i] = NodeRow{CanonicalPath: p}
		}
		if got := SheetDigestDrawn(nodes); got != tc.want {
			t.Errorf("%s: SheetDigestDrawn = %t, want %t", tc.name, got, tc.want)
		}
	}
}
