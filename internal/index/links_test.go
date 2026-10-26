package index

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"kbase/internal/kb"
)

// graphFiles is g's two record files by kb-root-relative path, as writeKB
// takes them.
func graphFiles(g kb.Graph) map[string]string {
	return map[string]string{kb.GraphNodesFile: string(kb.EncodeNodes(g)), kb.GraphEdgesFile: string(kb.EncodeEdges(g))}
}

func TestLinkFindingsResolveAgainstTheRepository(t *testing.T) {
	files := graphFiles(kb.Graph{})
	files["leaf.md"] = "[a](missing.md)\n[b](../assets/x.pdf)\n[c](../assets/gone.pdf)\n"
	root := writeKB(t, files)
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

// TestLinkFindingsReportAMovedNodesOldFragment: a claim whose marker moved from
// volume a to volume b is rendered on b's page, so a prose link to its old
// entry on a's page — which still stands, unrefreshed and holding the entry —
// is a dead link in the linking document, while the link to b's page holds.
func TestLinkFindingsReportAMovedNodesOldFragment(t *testing.T) {
	files := graphFiles(kb.Graph{Claims: []kb.ClaimNode{{ID: "clm-aaaaaa", Title: "Theorem 1"}}})
	for rel, text := range map[string]string{
		kb.EntryPointFile: "# KB\n\n- [a](a/index.md)\n- [b](b/index.md)\n",
		"a/index.md":      "[↑ KB](../entry-point.md)\n\n# A\n\n- [prose](prose.md)\n",
		"a/prose.md": "[↑ A](index.md)\n\n# Prose\n\nSee [the old entry](claim-rigor.md#theorem-1) and " +
			"[the new one](../b/claim-rigor.md#theorem-1).\n",
		"a/claim-rigor.md": "# Claim register\n\n## Theorem 1\n<!-- id: clm-aaaaaa -->\n",
		"b/index.md":       "[↑ KB](../entry-point.md)\n\n# B\n\n- [leaf](leaf.md)\n",
		"b/leaf.md":        "[↑ B](index.md)\n\n# Leaf\n\nA theorem. <!-- claim-rigor: clm-aaaaaa -->\n",
	} {
		files[rel] = text
	}
	root := writeKB(t, files)
	src := kb.OnDisk(root)
	pages, err := renderRegisterPages(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "b", kb.RegisterFile), pages["b/"+kb.RegisterFile], 0o644); err != nil {
		t.Fatal(err)
	}
	found, err := LinkFindings(kb.OnDisk(root))
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range found {
		got = append(got, f.Path+" "+f.Detail)
	}
	if want := []string{`a/prose.md a/prose.md:5 broken fragment "claim-rigor.md#theorem-1"`}; !slices.Equal(got, want) {
		t.Errorf("LinkFindings =\n%q\nwant\n%q", got, want)
	}
}
