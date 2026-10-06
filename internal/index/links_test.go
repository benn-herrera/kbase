package index

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"kbase/internal/kb"
)

func TestLinkFindingsResolveAgainstTheRepository(t *testing.T) {
	root := writeKB(t, map[string]string{
		"leaf.md": "[a](missing.md)\n[b](../assets/x.pdf)\n[c](../assets/gone.pdf)\n",
	})
	repo := filepath.Dir(root)
	for rel, text := range map[string]string{"docs/x.md": "[a](missing.md)\n", "assets/x.pdf": "%PDF\n"} {
		p := filepath.Join(repo, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	found, err := LinkFindings(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range found {
		got = append(got, f.Detail)
	}
	want := []string{`leaf.md:1 broken intra "missing.md"`, `leaf.md:3 broken intra "../assets/gone.pdf"`}
	if !slices.Equal(got, want) {
		t.Errorf("LinkFindings =\n%q\nwant\n%q", got, want)
	}
}
